package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMediaElementLoadAndPlaybackState(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	defer p.Close()
	if _, err := p.Evaluate(context.Background(), `
(() => {
  const audio = new Audio('/feedback.mp3');
  if (!(audio instanceof HTMLAudioElement) || !(audio instanceof HTMLMediaElement)) throw new Error('audio prototype');
  if (typeof audio.load !== 'function' || typeof audio.play !== 'function' || typeof audio.pause !== 'function') throw new Error('media methods');
  audio.volume = 0.5;
  if (!audio.paused || audio.volume !== 0.5 || audio.currentSrc !== '') throw new Error('initial state');
  audio.pause();
  if (!audio.paused) throw new Error('pause state');
  audio.currentTime = 12;
  audio.load();
  // Chrome 152 retains the default playback start position before metadata.
  if (audio.currentTime !== 12 || !audio.paused || audio.networkState !== 2 || audio.readyState !== 0) throw new Error('load state');
  const video = document.createElement('video');
  if (!(video instanceof HTMLVideoElement) || !(video instanceof HTMLMediaElement)) throw new Error('video prototype');
  return true;
})()
`); err != nil {
		t.Fatal(err)
	}
}

func TestMediaControlsListChrome152(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `(()=>{const audio=document.createElement('audio'),list=audio.controlsList;audio.controlsList='foo   nodownload foo';return JSON.stringify({same:list===audio.controlsList,value:list.value,attribute:audio.getAttribute('controlslist'),supported:list.supports('nodownload'),unknown:list.supports('foo'),setter:typeof Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype,'controlsList').set})})()`)
	if err != nil || value != `{"same":true,"value":"foo   nodownload foo","attribute":"foo   nodownload foo","supported":true,"unknown":false,"setter":"function"}` {
		t.Fatalf("controlsList: %v %v", value, err)
	}
}

func TestMediaPreloadNoneDefersTransportUntilExplicitLoad(t *testing.T) {
	parallelBrowserTest(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("opaque media bytes"))
	}))
	defer server.Close()
	p := testPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `new Promise(resolve=>{
		const audio=document.createElement('audio');
		audio.preload='none';
		audio.src=`+fmt.Sprintf("%q", server.URL+"/clip.wav")+`;
		document.body.append(audio);
		setTimeout(()=>resolve([audio.networkState,audio.readyState,audio.currentSrc]),10);
	})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("preload=none issued %d requests", got)
	}
	state := value.([]any)
	if state[1] != int64(0) || state[2] != "" {
		t.Fatalf("unexpected deferred state: %#v", state)
	}
	if _, err = p.Evaluate(context.Background(), `new Promise(resolve=>{const audio=document.querySelector('audio');audio.oncanplay=resolve;audio.load()})`); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("explicit load issued %d requests", got)
	}
}

func TestMediaElementLoadsResourceWithoutDecoder(t *testing.T) {
	parallelBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("opaque media bytes"))
	}))
	defer server.Close()
	p := testPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `new Promise(resolve=>{
		const video=document.createElement('video'), events=[];
		for(const type of ['loadedmetadata','canplay','play','playing']) video.addEventListener(type,()=>events.push(type));
		video.src=`+fmt.Sprintf("%q", server.URL+"/clip.mp4")+`;
		document.body.append(video);
		video.play().then(()=>setTimeout(()=>resolve({src:video.currentSrc,ready:video.readyState,network:video.networkState,paused:video.paused,events}),0),error=>resolve({error:error.name}));
	})`)
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["src"] != server.URL+"/clip.mp4" || result["ready"] != int64(4) || result["network"] != int64(1) || result["paused"] != false {
		t.Fatalf("unexpected media state: %#v", result)
	}
}
