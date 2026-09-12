package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
)

// Debugger owns the object groups for one protocol session. All operations,
// including Close, run under the Page command boundary. Live JavaScript values
// stay in their realm; protocol serialization never exports a user object to Go.
type Debugger struct {
	page       *Page
	id         string
	realms     map[string]*debuggerRealm
	exceptions int64
	// Pending promises release the command boundary only while waiting for a
	// Page task. The embedding supplies both hooks, or neither, and reacquires
	// exactly the locks it released before the debugger touches realm state.
	BeforeWait     func()
	AfterWait      func()
	Console        func(realmID, name string, args []any)
	ConsoleEnabled func() bool
	BindingCalled  func(realmID, name, payload string)
	bindings       map[string][]debuggerBindingScope
}

type debuggerRealm struct {
	realm   *Realm
	frameID string
	bridge  engine.Value
}

type DebuggerOptions struct {
	ObjectGroup   string
	ReturnByValue bool
	AwaitPromise  bool
}

func NewDebugger(page *Page) *Debugger {
	d := &Debugger{page: page, id: uuid.NewString(), realms: make(map[string]*debuggerRealm)}
	if page.debuggers == nil {
		page.debuggers = make(map[*Debugger]struct{})
	}
	page.debuggers[d] = struct{}{}
	return d
}

func (p *Page) waitDebuggerProgress() <-chan struct{} {
	p.debuggerWaitMu.Lock()
	defer p.debuggerWaitMu.Unlock()
	if p.debuggerProgress == nil {
		p.debuggerProgress = make(chan struct{})
	}
	return p.debuggerProgress
}

func (p *Page) notifyDebuggerProgress() {
	p.debuggerWaitMu.Lock()
	if p.debuggerProgress != nil {
		close(p.debuggerProgress)
		p.debuggerProgress = nil
	}
	p.debuggerWaitMu.Unlock()
}

func releaseDebuggerValue(r *Realm, v engine.Value) {
	if v != nil && !r.closed {
		if owner, ok := r.runtime.(engine.ValueReleaser); ok {
			owner.ReleaseValue(v)
		}
	}
}

func (d *Debugger) Close() {
	delete(d.page.debuggers, d)
	for id, state := range d.realms {
		releaseDebuggerValue(state.realm, state.bridge)
		delete(d.realms, id)
	}
}

// Prune drops session roots for destroyed execution contexts, including a
// document realm which remains alive only through a page-script reference.
func (d *Debugger) Prune() {
	for id, state := range d.realms {
		if !d.alive(state) {
			releaseDebuggerValue(state.realm, state.bridge)
			delete(d.realms, id)
		}
	}
}

func (d *Debugger) alive(state *debuggerRealm) bool {
	frame, ok := d.page.Frame(state.frameID)
	if !ok || frame.Realm == nil || state.realm.closed || state.realm.inactive {
		return false
	}
	return frame.Realm == state.realm || state.realm.mainWorld == frame.Realm
}

func (d *Debugger) state(ctx context.Context, frameID, realmID string) (*debuggerRealm, error) {
	if frameID == "" {
		frameID = d.page.Top.ID
	}
	frame, ok := d.page.Frame(frameID)
	if !ok || frame.Realm == nil || frame.Realm.closed || frame.Realm.inactive {
		return nil, fmt.Errorf("Cannot find context with specified id")
	}
	r := frame.Realm
	if realmID != "" && realmID != r.ID {
		r = nil
		for _, world := range frame.Realm.isolatedWorlds {
			if world.ID == realmID {
				r = world
				break
			}
		}
		if r == nil {
			return nil, fmt.Errorf("Cannot find context with specified id")
		}
	}
	if old := d.realms[r.ID]; old != nil {
		return old, nil
	}
	d.Prune()
	if deferred, ok := r.runtime.(*deferredRuntime); ok {
		if _, err := deferred.ready(); err != nil {
			return nil, err
		}
	}
	prefix := r.val(d.id + "." + r.ID)
	defer releaseDebuggerValue(r, prefix)
	bridge, err := r.runtime.Call(ctx, r.debuggerFactory, nil, r.frameNodeDescribe, prefix)
	if err != nil {
		return nil, err
	}
	state := &debuggerRealm{realm: r, frameID: frameID, bridge: bridge}
	d.realms[r.ID] = state
	return state, nil
}

func (d *Debugger) objectState(id string) (*debuggerRealm, error) {
	parts := strings.SplitN(id, ".", 3)
	if len(parts) != 3 || parts[0] != d.id {
		return nil, fmt.Errorf("Could not find object with given id")
	}
	state := d.realms[parts[1]]
	if state == nil || !d.alive(state) {
		return nil, fmt.Errorf("Could not find object with given id")
	}
	return state, nil
}

func (state *debuggerRealm) invoke(ctx context.Context, operation string, params any, value engine.Value) (engine.Value, error) {
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	operationValue, paramsValue := state.realm.val(operation), state.realm.val(string(encoded))
	defer releaseDebuggerValue(state.realm, operationValue)
	defer releaseDebuggerValue(state.realm, paramsValue)
	if value == nil {
		return state.realm.runtime.Call(ctx, state.bridge, nil, operationValue, paramsValue)
	}
	return state.realm.runtime.Call(ctx, state.bridge, nil, operationValue, paramsValue, value)
}

func (state *debuggerRealm) json(ctx context.Context, operation string, params any, value engine.Value) (map[string]any, error) {
	result, err := state.invoke(ctx, operation, params, value)
	if err != nil {
		return nil, err
	}
	defer releaseDebuggerValue(state.realm, result)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.String()), &decoded); err != nil {
		return nil, fmt.Errorf("debugger serialization: %w", err)
	}
	return decoded, nil
}

func (d *Debugger) enter() func() {
	d.page.realmEvaluationDepth++
	return func() { d.page.realmEvaluationDepth--; d.page.collectRealmOwners() }
}

func (d *Debugger) Evaluate(ctx context.Context, frameID, realmID, source string, options DebuggerOptions) (map[string]any, error) {
	defer d.enter()()
	state, err := d.state(ctx, frameID, realmID)
	if err != nil {
		return nil, err
	}
	var value engine.Value
	err = state.realm.scheduler.RunInline(ctx, func(ctx context.Context) error {
		var err error
		value, err = state.realm.Evaluate(ctx, source, "__pyppeteer_evaluation_script__")
		return err
	})
	if err != nil {
		return d.exception(ctx, state, err, options, false)
	}
	defer releaseDebuggerValue(state.realm, value)
	return d.finish(ctx, state, value, options)
}

func (d *Debugger) CallFunction(ctx context.Context, frameID, realmID, declaration string, params map[string]any, options DebuggerOptions) (map[string]any, error) {
	defer d.enter()()
	var state *debuggerRealm
	var err error
	if objectID, _ := params["objectId"].(string); objectID != "" {
		state, err = d.objectState(objectID)
		if err == nil && (frameID != "" && frameID != state.frameID || realmID != "" && realmID != state.realm.ID) {
			err = fmt.Errorf("Object belongs to a different JavaScript world than target execution context")
		}
		if err == nil {
			if _, explicit := params["objectGroup"]; !explicit {
				group, e := state.json(ctx, "group", params, nil)
				if e != nil {
					return nil, e
				}
				options.ObjectGroup, _ = group["group"].(string)
			}
		}
	} else {
		state, err = d.state(ctx, frameID, realmID)
	}
	if err != nil {
		return nil, err
	}
	if arguments, ok := params["arguments"].([]any); ok {
		for _, argument := range arguments {
			if arg, ok := argument.(map[string]any); ok {
				if objectID, _ := arg["objectId"].(string); objectID != "" {
					owner, err := d.objectState(objectID)
					if err != nil {
						return nil, err
					}
					if owner != state {
						return nil, fmt.Errorf("Argument should belong to the same JavaScript world as target object")
					}
				}
			}
		}
	}
	var value engine.Value
	err = state.realm.scheduler.RunInline(ctx, func(ctx context.Context) error {
		function, err := state.realm.Evaluate(ctx, "(\n"+declaration+"\n)", "__pyppeteer_evaluation_script__")
		if err != nil {
			return err
		}
		defer releaseDebuggerValue(state.realm, function)
		if state.realm.runtime.TypeOf(function) != "function" {
			return fmt.Errorf("Given expression does not evaluate to a function")
		}
		value, err = state.invoke(ctx, "call", params, function)
		return err
	})
	if err != nil {
		return d.exception(ctx, state, err, options, false)
	}
	defer releaseDebuggerValue(state.realm, value)
	return d.finish(ctx, state, value, options)
}

func (d *Debugger) finish(ctx context.Context, state *debuggerRealm, value engine.Value, options DebuggerOptions) (map[string]any, error) {
	if options.AwaitPromise {
		resolved, err := d.await(ctx, state, value)
		if err != nil {
			return d.exception(ctx, state, err, options, true)
		}
		defer releaseDebuggerValue(state.realm, resolved)
		value = resolved
	}
	if !d.alive(state) {
		return nil, fmt.Errorf("Execution context was destroyed")
	}
	return state.json(ctx, "hold", map[string]any{"objectGroup": options.ObjectGroup, "returnByValue": options.ReturnByValue}, value)
}

func (d *Debugger) await(ctx context.Context, state *debuggerRealm, value engine.Value) (engine.Value, error) {
	for {
		if !d.alive(state) {
			return nil, fmt.Errorf("Execution context was destroyed")
		}
		resolved, done, err := state.realm.runtime.Await(value)
		if err != nil || done {
			return resolved, err
		}
		if d.BeforeWait != nil && d.AfterWait != nil {
			progress := d.page.waitDebuggerProgress()
			d.BeforeWait()
			select {
			case <-progress:
			case <-ctx.Done():
				err = ctx.Err()
			}
			d.AfterWait()
			if !d.alive(state) {
				return nil, fmt.Errorf("Execution context was destroyed")
			}
			if err != nil {
				return nil, err
			}
			continue
		}
		queues := make([]*scheduler.Scheduler, 0)
		for _, r := range d.page.evaluationRealms(state.realm) {
			queues = append(queues, r.scheduler)
		}
		err = scheduler.WaitAny(ctx, queues)
		if !d.alive(state) {
			return nil, fmt.Errorf("Execution context was destroyed")
		}
		if err != nil {
			return nil, err
		}
		if err := d.page.runEvaluationTasks(ctx, state.realm); err != nil {
			return nil, err
		}
	}
}

func (d *Debugger) exception(ctx context.Context, state *debuggerRealm, err error, options DebuggerOptions, promise bool) (map[string]any, error) {
	var thrown engine.ThrownValue
	if !errors.As(err, &thrown) {
		return nil, err
	}
	value := thrown.ThrownValue()
	defer releaseDebuggerValue(state.realm, value)
	result, describeErr := state.json(ctx, "hold", map[string]any{"objectGroup": options.ObjectGroup}, value)
	if describeErr != nil {
		return nil, describeErr
	}
	d.exceptions++
	text := "Uncaught"
	if promise {
		text = "Uncaught (in promise)"
	}
	result["exceptionDetails"] = map[string]any{"exceptionId": d.exceptions, "text": text, "lineNumber": 0, "columnNumber": 0, "exception": result["result"]}
	return result, nil
}

func (d *Debugger) AwaitPromise(ctx context.Context, objectID string, options DebuggerOptions) (map[string]any, error) {
	defer d.enter()()
	state, err := d.objectState(objectID)
	if err != nil {
		return nil, err
	}
	value, err := state.invoke(ctx, "lookup", map[string]any{"objectId": objectID}, nil)
	if err != nil {
		return nil, err
	}
	defer releaseDebuggerValue(state.realm, value)
	options.AwaitPromise = true
	return d.finish(ctx, state, value, options)
}

func (d *Debugger) GetProperties(ctx context.Context, params map[string]any) (map[string]any, error) {
	objectID, _ := params["objectId"].(string)
	state, err := d.objectState(objectID)
	if err != nil {
		return nil, err
	}
	return state.json(ctx, "properties", params, nil)
}

func (d *Debugger) ReleaseObject(ctx context.Context, objectID string) error {
	state, err := d.objectState(objectID)
	if err != nil {
		return nil
	} // Chrome permits releasing an already released handle.
	_, err = state.json(ctx, "release", map[string]any{"objectId": objectID}, nil)
	return err
}

func (d *Debugger) ReleaseObjectGroup(ctx context.Context, group string) error {
	for _, state := range d.realms {
		if !d.alive(state) {
			continue
		}
		if _, err := state.json(ctx, "releaseGroup", map[string]any{"objectGroup": group}, nil); err != nil {
			return err
		}
	}
	return nil
}

// ResolveNode imports the DOM's canonical wrapper into the selected realm.
func (d *Debugger) ResolveNode(ctx context.Context, frameID, realmID string, nodeID int64, group string) (map[string]any, error) {
	state, err := d.state(ctx, frameID, realmID)
	if err != nil {
		return nil, err
	}
	if _, ok := state.realm.document.Get(nodeID); !ok {
		return nil, fmt.Errorf("Could not find node with given id")
	}
	result, err := state.json(ctx, "resolve", map[string]any{"nodeId": nodeID, "objectGroup": group}, nil)
	if err != nil {
		return nil, err
	}
	return result["result"].(map[string]any), nil
}

func (d *Debugger) RequestNode(ctx context.Context, objectID string) (int64, error) {
	state, err := d.objectState(objectID)
	if err != nil {
		return 0, err
	}
	result, err := state.json(ctx, "node", map[string]any{"objectId": objectID}, nil)
	if err != nil {
		return 0, err
	}
	if id, ok := result["nodeId"].(float64); ok {
		return int64(id), nil
	}
	return 0, nil
}
