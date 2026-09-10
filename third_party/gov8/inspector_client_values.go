//go:build windows && amd64

package gov8

import "fmt"

// InspectorValueSubtypeClient supplies an optional protocol subtype for an
// object. The CallbackScope and Value are borrowed for this callback only.
type InspectorValueSubtypeClient interface {
	ValueSubtype(scope *CallbackScope, value Value) *InspectorStringBuffer
}

// InspectorValueDescriptionClient supplies an optional description after a
// non-nil subtype result. The CallbackScope and Value are borrowed.
type InspectorValueDescriptionClient interface {
	DescriptionForValueSubtype(scope *CallbackScope, value Value) *InspectorStringBuffer
}

// InspectorDefaultContextClient supplies the default registered context for a
// context group. Nil means that no default context exists.
type InspectorDefaultContextClient interface {
	EnsureDefaultContextInGroup(contextGroupID int32) *Context
}

func inspectorRegisteredCallbackContext(entry *inspectorClientEntry, contextWire uintptr) (*Context, error) {
	if entry.inspector == nil || entry.inspector.closed {
		return nil, fmt.Errorf("gov8: Inspector value callback has no live Inspector")
	}
	for context := range entry.inspector.contexts {
		matched, _, _ := proc("gov8_icv_context_matches").Call(context.handle, contextWire)
		if int64(matched) < 0 {
			return nil, shimError("Inspector.ContextMatches", matched)
		}
		if matched == 1 {
			return context, nil
		}
	}
	return nil, fmt.Errorf("gov8: Inspector value callback context is not registered")
}

func inspectorClientValueDispatch(entry *inspectorClientEntry, kind int,
	a0, a1, a2 uintptr) (uintptr, bool) {
	switch kind {
	case 4, 5:
		context, err := inspectorRegisteredCallbackContext(entry, a0)
		if err != nil {
			fatalHostMisuse("%v", err)
		}
		scope := &Scope{iso: entry.iso, handle: a2, borrowed: true}
		callbackScope := &CallbackScope{iso: entry.iso, sc: scope, ctxWire: a0}
		value := Value{iso: entry.iso, sc: scope, h: a1}
		defer func() { scope.closed = true }()
		if context.closed || value.h == 0 {
			fatalHostMisuse("invalid Inspector value callback local")
		}
		var buffer *InspectorStringBuffer
		if kind == 4 {
			if client, ok := entry.client.(InspectorValueSubtypeClient); ok {
				buffer = client.ValueSubtype(callbackScope, value)
			}
		} else if client, ok := entry.client.(InspectorValueDescriptionClient); ok {
			buffer = client.DescriptionForValueSubtype(callbackScope, value)
		}
		return inspectorClientNativeBuffer(buffer), true
	case 6:
		client, ok := entry.client.(InspectorDefaultContextClient)
		if !ok {
			return 0, true
		}
		context := client.EnsureDefaultContextInGroup(int32(a0))
		if context == nil {
			return 0, true
		}
		if context.iso != entry.iso {
			fatalHostMisuse("Inspector default context belongs to a different isolate")
		}
		if err := context.checkAssumingIsolate(); err != nil {
			fatalHostMisuse("invalid Inspector default context: %v", err)
		}
		if entry.inspector == nil {
			fatalHostMisuse("Inspector default context has no Inspector")
		}
		if _, registered := entry.inspector.contexts[context]; !registered {
			fatalHostMisuse("Inspector default context is not registered")
		}
		return context.handle, true
	}
	return 0, false
}
