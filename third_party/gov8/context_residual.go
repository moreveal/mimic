//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// NoContextSnapshotIndex is the size_t sentinel accepted by
// Context::FromSnapshot. On an isolate without snapshot data it creates a
// fresh context, while ordinary absent indices return ok=false.
const NoContextSnapshotIndex = ^uint64(0)

// maxContextEmbedderDataSlots is the number of logical slots exposed by the
// pinned crate. V8's backing array is limited to 4096 fields and rusty_v8
// reserves the first two fields for its internal annex.
const maxContextEmbedderDataSlots = 4096 - kCtxInternalSlotCount

func validateContextEmbedderDataSlot(slot int) error {
	if slot < 0 || slot >= maxContextEmbedderDataSlots {
		return fmt.Errorf("gov8: embedder data slot out of range: %d", slot)
	}
	return nil
}

// ContextFromSnapshotWithOptions recovers an added context using the exact
// size_t snapshot index and the options supported by rusty_v8. GlobalTemplate
// is intentionally ignored: Context::from_snapshot reuses serialized global
// state and forwards only GlobalObject and MicrotaskQueue to V8.
//
// NoContextSnapshotIndex mirrors usize::MAX. It creates a fresh context even
// when the isolate has no startup blob. Other absent indices return ok=false.
func (s *Scope) ContextFromSnapshotWithOptions(index uint64, options *ContextOptions) (*Context, bool, error) {
	if err := s.check(); err != nil {
		return nil, false, err
	}
	ih, err := s.iso.handleChecked()
	if err != nil {
		return nil, false, err
	}

	var globalH, queueH uintptr
	var queue *MicrotaskQueue
	if options != nil {
		// options.GlobalTemplate is deliberately not inspected. The pinned
		// crate accepts this field in ContextOptions but ignores it on the
		// from_snapshot path.
		if options.GlobalObject != nil {
			if err := options.GlobalObject.Value.check(); err != nil {
				return nil, false, err
			}
			if options.GlobalObject.iso != s.iso {
				return nil, false, foreignIsolate("global object")
			}
			globalH = options.GlobalObject.h
		}
		if options.MicrotaskQueue != nil {
			if err := options.MicrotaskQueue.check(); err != nil {
				return nil, false, err
			}
			if options.MicrotaskQueue.iso != s.iso {
				return nil, false, foreignIsolate("microtask queue")
			}
			queue = options.MicrotaskQueue
			queueH = queue.handle
		}
	}

	var contextH uintptr
	var found int32
	r1, _, _ := proc("gov8_cr_context_from_snapshot_options").Call(
		ih, s.handle, uintptr(index), globalH, queueH,
		uintptr(unsafe.Pointer(&contextH)), uintptr(unsafe.Pointer(&found)))
	if int64(r1) < 0 {
		return nil, false, shimError("Scope.ContextFromSnapshotWithOptions", r1)
	}
	if found != 1 {
		return nil, false, nil
	}
	if contextH == 0 {
		return nil, false, fmt.Errorf("gov8: Context::FromSnapshot returned an invalid context")
	}
	s.iso.mu.Lock()
	s.iso.contextsCreated = true
	s.iso.mu.Unlock()
	if queue != nil {
		queue.attachments++
	}
	return &Context{iso: s.iso, handle: contextH, microtaskQueue: queue}, true, nil
}

// contextHasUnclearedSlots reports host-side Context slots that rusty_v8
// would represent with a Weak-backed annex. Such an annex makes snapshot
// creation process-fatal upstream and must be cleared first.
func contextHasUnclearedSlots(iso *Isolate) bool {
	contextSlots.mu.Lock()
	defer contextSlots.mu.Unlock()
	for context, slots := range contextSlots.m {
		if context.iso == iso && len(slots) != 0 {
			return true
		}
	}
	return false
}
