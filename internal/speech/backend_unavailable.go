//go:build !windows || !cgo

package speech

type unavailable struct{ notify func(Event) }

func openPlatform(notify func(Event)) Backend { return &unavailable{notify} }
func (b *unavailable) Speak(u Utterance) {
	b.notify(Event{Kind: "error", ID: u.ID, Error: "synthesis-unavailable"})
}
func (b *unavailable) Pause()  {}
func (b *unavailable) Resume() {}
func (b *unavailable) Cancel() {}
func (b *unavailable) Close()  {}
