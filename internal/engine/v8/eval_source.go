//go:build windows && amd64

package v8

import (
	"fmt"

	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

// SetEvalSourceResolver retains a realm-owned brand check, not a second registry
// of trusted values. V8 still owns compilation, direct/indirect eval scope and
// exceptions. Ordinary string evals take V8's existing fast path.
func (a *adapter) SetEvalSourceResolver(resolver engine.Value) error {
	_, err := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.local(scope, resolver)
		if err != nil {
			return nil, err
		}
		if callable, err := local.IsFunction(); err != nil || !callable {
			return nil, fmt.Errorf("eval source resolver must be a function")
		}
		return nil, s.isolate.SetModifyCodeGenerationFromStringsCallback(func(source gov8.Value, _ bool) (bool, *string) {
			if object, err := source.IsObject(); err != nil || !object {
				return err == nil, nil
			}
			// This callback already runs on the owning isolate thread. A nested
			// scope bounds all temporary handles to this one eval operation.
			scope, err := s.isolate.NewScope()
			if err != nil {
				return false, nil
			}
			defer scope.Close()
			local, err := a.local(scope, resolver)
			if err != nil {
				return false, nil
			}
			fn, ok, err := gov8.AsFunction(local, realm)
			if err != nil || !ok {
				return false, nil
			}
			receiver, err := scope.Undefined()
			if err != nil {
				return false, nil
			}
			result, ok, err := fn.Call(scope, receiver, source)
			if err != nil || !ok {
				return false, nil
			}
			if isString, err := result.IsString(); err != nil || !isString {
				return err == nil, nil
			}
			text, err := result.StringValue()
			if err != nil {
				return false, nil
			}
			return true, &text
		})
	})
	return err
}
