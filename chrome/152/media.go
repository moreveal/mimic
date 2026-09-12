package chrome152

import "github.com/moreveal/mimic/internal/state"

func mediaFormats() state.MediaFormatCatalog {
	c := state.MediaFormatCatalog{Containers: map[string]state.MediaContainerPolicy{}, EfficientVideo: []string{"avc", "hevc", "vp9", "av1"}}
	add := func(names []string, codecs []string, defaultCodec string, mse bool) {
		for _, name := range names {
			c.Containers[name] = state.MediaContainerPolicy{Codecs: append([]string(nil), codecs...), DefaultCodec: defaultCodec, MediaSource: mse}
		}
	}
	add([]string{"audio/mpeg"}, []string{"mp3"}, "mp3", true)
	add([]string{"audio/mp3"}, []string{"mp3"}, "mp3", false)
	add([]string{"audio/aac"}, []string{"aac"}, "aac", true)
	add([]string{"audio/flac"}, []string{"flac"}, "flac", false)
	add([]string{"audio/wav", "audio/x-wav"}, []string{"pcm"}, "", false)
	add([]string{"audio/ogg", "application/ogg"}, []string{"opus", "vorbis", "flac"}, "", false)
	add([]string{"audio/webm"}, []string{"opus", "vorbis"}, "", true)
	add([]string{"video/webm"}, []string{"vp8", "vp9", "av1", "opus", "vorbis"}, "", true)
	add([]string{"audio/mp4"}, []string{"aac", "opus", "flac", "mp3"}, "", true)
	add([]string{"video/mp4"}, []string{"avc", "hevc", "vp9", "av1", "aac", "opus", "flac", "mp3"}, "", true)
	return c
}
