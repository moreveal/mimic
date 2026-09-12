package trace

import (
	"sort"
	"sync"
	"time"
)

type Kind string

const (
	API             Kind = "api"
	Unsupported     Kind = "unsupported"
	SurfaceMissing  Kind = "surface-missing"
	SemanticMissing Kind = "semantic-missing"
	Network         Kind = "network"
	Console         Kind = "console"
	Exception       Kind = "exception"
	Lifecycle       Kind = "lifecycle"
	CDP             Kind = "cdp"
	Resource        Kind = "resource"
	DOM             Kind = "dom"
	JS              Kind = "js"
	CSP             Kind = "csp"
	Scheduler       Kind = "scheduler"
	Error           Kind = "error"
)

type Event struct {
	Sequence uint64         `json:"sequence"`
	Time     time.Time      `json:"time"`
	Kind     Kind           `json:"kind"`
	Name     string         `json:"name"`
	Data     map[string]any `json:"data,omitempty"`
}
type Recorder struct {
	mu          sync.RWMutex
	next        uint64
	events      []Event
	subscribers map[uint64]func(Event)
	subID       uint64
}

func New() *Recorder { return &Recorder{subscribers: map[uint64]func(Event){}} }
func (r *Recorder) Add(kind Kind, name string, data map[string]any) {
	r.mu.Lock()
	r.next++
	e := Event{r.next, time.Now().UTC(), kind, name, data}
	r.events = append(r.events, e)
	subs := make([]func(Event), 0, len(r.subscribers))
	for _, f := range r.subscribers {
		subs = append(subs, f)
	}
	r.mu.Unlock()
	for _, f := range subs {
		f(e)
	}
}
func (r *Recorder) Events() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Event(nil), r.events...)
}

// EventsSince returns newly recorded events without copying the complete trace.
// Sequence numbers survive Clear, so consumers cannot replay cleared history.
func (r *Recorder) EventsSince(sequence uint64) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	start := sort.Search(len(r.events), func(i int) bool { return r.events[i].Sequence > sequence })
	return append([]Event(nil), r.events[start:]...)
}
func (r *Recorder) Clear() {
	r.mu.Lock()
	r.events = nil
	r.mu.Unlock()
}
func (r *Recorder) Subscribe(f func(Event)) func() {
	r.mu.Lock()
	r.subID++
	id := r.subID
	r.subscribers[id] = f
	r.mu.Unlock()
	return func() { r.mu.Lock(); delete(r.subscribers, id); r.mu.Unlock() }
}
