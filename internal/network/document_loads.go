package network

import (
	"context"
	"fmt"
)

var ErrDocumentLoadingStopped = fmt.Errorf("document loading stopped: %w", context.Canceled)

// A Loader belongs to one Page. Track logical document requests rather than
// replacing realm lifetimes: stopping current loads must permit later fetches.
// Redirect/preflight calls inherit this context and need no second registration.
func (l *Loader) trackDocumentLoad(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	l.activityMu.Lock()
	if l.documentLoads == nil {
		l.documentLoads = make(map[uint64]context.CancelCauseFunc)
	}
	l.documentLoadSequence++
	id := l.documentLoadSequence
	l.documentLoads[id] = cancel
	l.activityMu.Unlock()
	return ctx, func() {
		l.activityMu.Lock()
		delete(l.documentLoads, id)
		l.activityMu.Unlock()
		cancel(nil)
	}
}

func (l *Loader) CancelDocumentRequests() {
	l.activityMu.Lock()
	cancels := make([]context.CancelCauseFunc, 0, len(l.documentLoads))
	for _, cancel := range l.documentLoads {
		cancels = append(cancels, cancel)
	}
	l.activityMu.Unlock()
	for _, cancel := range cancels {
		cancel(ErrDocumentLoadingStopped)
	}
}
