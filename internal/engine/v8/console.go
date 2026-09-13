//go:build (windows || linux) && amd64

package v8

import (
	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

func (a *adapter) ConsoleValueKind(value engine.Value) (string, error) {
	kind := ""
	_, err := a.withCloneScope(func(_ *gov8.Isolate, _ *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		v, err := a.local(scope, value)
		if err != nil {
			return nil, err
		}
		if proxy, err := v.IsProxy(); err != nil || proxy {
			return nil, err
		}
		for _, test := range []struct {
			name  string
			check func() (bool, error)
		}{
			{"function", v.IsFunction}, {"date", v.IsDate}, {"regexp", v.IsRegExp}, {"error", v.IsNativeError},
		} {
			match, err := test.check()
			if err != nil {
				return nil, err
			}
			if match {
				kind = test.name
				break
			}
		}
		return nil, nil
	})
	return kind, err
}
