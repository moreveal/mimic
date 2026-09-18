package trace

import (
	"context"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

type correlation struct {
	id       string
	started  time.Time
	recorder *Recorder
	spanSeq  atomic.Uint64
}

type correlationKey struct{}

// WithCorrelation attaches one request-level monotonic timeline to a context.
// The recorder is optional so engine-neutral callers can still propagate the ID.
func WithCorrelation(ctx context.Context, id string, started time.Time, recorder *Recorder) context.Context {
	return context.WithValue(ctx, correlationKey{}, &correlation{id: id, started: started, recorder: recorder})
}

func CorrelationID(ctx context.Context) string {
	if value, ok := ctx.Value(correlationKey{}).(*correlation); ok && value != nil {
		return value.id
	}
	return ""
}

func NextCorrelationSpan(ctx context.Context) uint64 {
	if value, ok := ctx.Value(correlationKey{}).(*correlation); ok && value != nil {
		return value.spanSeq.Add(1)
	}
	return 0
}

func CorrelatedData(ctx context.Context, data map[string]any) map[string]any {
	out := make(map[string]any, len(data)+4)
	for key, value := range data {
		out[key] = value
	}
	value, ok := ctx.Value(correlationKey{}).(*correlation)
	if !ok || value == nil {
		return out
	}
	out["callID"] = value.id
	out["elapsedNs"] = time.Since(value.started).Nanoseconds()
	out["goroutineID"] = goroutineID()
	return out
}

// Record emits an invocation-correlated event when the context carries a
// recorder. All elapsed values use time.Since on the original call start and
// therefore remain monotonic even though Event.Time is wall-clock metadata.
func Record(ctx context.Context, kind Kind, name string, data map[string]any) {
	value, ok := ctx.Value(correlationKey{}).(*correlation)
	if !ok || value == nil || value.recorder == nil {
		return
	}
	value.recorder.Add(kind, name, CorrelatedData(ctx, data))
}

func goroutineID() uint64 {
	var buffer [64]byte
	n := runtime.Stack(buffer[:], false)
	fields := strings.Fields(string(buffer[:n]))
	if len(fields) < 2 {
		return 0
	}
	id, _ := strconv.ParseUint(fields[1], 10, 64)
	return id
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
