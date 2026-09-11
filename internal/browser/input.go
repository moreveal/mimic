package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moreveal/mimic/internal/engine"
)

type inputProtocolError struct{ message string }

func (e *inputProtocolError) Error() string     { return e.message }
func (e *inputProtocolError) ProtocolCode() int { return -32602 }

// DispatchProtocolInput enters the current document's trusted user-interaction
// path. The document owns focus, pressed buttons and editing state, so sessions
// and isolated worlds observe the same input operation and default action.
func (p *Page) DispatchProtocolInput(ctx context.Context, method string, params map[string]any) error {
	r := p.Top.Realm
	if r == nil || r.closed || r.inactive {
		return fmt.Errorf("input document is unavailable")
	}
	operation := ""
	switch method {
	case "Input.dispatchKeyEvent":
		kind, _ := params["type"].(string)
		switch kind {
		case "keyDown", "keyUp", "rawKeyDown", "char":
		default:
			return &inputProtocolError{"Unexpected event type '" + kind + "'"}
		}
		if commands, ok := params["commands"].([]any); ok {
			for _, command := range commands {
				if command != "selectAll" {
					return fmt.Errorf("keyboard editing command %q is not supported", command)
				}
			}
		}
		operation = "key"
	case "Input.insertText":
		operation = "text"
	case "Input.dispatchMouseEvent":
		kind, _ := params["type"].(string)
		switch kind {
		case "mouseMoved", "mousePressed", "mouseReleased":
		case "mouseWheel":
			return fmt.Errorf("wheel scrolling is not supported by the current geometry model")
		default:
			return &inputProtocolError{"Unexpected event type '" + kind + "'"}
		}
		if pointer, _ := params["pointerType"].(string); pointer != "" && pointer != "mouse" {
			return fmt.Errorf("pen input is not supported")
		}
		button, _ := params["button"].(string)
		switch button {
		case "", "none", "left", "middle", "right", "back", "forward":
		default:
			return &inputProtocolError{"Invalid mouse button '" + button + "'"}
		}
		operation = "mouse"
	case "Input.setIgnoreInputEvents":
		p.inputIgnored, _ = params["ignore"].(bool)
		return nil
	default:
		return fmt.Errorf("unsupported input operation %s", method)
	}
	if p.inputIgnored {
		return nil
	}
	if deferred, ok := r.runtime.(*deferredRuntime); ok {
		if _, err := deferred.ready(); err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	p.realmEvaluationDepth++
	defer func() { p.realmEvaluationDepth--; p.collectRealmOwners() }()
	return r.scheduler.RunInline(ctx, func(ctx context.Context) error {
		_, err := r.invokeInputWorld(ctx, r, 0, operation, string(encoded))
		return err
	})
}

func (r *Realm) invokeInputWorld(ctx context.Context, target *Realm, nodeID int64, operation, payload string) (string, error) {
	if target.inputDispatcher == nil || target.closed || target.inactive {
		return "", fmt.Errorf("input dispatcher is unavailable")
	}
	var result string
	invoke := func(ctx context.Context) error {
		return target.runOnOwner(ctx, func(ctx context.Context) error {
			values := []engine.Value{target.val(nodeID), target.val(operation), target.val(payload)}
			for _, value := range values {
				defer releaseDebuggerValue(target, value)
			}
			value, err := target.runtime.Call(ctx, target.inputDispatcher, nil, values...)
			if err != nil {
				return err
			}
			defer releaseDebuggerValue(target, value)
			if value != nil {
				result = value.String()
			}
			return nil
		})
	}
	var err error
	if nested, ok := r.runtime.(engine.ReentrantRuntime); ok && r != target {
		err = nested.RunNested(ctx, invoke)
	} else {
		err = invoke(ctx)
	}
	if err == nil && r != target {
		r.agent.Page().requireCheckpoint(target)
	}
	return result, err
}

func (r *Realm) installProtocolInput(host map[string]any) {
	host["allowContentEventHandler"] = r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		page := r.agent.Page()
		page.mu.RLock()
		allowed := page.bypassCSP || page.policy.AllowsEventHandler(strarg(args, 0))
		page.mu.RUnlock()
		return r.val(allowed), nil
	})
	host["activateProtocolInput"] = r.transientFn(func(engine.Value, []engine.Value) (engine.Value, error) {
		for _, world := range r.documentWorlds() {
			world.activationAt = world.scheduler.Now()
			world.activationConsumed = false
		}
		return nil, nil
	})
	host["isIsolatedInputWorld"] = r.transientFn(func(engine.Value, []engine.Value) (engine.Value, error) {
		return r.val(r.mainWorld != nil), nil
	})
	host["mainWorldInput"] = r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		owner := r
		if r.mainWorld != nil {
			owner = r.mainWorld
		}
		result, err := r.invokeInputWorld(context.Background(), owner, int64(numarg(args, 0)), strarg(args, 1), strarg(args, 2))
		if err != nil {
			return nil, err
		}
		return r.val(result), nil
	})
	host["broadcastInputEvent"] = r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		allowed := true
		for _, world := range r.documentWorlds() {
			if world == r || world.inputDispatcher == nil {
				continue
			}
			result, err := r.invokeInputWorld(context.Background(), world, int64(numarg(args, 0)), "event", strarg(args, 1))
			if err != nil {
				return nil, err
			}
			var response struct{ Allowed bool }
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				return nil, err
			}
			allowed = allowed && response.Allowed
		}
		return r.val(allowed), nil
	})
}
