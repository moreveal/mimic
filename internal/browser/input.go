package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moreveal/mimic/internal/engine"
)

type inputProtocolError struct{ message string }

type inputFrameRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ScrollNodeIntoView is a browser operation, not an invocation of a replaceable
// author method. The node's document owner shares state with all CDP worlds.
func (p *Page) ScrollNodeIntoView(ctx context.Context, nodeID int64, rect any) error {
	frame, ok := p.FrameForDOMNode(nodeID)
	if !ok || frame.Realm == nil {
		return fmt.Errorf("Node is detached from document")
	}
	r := frame.Realm
	payload, err := json.Marshal(map[string]any{"action": "protocolInto", "opts": map[string]any{"block": "center", "inline": "center", "behavior": "instant", "rect": rect}})
	if err != nil {
		return err
	}
	return r.scheduler.RunInline(ctx, func(ctx context.Context) error {
		_, err := r.invokeInputWorld(ctx, r, nodeID, "scroll", string(payload))
		return err
	})
}

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
		case "mouseMoved", "mousePressed", "mouseReleased", "mouseWheel":
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
		target := r
		if operation == "mouse" {
			var err error
			target, params, err = p.mouseInputTarget(ctx, target, params)
			if err != nil {
				return err
			}
			encoded, err = json.Marshal(params)
			if err != nil {
				return err
			}
		}
		_, err := r.invokeInputWorld(ctx, target, 0, operation, string(encoded))
		return err
	})
}

// mouseInputTarget maps top-level CDP coordinates through the active iframe
// tree. CDP exposes one input surface per Page even though DOM handles belong
// to individual frame realms.
func (p *Page) mouseInputTarget(ctx context.Context, owner *Realm, params map[string]any) (*Realm, map[string]any, error) {
	x, xOK := numberParameter(params["x"])
	y, yOK := numberParameter(params["y"])
	if !xOK || !yOK {
		return owner, params, nil
	}
	frame, _ := owner.agent.(*Frame)
	if frame == nil {
		return owner, params, nil
	}
	for _, child := range frame.Children() {
		if child.Realm == nil || child.ElementNodeID() == 0 {
			continue
		}
		raw, err := owner.invokeInputWorld(ctx, owner, child.ElementNodeID(), "rect", "{}")
		if err != nil {
			return nil, nil, err
		}
		var rect inputFrameRect
		if err := json.Unmarshal([]byte(raw), &rect); err != nil {
			return nil, nil, err
		}
		if rect.Width <= 0 || rect.Height <= 0 || x < rect.X || y < rect.Y || x >= rect.X+rect.Width || y >= rect.Y+rect.Height {
			continue
		}
		local := make(map[string]any, len(params))
		for key, value := range params {
			local[key] = value
		}
		local["x"], local["y"] = x-rect.X, y-rect.Y
		return p.mouseInputTarget(ctx, child.Realm, local)
	}
	return owner, params, nil
}

func numberParameter(value any) (float64, bool) {
	switch value := value.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}

func (r *Realm) invokeInputWorld(ctx context.Context, target *Realm, nodeID int64, operation, payload string) (string, error) {
	if target.closed || target.inactive {
		return "", fmt.Errorf("input dispatcher is unavailable")
	}
	var result string
	invoke := func(ctx context.Context) error {
		// An isolated world can observe CSS/input before the main world runs JS.
		if deferred, ok := target.runtime.(*deferredRuntime); ok {
			if _, err := deferred.ready(); err != nil {
				return err
			}
		}
		return target.runOnOwner(ctx, func(ctx context.Context) error {
			if target.inputDispatcher == nil {
				return fmt.Errorf("input dispatcher is unavailable")
			}
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
	host["recordScrollPosition"] = r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		p.mu.Lock()
		if p.historyIndex >= 0 && p.historyIndex < len(p.history) {
			if state := p.history[p.historyIndex].frames[r.agent.ContextID()]; state != nil && state.realmID == r.ID {
				state.scrollX, state.scrollY = numarg(args, 0), numarg(args, 1)
			}
		}
		p.mu.Unlock()
		return nil, nil
	})
	host["scrollParentFrame"] = r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		frame, ok := r.agent.(*Frame)
		if !ok || frame.parent == nil || frame.parent.Realm == nil {
			return nil, nil
		}
		_, err := r.invokeInputWorld(context.Background(), frame.parent.Realm, frame.elementID, "scroll", strarg(args, 0))
		return nil, err
	})
	host["invalidateStyleObservations"] = r.transientFn(func(engine.Value, []engine.Value) (engine.Value, error) {
		r.document.InvalidateObservations()
		return nil, nil
	})
	host["allowContentEventHandler"] = r.transientFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		page := r.agent.Page()
		allowed := page.cspBypassed() || r.contentPolicy().AllowsEventHandler(strarg(args, 0))
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
