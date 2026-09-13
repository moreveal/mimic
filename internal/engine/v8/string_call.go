//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"fmt"

	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

func (a *adapter) CallString(ctx context.Context, function engine.Value, args ...any) (string, error) {
	var text string
	_, err := a.runContext(ctx, func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local := func(value any) (gov8.Value, error) {
			if v, ok := value.(engine.Value); ok && a.onCallback() != nil {
				return a.localCallback(v)
			}
			return a.marshal(scope, realm, value)
		}
		fnValue, err := local(function)
		if err != nil {
			return nil, err
		}
		fn, ok, err := gov8.AsFunction(fnValue, realm)
		if err != nil || !ok {
			return nil, fmt.Errorf("value is not callable")
		}
		argv := make([]gov8.Value, len(args))
		for i, argument := range args {
			argv[i], err = local(argument)
			if err != nil {
				return nil, err
			}
		}
		catcher, err := s.isolate.NewTryCatch()
		if err != nil {
			return nil, err
		}
		defer catcher.Close()
		receiver, err := scope.Undefined()
		if err != nil {
			return nil, err
		}
		result, ok, err := fn.Call(scope, receiver, argv...)
		if caught, _ := catcher.HasCaught(); caught {
			return nil, a.callError(catcher, scope, realm)
		}
		if err != nil || !ok {
			return nil, fmt.Errorf("string callback failed: %w", err)
		}
		if isString, _ := result.IsString(); !isString {
			return nil, fmt.Errorf("serialization callback did not return a string")
		}
		text, err = result.StringValue()
		return nil, err
	})
	return text, err
}

var _ engine.StringCallRuntime = (*adapter)(nil)
