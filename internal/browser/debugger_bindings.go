package browser

import (
	"context"
	"fmt"

	"github.com/moreveal/mimic/internal/engine"
)

type debuggerBindingScope struct {
	RealmID, WorldName string
	ByName             bool
}

func (scope debuggerBindingScope) matches(r *Realm) bool {
	if scope.RealmID != "" {
		return scope.RealmID == r.ID
	}
	return !scope.ByName || r.worldName == scope.WorldName
}

func (d *Debugger) AddBinding(ctx context.Context, name, realmID, worldName string, byName bool) error {
	if d.bindings == nil {
		d.bindings = make(map[string][]debuggerBindingScope)
	}
	scope := debuggerBindingScope{realmID, worldName, byName}
	found := false
	for _, existing := range d.bindings[name] {
		found = found || existing == scope
	}
	if !found {
		d.bindings[name] = append(d.bindings[name], scope)
	}
	for _, r := range d.page.evaluationRealms(nil) {
		if r == nil || !scope.matches(r) {
			continue
		}
		if deferred, ok := r.runtime.(*deferredRuntime); ok {
			if _, err := deferred.ready(); err != nil {
				return err
			}
		}
		if err := r.installDebuggerBinding(name); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (d *Debugger) RemoveBinding(name string) { delete(d.bindings, name) }

func (r *Realm) installDebuggerBindings() error {
	for debugger := range r.agent.Page().debuggers {
		for name, scopes := range debugger.bindings {
			for _, scope := range scopes {
				if scope.matches(r) {
					if err := r.installDebuggerBinding(name); err != nil {
						return err
					}
					break
				}
			}
		}
	}
	return nil
}

func (r *Realm) installDebuggerBinding(name string) error {
	if r.debuggerBindings == nil {
		r.debuggerBindings = make(map[string]bool)
	}
	if r.debuggerBindings[name] {
		return nil
	}
	function := r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		if len(args) != 1 || r.runtime.TypeOf(args[0]) != "string" {
			return nil, fmt.Errorf("Invalid arguments: should be exactly one string.")
		}
		payload := args[0].String()
		for debugger := range r.agent.Page().debuggers {
			if debugger.BindingCalled == nil {
				continue
			}
			for _, scope := range debugger.bindings[name] {
				if scope.matches(r) {
					debugger.BindingCalled(r.ID, name, payload)
					break
				}
			}
		}
		return nil, nil
	})
	if err := r.runtime.Set(name, function); err != nil {
		return err
	}
	r.debuggerBindings[name] = true
	return nil
}
