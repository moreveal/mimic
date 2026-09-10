//go:build windows && amd64

package v8

import (
	"fmt"
	"github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

type cloneErrorReporter struct {
	message string
	scope   *gov8.Scope
	reject  *gov8.Function
}

func (d *cloneErrorReporter) HasCustomHostObject() bool { return d.reject != nil }
func (d *cloneErrorReporter) GetSharedArrayBufferID(*gov8.SharedArrayBuffer) (uint32, bool) {
	d.message = "SharedArrayBuffer cannot be stored in history."
	return 0, false
}
func (d *cloneErrorReporter) GetWasmModuleTransferID(gov8.Value) (uint32, bool) {
	d.message = "WebAssembly.Module cannot be stored in history."
	return 0, false
}
func (d *cloneErrorReporter) IsHostObject(object *gov8.Object) (bool, bool) {
	result, ok, err := d.reject.Call(d.scope, object.Value, object.Value)
	if err != nil || !ok {
		return false, false
	}
	reject, err := result.BooleanValue()
	if err != nil {
		return false, false
	}
	if reject {
		d.message = "The platform object could not be cloned."
		return false, false
	}
	return false, true
}

func (d *cloneErrorReporter) ThrowDataCloneError(message string) bool {
	d.message = message
	return false
}

func (a *adapter) StructuredClone(value engine.Value, rejectHostObject engine.Value) (engine.Value, error) {
	clone := func(iso *gov8.Isolate, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		input, err := a.local(scope, value)
		if err != nil {
			return nil, err
		}
		tc, err := iso.NewTryCatch()
		if err != nil {
			return nil, err
		}
		defer tc.Close()
		reporter := &cloneErrorReporter{scope: scope}
		if rejectHostObject != nil {
			predicate, err := a.local(scope, rejectHostObject)
			if err != nil {
				return nil, err
			}
			reporter.reject, _, err = gov8.AsFunction(predicate, realm)
			if err != nil {
				return nil, err
			}
		}
		serializer, err := gov8.NewDelegateValueSerializer(scope, realm, reporter)
		if err != nil {
			return nil, err
		}
		defer serializer.Close()
		if err = serializer.WriteHeader(); err != nil {
			return nil, err
		}
		ok, err := serializer.WriteValue(realm, input, tc)
		if reporter.message != "" {
			return nil, &engine.DataCloneError{Message: reporter.message}
		}
		if caught, _ := tc.HasCaught(); caught {
			return nil, a.callError(tc, scope, realm)
		}
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, &engine.DataCloneError{Message: "The value could not be cloned."}
		}
		bytes, err := serializer.Release()
		if err != nil {
			return nil, err
		}
		deserializer, err := gov8.NewValueDeserializer(scope, realm, bytes)
		if err != nil {
			return nil, err
		}
		defer deserializer.Close()
		ok, err = deserializer.ReadHeader(realm)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("invalid structured clone header")
		}
		output, err := deserializer.ReadValue(realm, tc)
		if err != nil {
			return nil, err
		}
		return a.persist(scope, output)
	}
	if callback := a.onCallback(); callback != nil {
		return clone(callback.scope.Isolate(), callback.ctx, callback.scope.Scope())
	}
	return a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		return clone(s.isolate, realm, scope)
	})
}
