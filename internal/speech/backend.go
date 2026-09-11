// Package speech owns platform synthesis resources. It never calls JavaScript;
// the browser marshals its notifications onto the owning Page event loop.
package speech

type Voice struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Lang    string `json:"lang"`
	Default bool   `json:"default"`
	Local   bool   `json:"localService"`
}

type Utterance struct {
	ID                  uint64
	Text, VoiceID, Lang string
	Volume, Rate, Pitch float64
}

type Event struct {
	Kind                  string
	ID                    uint64
	Voices                []Voice
	Error                 string
	CharIndex, CharLength uint32
	Elapsed               float64
}

type Backend interface {
	Speak(Utterance)
	Pause()
	Resume()
	Cancel()
	Close()
}

// Provider creates independent synthesis resources. Embedders can supply any
// provider on any OS; browser semantics never depend on a particular driver.
type Provider func(notify func(Event)) Backend

// Open starts one lazy platform service for a document. Notifications are
// asynchronous, including voice discovery and native initialization failure.
func Open(notify func(Event)) Backend { return openPlatform(notify) }
