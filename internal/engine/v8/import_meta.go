//go:build windows && amd64

package v8

import (
	"fmt"

	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

func (a *adapter) SetImportMetaResolveFactory(factory engine.Value) error {
	_, err := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		value, err := a.local(scope, factory)
		if err != nil {
			return nil, err
		}
		if callable, err := value.IsFunction(); err != nil || !callable {
			return nil, fmt.Errorf("import.meta resolver factory must be a function")
		}
		a.importMetaResolveFactory, err = a.persist(scope, value)
		return nil, err
	})
	return err
}
