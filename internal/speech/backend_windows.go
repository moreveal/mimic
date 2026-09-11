//go:build windows && cgo

package speech

/*
#cgo windows LDFLAGS: -lole32 -luuid -lsapi
#define COBJMACROS
#define _SAPI_VER 0x54
#include <windows.h>
#include <sapi.h>
#include <stdlib.h>

typedef struct {
 ISpVoice *voice;
 HANDLE wake;
 HANDLE events;
 int initialized;
 unsigned long bytesPerSecond;
} mimic_speech;
typedef struct { wchar_t *id,*name; wchar_t lang[LOCALE_NAME_MAX_LENGTH]; int isDefault; } mimic_voice;
typedef struct { unsigned kind,stream,index,length; double elapsed; } mimic_speech_event;

static mimic_speech *speech_create(void) {
 mimic_speech *s=calloc(1,sizeof(*s));
 if(s) { s->wake=CreateEventW(NULL,FALSE,FALSE,NULL); if(!s->wake){free(s);return NULL;} }
 return s;
}
static HRESULT speech_init(mimic_speech *s) {
 HRESULT hr=CoInitializeEx(NULL,COINIT_MULTITHREADED);
 if(FAILED(hr))return hr;
 s->initialized=1;
 hr=CoCreateInstance(&CLSID_SpVoice,NULL,CLSCTX_INPROC_SERVER,&IID_ISpVoice,(void**)&s->voice);
 if(FAILED(hr))return hr;
 hr=ISpVoice_SetNotifyWin32Event(s->voice);if(FAILED(hr))return hr;
 ULONGLONG interest=SPFEI(SPEI_START_INPUT_STREAM)|SPFEI(SPEI_END_INPUT_STREAM)|SPFEI(SPEI_WORD_BOUNDARY)|SPFEI(SPEI_SENTENCE_BOUNDARY);
 hr=ISpVoice_SetInterest(s->voice,interest,interest);if(FAILED(hr))return hr;
 s->events=ISpVoice_GetNotifyEventHandle(s->voice);
 ISpStreamFormat *stream=NULL;
 if(SUCCEEDED(ISpVoice_GetOutputStream(s->voice,&stream))&&stream){
  GUID format;WAVEFORMATEX *wave=NULL;
  if(SUCCEEDED(ISpStreamFormat_GetFormat(stream,&format,&wave))&&wave){s->bytesPerSecond=wave->nAvgBytesPerSec;CoTaskMemFree(wave);}
  ISpStreamFormat_Release(stream);
 }
 return S_OK;
}
static HRESULT speech_voices(mimic_speech *s,mimic_voice **out,ULONG *count) {
 ISpObjectTokenCategory *category=NULL;IEnumSpObjectTokens *enumerator=NULL;wchar_t *defaultID=NULL;
 HRESULT hr=CoCreateInstance(&CLSID_SpObjectTokenCategory,NULL,CLSCTX_INPROC_SERVER,&IID_ISpObjectTokenCategory,(void**)&category);
 if(FAILED(hr))return hr;
 hr=ISpObjectTokenCategory_SetId(category,L"HKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Speech_OneCore\\Voices",FALSE);
 if(SUCCEEDED(hr)){ISpObjectTokenCategory_GetDefaultTokenId(category,&defaultID);hr=ISpObjectTokenCategory_EnumTokens(category,NULL,NULL,&enumerator);}
 if(SUCCEEDED(hr))hr=IEnumSpObjectTokens_GetCount(enumerator,count);
 if(SUCCEEDED(hr)){
  *out=calloc(*count,sizeof(mimic_voice));if(*count&&!*out)hr=E_OUTOFMEMORY;
  if(SUCCEEDED(hr))for(ULONG i=0;i<*count;i++){
   ISpObjectToken *token=NULL;ISpDataKey *attributes=NULL;wchar_t *language=NULL;
   if(FAILED(IEnumSpObjectTokens_Item(enumerator,i,&token)))continue;
   ISpObjectToken_GetId(token,&(*out)[i].id);ISpObjectToken_GetStringValue(token,NULL,&(*out)[i].name);
   if(SUCCEEDED(ISpObjectToken_OpenKey(token,L"Attributes",&attributes))){
    if(SUCCEEDED(ISpDataKey_GetStringValue(attributes,L"Language",&language))){LCIDToLocaleName((LCID)wcstoul(language,NULL,16),(*out)[i].lang,LOCALE_NAME_MAX_LENGTH,0);CoTaskMemFree(language);}
    ISpDataKey_Release(attributes);
   }
   (*out)[i].isDefault=defaultID&&(*out)[i].id&&wcscmp(defaultID,(*out)[i].id)==0;
   ISpObjectToken_Release(token);
  }
 }
 if(defaultID)CoTaskMemFree(defaultID);if(enumerator)IEnumSpObjectTokens_Release(enumerator);ISpObjectTokenCategory_Release(category);return hr;
}
static void speech_free_voices(mimic_voice *voices,ULONG count){for(ULONG i=0;i<count;i++){CoTaskMemFree(voices[i].id);CoTaskMemFree(voices[i].name);}free(voices);}
static HRESULT speech_speak(mimic_speech *s,const wchar_t *text,const wchar_t *voice,int rate,unsigned volume,int xml,ULONG *stream){
 HRESULT hr;
 if(voice&&*voice){ISpObjectToken *token=NULL;hr=CoCreateInstance(&CLSID_SpObjectToken,NULL,CLSCTX_INPROC_SERVER,&IID_ISpObjectToken,(void**)&token);if(FAILED(hr))return hr;
  hr=ISpObjectToken_SetId(token,NULL,voice,FALSE);if(SUCCEEDED(hr))hr=ISpVoice_SetVoice(s->voice,token);ISpObjectToken_Release(token);if(FAILED(hr))return hr;
 }
 ISpVoice_SetRate(s->voice,rate);ISpVoice_SetVolume(s->voice,(USHORT)volume);
 return ISpVoice_Speak(s->voice,text,SPF_ASYNC|(xml?SPF_IS_XML:SPF_IS_NOT_XML),stream);
}
static HRESULT speech_pause(mimic_speech *s){return ISpVoice_Pause(s->voice);}
static HRESULT speech_resume(mimic_speech *s){return ISpVoice_Resume(s->voice);}
static void speech_cancel(mimic_speech *s){if(s->voice)ISpVoice_Speak(s->voice,NULL,SPF_ASYNC|SPF_PURGEBEFORESPEAK,NULL);}
static int speech_next_event(mimic_speech *s,mimic_speech_event *out){
 SPEVENT e;ULONG fetched=0;
 if(FAILED(ISpVoice_GetEvents(s->voice,1,&e,&fetched))||!fetched)return 0;
 out->kind=e.eEventId;out->stream=e.ulStreamNum;out->index=(unsigned)e.lParam;out->length=(unsigned)e.wParam;
 out->elapsed=s->bytesPerSecond?(double)e.ullAudioStreamOffset/s->bytesPerSecond:0;
 if(e.elParamType==SPET_LPARAM_IS_OBJECT||e.elParamType==SPET_LPARAM_IS_TOKEN)IUnknown_Release((IUnknown*)e.lParam);
 else if(e.elParamType==SPET_LPARAM_IS_POINTER||e.elParamType==SPET_LPARAM_IS_STRING)CoTaskMemFree((void*)e.lParam);
 return 1;
}
static void speech_wake(mimic_speech *s){SetEvent(s->wake);}
static DWORD speech_wait(mimic_speech *s){HANDLE handles[2]={s->wake,s->events};return WaitForMultipleObjects(s->events?2:1,handles,FALSE,INFINITE);}
static void speech_destroy(mimic_speech *s){if(s->voice){speech_cancel(s);ISpVoice_Release(s->voice);}if(s->initialized)CoUninitialize();CloseHandle(s->wake);free(s);}
*/
import "C"

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type nativeCommand struct {
	kind      string
	utterance Utterance
}
type windowsBackend struct {
	mu       sync.Mutex
	native   *C.mimic_speech
	commands []nativeCommand
	closed   bool
	done     chan struct{}
	wake     chan struct{}
	notify   func(Event)
}

func openPlatform(notify func(Event)) Backend {
	b := &windowsBackend{native: C.speech_create(), done: make(chan struct{}), wake: make(chan struct{}, 1), notify: notify}
	go b.run()
	return b
}
func (b *windowsBackend) submit(command nativeCommand) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.commands = append(b.commands, command)
	select {
	case b.wake <- struct{}{}:
	default:
	}
	if b.native != nil {
		C.speech_wake(b.native)
	}
}
func (b *windowsBackend) Speak(u Utterance) { b.submit(nativeCommand{kind: "speak", utterance: u}) }
func (b *windowsBackend) Pause()            { b.submit(nativeCommand{kind: "pause"}) }
func (b *windowsBackend) Resume()           { b.submit(nativeCommand{kind: "resume"}) }
func (b *windowsBackend) Cancel()           { b.submit(nativeCommand{kind: "cancel"}) }
func (b *windowsBackend) Close()            { b.submit(nativeCommand{kind: "close"}); <-b.done }

func speechUTF16(value *C.wchar_t) string {
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(value)))
}
func (b *windowsBackend) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(b.done)
	defer func() {
		b.mu.Lock()
		b.closed = true
		b.commands = nil
		if b.native != nil {
			C.speech_destroy(b.native)
			b.native = nil
		}
		b.mu.Unlock()
	}()
	available := b.native != nil
	if available {
		available = C.speech_init(b.native) >= 0
	}
	voices := []Voice{}
	if available {
		var rows *C.mimic_voice
		var count C.ULONG
		if C.speech_voices(b.native, &rows, &count) >= 0 {
			for _, row := range unsafe.Slice(rows, int(count)) {
				voices = append(voices, Voice{ID: speechUTF16(row.id), Name: speechUTF16(row.name), Lang: speechUTF16(&row.lang[0]), Local: true, Default: row.isDefault != 0})
			}
			C.speech_free_voices(rows, count)
		}
	}
	b.notify(Event{Kind: "voices", Voices: voices})
	var active Utterance
	var stream C.ULONG
	var positions []uint32
	for {
		b.mu.Lock()
		commands := b.commands
		b.commands = nil
		b.mu.Unlock()
		for _, command := range commands {
			switch command.kind {
			case "close":
				return
			case "cancel":
				if available {
					C.speech_cancel(b.native)
				}
				active = Utterance{}
			case "pause":
				if available && C.speech_pause(b.native) >= 0 {
					b.notify(Event{Kind: "pause", ID: active.ID})
				}
			case "resume":
				if available && C.speech_resume(b.native) >= 0 {
					b.notify(Event{Kind: "resume", ID: active.ID})
				}
			case "speak":
				active = command.utterance
				if !available {
					b.notify(Event{Kind: "error", ID: active.ID, Error: "synthesis-unavailable"})
					active = Utterance{}
					continue
				}
				voiceID := active.VoiceID
				if voiceID == "" {
					for _, voice := range voices {
						if active.Lang != "" && strings.EqualFold(voice.Lang, active.Lang) || active.Lang == "" && voice.Default {
							voiceID = voice.ID
							break
						}
					}
				}
				text, xml, mapping := sapiText(active.Text, active.Pitch)
				positions = mapping
				token, _ := windows.UTF16FromString(voiceID)
				encoded, _ := windows.UTF16FromString(text)
				rate := int(math.Round(10 * math.Log2(active.Rate)))
				rate = max(-10, min(10, rate))
				xmlFlag := 0
				if xml {
					xmlFlag = 1
				}
				hr := C.speech_speak(b.native, (*C.wchar_t)(unsafe.Pointer(&encoded[0])), (*C.wchar_t)(unsafe.Pointer(&token[0])), C.int(rate), C.uint(math.Round(active.Volume*100)), C.int(xmlFlag), &stream)
				if hr < 0 {
					b.notify(Event{Kind: "error", ID: active.ID, Error: "synthesis-failed"})
					active = Utterance{}
				}
			}
		}
		if available {
			var raw C.mimic_speech_event
			for C.speech_next_event(b.native, &raw) != 0 {
				if active.ID == 0 || raw.stream != C.uint(stream) {
					continue
				}
				e := Event{ID: active.ID, CharIndex: uint32(raw.index), CharLength: uint32(raw.length), Elapsed: float64(raw.elapsed)}
				switch raw.kind {
				case 1:
					e.Kind = "start"
					e.CharIndex = 0
					e.CharLength = 0
				case 2:
					e.Kind = "end"
					e.CharIndex = uint32(len(utf16.Encode([]rune(active.Text))))
					e.CharLength = 0
				case 5:
					e.Kind = "word"
				case 7:
					e.Kind = "sentence"
				default:
					continue
				}
				if len(positions) > 0 && (e.Kind == "word" || e.Kind == "sentence") {
					end := min(int(e.CharIndex+e.CharLength), len(positions)-1)
					start := min(int(e.CharIndex), len(positions)-1)
					e.CharIndex = positions[start]
					e.CharLength = positions[end] - e.CharIndex
				}
				b.notify(e)
				if e.Kind == "end" {
					active = Utterance{}
				}
			}
		}
		if !available {
			// Even allocation/initialization failure leaves a responsive provider:
			// future Speak calls must receive failure and Close must always join.
			<-b.wake
			continue
		}
		if C.speech_wait(b.native) == C.WAIT_FAILED {
			C.speech_cancel(b.native)
			available = false
			if active.ID != 0 {
				b.notify(Event{Kind: "error", ID: active.ID, Error: "synthesis-failed"})
				active = Utterance{}
			}
		}
	}
}

// SAPI exposes pitch through XML. Track source UTF-16 positions through the
// escaping so word/sentence events still address the caller's plain text.
func sapiText(text string, pitch float64) (string, bool, []uint32) {
	if pitch == 1 {
		return strings.ReplaceAll(text, "\x00", ""), false, nil
	}
	prefix := fmt.Sprintf("<pitch absmiddle=\"%d\">", int(math.Round((pitch-1)*10)))
	var encoded strings.Builder
	encoded.WriteString(prefix)
	mapping := make([]uint32, len(prefix))
	var index uint32
	for _, r := range text {
		part := string(r)
		switch r {
		case '&':
			part = "&amp;"
		case '<':
			part = "&lt;"
		case '>':
			part = "&gt;"
		case 0:
			part = ""
		}
		encoded.WriteString(part)
		for range utf16.Encode([]rune(part)) {
			mapping = append(mapping, index)
		}
		index += uint32(len(utf16.Encode([]rune{r})))
	}
	encoded.WriteString("</pitch>")
	for range len("</pitch>") + 1 {
		mapping = append(mapping, index)
	}
	return encoded.String(), true, mapping
}
