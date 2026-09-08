//go:build windows && amd64

// Package v8 implements the primary engine on a thread-affine isolate owner.
// The low-level Realm helpers are also used by the engine differential harness.
package v8

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	gov8 "github.com/maclof/gov8"
)

type HostFunction func(args []string) (any, error)

type response struct {
	value any
	err   error
}

type command func(*state) response

type state struct {
	isolate             *gov8.Isolate
	realms              map[uint64]*gov8.Context
	next                uint64
	restoreThreadPolicy func() error
}

// Runtime owns one V8 isolate on a dedicated, permanently thread-affine
// goroutine. This is necessary on Windows: V8 isolates cannot follow a Go
// goroutine when it migrates between OS threads.
type Runtime struct {
	commands chan command
	done     chan struct{}
	once     sync.Once
	actorTID uint32
}

type Realm struct {
	runtime *Runtime
	id      uint64
}

var initialize = sync.OnceValues(func() (bool, error) {
	return true, gov8.Initialize()
})

func NewRuntime() (*Runtime, error) {
	if _, err := initialize(); err != nil {
		return nil, err
	}
	r := &Runtime{commands: make(chan command), done: make(chan struct{})}
	ready := make(chan error, 1)
	go r.loop(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Runtime) loop(ready chan<- error) {
	// Each Page has an independent isolate. Process-sized nursery defaults
	// retain 16 MiB per tiny Page after bootstrap with under 2 MiB in use.
	// Bound only the young generation; the old generation keeps V8 defaults.
	iso, err := gov8.NewIsolateWithParams(gov8.NewCreateParams().SetMaxYoungGenerationSizeInBytes(4 << 20))
	if err != nil {
		ready <- err
		close(r.done)
		return
	}
	r.actorTID = currentThreadID() // NewIsolate has locked the OS thread.
	// Chrome performs Promise jobs at browser event-loop microtask checkpoints,
	// not whenever an arbitrary embedder call happens to return. Leaving V8's
	// default kAuto policy enabled lets microtasks run inside Script::Run and
	// host Function::Call, bypassing Mimic's scheduler and deadlocking async Web
	// APIs whose completion is queued as a browser task.
	if err := iso.SetMicrotasksPolicy(gov8.PolicyExplicit); err != nil {
		_ = iso.Close()
		ready <- err
		close(r.done)
		return
	}
	s := &state{isolate: iso, realms: make(map[uint64]*gov8.Context), restoreThreadPolicy: configurePageThreadPolicy()}
	ready <- nil
	for command := range r.commands {
		result := command(s)
		if result.err != nil && errors.Is(result.err, errDispose) {
			break
		}
	}
	close(r.done)
}

func (r *Runtime) execute(command command) (any, error) {
	select {
	case <-r.done:
		return nil, errors.New("V8 runtime is closed")
	default:
	}
	replies := make(chan response, 1)
	select {
	case r.commands <- func(s *state) response {
		result := command(s)
		replies <- result
		return result
	}:
	case <-r.done:
		return nil, errors.New("V8 runtime is closed")
	}
	result := <-replies
	return result.value, result.err
}

func (r *Runtime) NewRealm() (*Realm, error) {
	value, err := r.execute(func(s *state) response {
		ctx, err := s.isolate.NewContext()
		if err != nil {
			return response{err: err}
		}
		s.next++
		s.realms[s.next] = ctx
		return response{value: s.next}
	})
	if err != nil {
		return nil, err
	}
	return &Realm{runtime: r, id: value.(uint64)}, nil
}

// Eval evaluates an expression and returns its JSON-compatible observable
// value. The differential corpus intentionally consists of expressions whose
// results can cross CDP by value, so this is also the right spike boundary.
func (r *Realm) Eval(source, name string) (any, error) {
	return r.runtime.execute(func(s *state) response {
		ctx := s.realms[r.id]
		if ctx == nil {
			return response{err: errors.New("V8 realm is closed")}
		}
		encodedSource, _ := json.Marshal(source)
		// Indirect eval executes as a global script and returns the script's
		// completion value. Encoding the source also makes arbitrary statements
		// valid here, rather than limiting the spike to parenthesized expressions.
		wrapped := "JSON.stringify((0,eval)(" + string(encodedSource) + "))"
		text, err := evalText(s.isolate, ctx, wrapped, name)
		if err != nil {
			return response{err: err}
		}
		if text == "undefined" || text == "" {
			return response{}
		}
		var value any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			return response{err: fmt.Errorf("decode V8 result: %w", err)}
		}
		return response{value: value}
	})
}

// SetHostObject installs a plain host object. HostFunction members execute on
// the isolate thread; primitive members are copied by value.
func (r *Realm) SetHostObject(name string, members map[string]any) error {
	_, err := r.runtime.execute(func(s *state) response {
		ctx := s.realms[r.id]
		scope, err := s.isolate.NewScope()
		if err != nil {
			return response{err: err}
		}
		defer scope.Close()
		object, err := scope.NewObject(ctx)
		if err != nil {
			return response{err: err}
		}
		for key, member := range members {
			var value gov8.Value
			switch member := member.(type) {
			case HostFunction:
				function, makeErr := s.isolate.NewFunction(scope, ctx, func(cs *gov8.CallbackScope, args gov8.FunctionCallbackArguments, rv gov8.ReturnValue) {
					strings := make([]string, args.Length())
					for i := range strings {
						argument, getErr := args.Get(i)
						if getErr != nil {
							return
						}
						strings[i], _ = cs.ToString(argument)
					}
					result, callErr := member(strings)
					if callErr != nil {
						exception, _ := cs.NewError(callErr.Error())
						_ = cs.ThrowException(exception)
						return
					}
					setCallbackResult(cs, rv, result)
				}, nil)
				if makeErr != nil {
					return response{err: makeErr}
				}
				value = function.Value
			default:
				value, err = primitive(scope, member)
				if err != nil {
					return response{err: err}
				}
			}
			ok, setErr := object.SetByName(scope, ctx, key, value)
			if setErr != nil || !ok {
				return response{err: fmt.Errorf("set host member %s: %w", key, setErr)}
			}
		}
		global, err := ctx.GlobalObject(scope)
		if err != nil {
			return response{err: err}
		}
		ok, err := global.SetByName(scope, ctx, name, object.Value)
		if err != nil || !ok {
			return response{err: fmt.Errorf("set host object %s: %w", name, err)}
		}
		return response{}
	})
	return err
}

func (r *Realm) MicrotaskCheckpoint() error {
	_, err := r.runtime.execute(func(s *state) response {
		return response{err: s.isolate.PerformMicrotaskCheckpoint()}
	})
	return err
}

func (r *Realm) Dispose() error {
	_, err := r.runtime.execute(func(s *state) response {
		ctx := s.realms[r.id]
		if ctx == nil {
			return response{}
		}
		delete(s.realms, r.id)
		return response{err: ctx.Close()}
	})
	return err
}

var errDispose = errors.New("dispose V8 runtime")

func (r *Runtime) Dispose() error {
	var disposeErr error
	r.once.Do(func() {
		_, dispatchErr := r.execute(func(s *state) response {
			for id, ctx := range s.realms {
				if err := ctx.Close(); err != nil && disposeErr == nil {
					disposeErr = err
				}
				delete(s.realms, id)
			}
			// All contexts and adapter roots have been released. Ask V8 to
			// return empty heap pages before disposal transfers them to its
			// process-wide allocator pools.
			if err := s.isolate.LowMemoryNotification(); err != nil && disposeErr == nil {
				disposeErr = err
			}
			// Isolate.Close disposes V8, but gov8's Go callback registry has
			// an explicit lifetime. Release it on the owner thread while the
			// native isolate still exists; otherwise callbacks retain closed
			// Realms, their DOM, response bodies and traces indefinitely.
			if err := gov8.ReleaseIsolateHostState(s.isolate); err != nil && disposeErr == nil {
				disposeErr = err
			}
			if s.restoreThreadPolicy != nil {
				if err := s.restoreThreadPolicy(); err != nil && disposeErr == nil {
					disposeErr = err
				}
				s.restoreThreadPolicy = nil
			}
			if err := s.isolate.Close(); err != nil && disposeErr == nil {
				disposeErr = err
			}
			return response{err: errDispose}
		})
		if dispatchErr != nil && !errors.Is(dispatchErr, errDispose) && disposeErr == nil {
			disposeErr = dispatchErr
		}
		<-r.done
	})
	return disposeErr
}

func evalText(iso *gov8.Isolate, ctx *gov8.Context, source, name string) (string, error) {
	scope, err := iso.NewScope()
	if err != nil {
		return "", err
	}
	defer scope.Close()
	catcher, err := iso.NewTryCatch()
	if err != nil {
		return "", err
	}
	defer catcher.Close()
	script, err := ctx.Compile(scope, source, catcher)
	if err != nil {
		return "", exceptionError(catcher, scope, ctx, name, err)
	}
	defer script.Close()
	result, err := script.Run(scope, catcher)
	if err != nil {
		return "", exceptionError(catcher, scope, ctx, name, err)
	}
	text, err := result.ToString(ctx)
	if err != nil {
		return "", err
	}
	return text, nil
}

func exceptionError(catcher *gov8.TryCatch, scope *gov8.Scope, ctx *gov8.Context, name string, cause error) error {
	text, err := catcher.ExceptionText(scope, ctx)
	if err == nil && text != "" {
		return fmt.Errorf("%s: %s", name, text)
	}
	return fmt.Errorf("%s: %w", name, cause)
}

func primitive(scope *gov8.Scope, value any) (gov8.Value, error) {
	switch value := value.(type) {
	case nil:
		return scope.Null()
	case string:
		return scope.NewString(value)
	case bool:
		return scope.Boolean(value)
	case int:
		return scope.Int32(int32(value))
	case int32:
		return scope.Int32(value)
	case uint32:
		return scope.Uint32(value)
	case float64:
		return scope.Number(value)
	default:
		return gov8.Value{}, fmt.Errorf("unsupported V8 spike host value %T", value)
	}
}

func setCallbackResult(cs *gov8.CallbackScope, rv gov8.ReturnValue, value any) {
	switch value := value.(type) {
	case nil:
		_ = rv.SetUndefined()
	case string:
		if value == "" {
			_ = rv.SetEmptyString()
			return
		}
		result, err := cs.NewString(value)
		if err == nil {
			_ = rv.Set(result)
		}
	case bool:
		_ = rv.SetBool(value)
	case int:
		_ = rv.SetInt32(int32(value))
	case int32:
		_ = rv.SetInt32(value)
	case uint32:
		_ = rv.SetUint32(value)
	case float64:
		_ = rv.SetFloat64(value)
	default:
		exception, _ := cs.NewError(fmt.Sprintf("unsupported host return %T", value))
		_ = cs.ThrowException(exception)
	}
}
