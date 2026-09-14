//go:build (windows || linux) && amd64

package v8

import (
	"runtime"

	gov8 "github.com/maclof/gov8"
)

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
