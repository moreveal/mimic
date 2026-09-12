//go:build windows && amd64

package v8

import (
	"fmt"

	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

// Install after snapshot initialization: Go callbacks belong to the live realm,
// whereas the weak source registry and original intrinsic belong in its snapshot.
func (a *adapter) InstallNativeFunctionToString(resolver engine.Value, original engine.Value) error {
	_, err := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		fn, err := s.isolate.FunctionBuilder(func(cs *gov8.CallbackScope, args gov8.FunctionCallbackArguments, rv gov8.ReturnValue) {
			tc, catchErr := s.isolate.NewTryCatch()
			if catchErr != nil {
				exception, err := cs.NewError(catchErr.Error())
				if err == nil {
					_ = cs.ThrowException(exception)
				}
				return
			}
			defer tc.Close()
			invoke := func() error {
				receiver, err := args.This()
				if err != nil {
					return err
				}
				callable, err := receiver.Value.IsFunction()
				if err != nil {
					return err
				}
				callOriginal := func() error {
					originalValue, err := a.local(cs.Scope(), original)
					if err != nil {
						return err
					}
					intrinsic, ok, err := gov8.AsFunction(originalValue, realm)
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("original Function.toString is not callable")
					}
					result, ok, err := intrinsic.Call(cs.Scope(), receiver.Value)
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("original Function.toString returned no result")
					}
					return rv.Set(result)
				}
				// Let V8's original intrinsic produce receiver TypeErrors and
				// native stack frames. Never construct/rewrite a replacement stack.
				if !callable {
					return callOriginal()
				}
				resolveValue, err := a.local(cs.Scope(), resolver)
				if err != nil {
					return err
				}
				resolve, ok, err := gov8.AsFunction(resolveValue, realm)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("native function source resolver is not callable")
				}
				undefined, err := cs.Scope().Undefined()
				if err != nil {
					return err
				}
				source, ok, err := resolve.Call(cs.Scope(), undefined, receiver.Value)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("native function source resolver returned no result")
				}
				marked, err := source.IsString()
				if err != nil {
					return err
				}
				if marked {
					return rv.Set(source)
				}
				return callOriginal()
			}
			invokeErr := invoke()
			caught, catchErr := tc.HasCaught()
			if caught {
				// gov8 Function.Call reports an empty MaybeLocal as ok=false,
				// sometimes with no Go error. ReThrow preserves the actual JS
				// exception and its native frames instead of substituting Error.
				_, _, err := tc.ReThrow(cs.Scope())
				if err != nil {
					exception, createErr := cs.NewError(err.Error())
					if createErr == nil {
						_ = tc.Close()
						_ = cs.ThrowException(exception)
					}
				}
				return
			}
			if invokeErr == nil {
				invokeErr = catchErr
			}
			if invokeErr != nil {
				exception, createErr := cs.NewError(invokeErr.Error())
				if createErr == nil {
					_ = tc.Close()
					_ = cs.ThrowException(exception)
				}
			}
		}).Length(0).ConstructorBehavior(gov8.ConstructorBehaviorThrow).Build(scope, realm)
		if err != nil {
			return nil, err
		}
		if err := fn.SetName("toString"); err != nil {
			return nil, err
		}
		replacement, err := a.persist(scope, fn.Value)
		if err != nil {
			return nil, err
		}
		prototype := a.GetProperty(a.Get("Function"), "prototype")
		if prototype == nil {
			return nil, fmt.Errorf("missing Function.prototype")
		}
		return nil, a.SetProperty(prototype, "toString", replacement)
	})
	return err
}
