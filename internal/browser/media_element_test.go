package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaElementLoadAndPlaybackState(t *testing.T) {
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

func TestMediaElementLoadsResourceWithoutDecoder(t *testing.T) {
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
