//go:build (windows || linux) && amd64

package v8

import (
	"runtime"

	gov8 "github.com/maclof/gov8"
)

// exportBinaryBuffer preserves the viewed byte range across the JS-to-Go host
// boundary. JSON.stringify turns typed arrays into indexed objects and cannot
// serialize BigInt typed arrays at all.
func exportBinaryBuffer(value gov8.Value) ([]byte, bool) {
	if yes, _ := value.IsTypedArray(); yes {
		view, err := gov8.AsTypedArray(value)
		if err != nil {
			return nil, true
		}
		length, err := view.ByteLength()
		if err != nil {
			return nil, true
		}
		out := make([]byte, length)
		if copied, err := view.CopyContents(out); err != nil || copied != length {
			return nil, true
		}
		return out, true
	}
	if yes, _ := value.IsDataView(); yes {
		view, err := gov8.AsDataView(value)
		if err != nil {
			return nil, true
		}
		length, err := view.ByteLength()
		if err != nil {
			return nil, true
		}
		out := make([]byte, length)
		if copied, err := view.CopyContents(out); err != nil || copied != length {
			return nil, true
		}
		return out, true
	}
	if yes, _ := value.IsArrayBuffer(); yes {
		buffer, err := gov8.AsArrayBuffer(value)
		if err != nil {
			return nil, true
		}
		store, err := buffer.GetBackingStore()
		if err != nil {
			return nil, true
		}
		defer store.Close()
		length, err := buffer.ByteLength()
		if err != nil {
			return nil, true
		}
		out := make([]byte, length)
		if copied, err := store.ReadAt(out, 0); err != nil || copied != length {
			return nil, true
		}
		return out, true
	}
	if yes, _ := value.IsSharedArrayBuffer(); yes {
		buffer, err := gov8.AsSharedArrayBuffer(value)
		if err != nil {
			return nil, true
		}
		store, err := buffer.GetBackingStore()
		if err != nil {
			return nil, true
		}
		defer store.Close()
		length, err := buffer.ByteLength()
		if err != nil {
			return nil, true
		}
		out := make([]byte, length)
		if copied, err := store.ReadAt(out, 0); err != nil || copied != length {
			return nil, true
		}
		return out, true
	}
	return nil, false
}

func marshalBinaryBuffer(scope *gov8.Scope, realm *gov8.Context, data []byte) (gov8.Value, error) {
	buffer, err := gov8.NewArrayBuffer(scope, realm, len(data))
	if err != nil {
		return gov8.Value{}, err
	}
	if len(data) != 0 {
		store, err := buffer.GetBackingStore()
		if err != nil {
			return gov8.Value{}, err
		}
		// V8 owns the allocation. Drop our counted reference before returning;
		// JS reachability, not a Go handle or pinned Go slice, owns its lifetime.
		_, writeErr := store.WriteAt(data, 0)
		runtime.KeepAlive(data)
		closeErr := store.Close()
		if writeErr != nil {
			return gov8.Value{}, writeErr
		}
		if closeErr != nil {
			return gov8.Value{}, closeErr
		}
	}
	return buffer.Value, nil
}
