package browser

import (
	"context"
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
  if (audio.currentTime !== 12 || !audio.paused || audio.networkState !== 3 || audio.readyState !== 0) throw new Error('load state');
  const video = document.createElement('video');
  if (!(video instanceof HTMLVideoElement) || !(video instanceof HTMLMediaElement)) throw new Error('video prototype');
  return true;
})()
`); err != nil {
		t.Fatal(err)
	}
}
