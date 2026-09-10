//go:build windows && amd64

package gov8

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// InspectorUniqueIDClient supplies Inspector-generated identifiers.
type InspectorUniqueIDClient interface{ GenerateUniqueID() int64 }

// InspectorWaitingForDebuggerClient receives Runtime.runIfWaitingForDebugger.
type InspectorWaitingForDebuggerClient interface{ RunIfWaitingForDebugger(contextGroupID int32) }

// InspectorResourceNameClient optionally maps script resource names to URLs.
type InspectorResourceNameClient interface {
	ResourceNameToURL(resourceName InspectorStringView) *InspectorStringBuffer
}

// InspectorConsoleMessageClient receives console API messages synchronously.
type InspectorConsoleMessageClient interface {
	ConsoleAPIMessage(contextGroupID int32, level int32, message, url InspectorStringView,
		lineNumber, columnNumber uint32, stackTrace InspectorBorrowedStackTrace)
}

// InspectorBorrowedStackTrace is an opaque marker valid only during a console
// callback. The underlying trace remains owned by V8.
type InspectorBorrowedStackTrace struct{ present bool }

// Present reports whether V8 supplied a borrowed stack trace marker.
func (s InspectorBorrowedStackTrace) Present() bool { return s.present }

const (
	inspectorClientGenerate = iota
	inspectorClientWaiting
	inspectorClientResource
	inspectorClientConsole
)

func inspectorCallbackView(is8Word, dataWord, lengthWord uintptr) InspectorStringView {
	if is8Word != 0 && is8Word != 1 {
		fatalHostMisuse("invalid Inspector client StringView encoding %d", is8Word)
	}
	if lengthWord > uintptr(^uint(0)>>1) {
		fatalHostMisuse("Inspector client StringView length %d exceeds max int", lengthWord)
	}
	if dataWord == 0 && lengthWord != 0 {
		fatalHostMisuse("Inspector client returned a null non-empty StringView")
	}
	length := int(lengthWord)
	if is8Word == 1 {
		data := make([]byte, length)
		if length != 0 {
			copy(data, unsafe.Slice((*byte)(abiWordToPtr(dataWord)), length))
		}
		return NewInspectorStringView8(data)
	}
	data := make([]uint16, length)
	if length != 0 {
		copy(data, unsafe.Slice((*uint16)(abiWordToPtr(dataWord)), length))
	}
	return NewInspectorStringView16(data)
}

func inspectorClientNativeBuffer(buffer *InspectorStringBuffer) uintptr {
	if buffer == nil {
		return 0
	}
	view := buffer.StringView()
	is8, data, length := view.native()
	var out uintptr
	r, _, _ := proc("gov8_icc_string_buffer_create").Call(is8, data, length, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(view)
	runtime.KeepAlive(buffer)
	if int64(r) < 0 || out == 0 {
		fatalHostMisuse("creating Inspector client StringBuffer: %v", shimError("StringBuffer.Create", r))
	}
	return out
}

var inspectorExtendedClientDispatch = syscall.NewCallback(func(
	idWord, kindWord, a0, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10 uintptr,
) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in inspector client callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	inspectorClients.Lock()
	entry := inspectorClients.byID[uint64(idWord)]
	if entry != nil {
		entry.active++
	}
	inspectorClients.Unlock()
	if entry == nil || entry.client == nil {
		fatalHostMisuse("unknown inspector client %d", idWord)
	}
	defer func() {
		inspectorClients.Lock()
		entry.active--
		inspectorClients.Unlock()
	}()
	if err := entry.iso.check(); err != nil {
		fatalHostMisuse("Inspector client invoked outside isolate lifetime: %v", err)
	}
	switch int(kindWord) {
	case inspectorClientGenerate:
		if client, ok := entry.client.(InspectorUniqueIDClient); ok {
			return uintptr(client.GenerateUniqueID())
		}
	case inspectorClientWaiting:
		if client, ok := entry.client.(InspectorWaitingForDebuggerClient); ok {
			client.RunIfWaitingForDebugger(int32(a0))
		}
	case inspectorClientResource:
		if client, ok := entry.client.(InspectorResourceNameClient); ok {
			buffer := client.ResourceNameToURL(inspectorCallbackView(a0, a1, a2))
			if buffer == nil {
				return 0
			}
			return inspectorClientNativeBuffer(buffer)
		}
	case inspectorClientConsole:
		if client, ok := entry.client.(InspectorConsoleMessageClient); ok {
			client.ConsoleAPIMessage(int32(a0), int32(a1),
				inspectorCallbackView(a2, a3, a4), inspectorCallbackView(a5, a6, a7),
				uint32(a8), uint32(a9), InspectorBorrowedStackTrace{present: a10 != 0})
		}
	default:
		if result, handled := inspectorClientValueDispatch(entry, int(kindWord), a0, a1, a2); handled {
			return result
		}
		fatalHostMisuse("invalid extended Inspector client callback kind %d", kindWord)
	}
	return 0
})

func ensureInspectorExtendedClientDispatch() error {
	return callErr("Inspector.ExtendedClientDispatch", proc("gov8_icc_set_dispatch"), inspectorExtendedClientDispatch)
}
