//go:build windows && amd64

package v8

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
	"time"

	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

// Factory exposes V8 through the same engine-neutral contract as QuickJS.
// Browser code never imports this package directly.
type Factory struct{}

func (Factory) New() engine.Runtime {
	profile := newDiagnostics()
	var started time.Time
	if profile != nil {
		started = time.Now()
	}
	owner, err := NewRuntime()
	if err != nil {
		panic(fmt.Sprintf("initialize pinned V8 backend: %v", err))
	}
	if profile != nil {
		profile.Costs["factory:isolate"] = diagnosticCost{Count: 1, Nanoseconds: time.Since(started).Nanoseconds()}
	}
	backend, err := newAdapter(owner, profile)
	if err != nil {
		panic(err)
	}
	return backend
}

// Both ordinary and restored isolates install the same adapter-owned hooks.
func newAdapter(owner *Runtime, profile *diagnosticState) (*adapter, error) {
	started := time.Now()
	realm, err := owner.NewRealm()
	if err != nil {
		_ = owner.Dispose()
		return nil, fmt.Errorf("create V8 realm: %w", err)
	}
	backend := &adapter{owner: owner, realm: realm, moduleCache: map[string]*gov8.Module{}, moduleNames: map[*gov8.Module]string{}, profile: profile}
	if os.Getenv("MIMIC_PROFILE_PROCESSORS") == "1" {
		backend.processorSamples = map[uintptr]uint64{}
	}
	if profile != nil {
		backend.recordCost("factory:context", started)
	}
	factory, err := backend.Eval(context.Background(), `(()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return[promise,resolve,reject]})`, "mimic-promise-factory.js")
	if err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("create V8 promise factory: %w", err)
	}
	backend.promiseFactory = factory
	return backend, nil
}

type hostFunction struct {
	function  engine.Function
	name      string
	transient bool
	packed    string
}

type callbackContext struct {
	scope  *gov8.CallbackScope
	ctx    *gov8.Context
	result gov8.ReturnValue
	id     uint64
}

type adapter struct {
	profile           *diagnosticState
	owner             *Runtime
	realm             *Realm
	now               func() time.Time
	observer          func(string, bool)
	closed            bool
	mu                sync.Mutex
	callback          *callbackContext  // actor-thread only; guarded from foreign readers by actor TID
	activeIsolate     *gov8.Isolate     // actor-thread only
	nestedTermination bool              // clear only after the outer actor turn unwinds
	activeContext     context.Context   // actor-thread only; inherited by cross-realm calls
	runDepth          int               // actor-thread only; includes cooperatively serviced calls
	transientFrames   []*transientFrame // owner-thread only; bounded scratch storage
	packedStore       *gov8.BackingStore
	packedMemory      *[packedBytes]byte
	packedBuffer      *gov8.Global
	packedFactories   map[string]*gov8.Global
	packedFrames      []*packedFrame
	callbackSeq       uint64
	promiseFactory    engine.Value
	globals           []*gov8.Global // retained engine.Values; released on the isolate thread
	modules           []*gov8.Module
	moduleCache       map[string]*gov8.Module
	moduleNames       map[*gov8.Module]string
	processorSamples  map[uintptr]uint64 // opt-in diagnostic sampling, actor-thread only
	nativePending     bool               // actor-thread only; foreground/background V8 tasks
}

// Transient arguments cannot escape the synchronous host call. A frame stays
// checked out until the return value is marshalled, including nested JS calls.
// Clearing it prevents closed scopes and host objects from becoming roots.
type transientFrame struct {
	values [9]runtimeValue
	args   [8]engine.Value
}

type runtimeValue struct {
	runtime    *adapter
	global     *gov8.Global
	local      gov8.Value
	borrowed   bool
	host       any
	hostSet    bool
	callbackID uint64
}

func (v *runtimeValue) Export() any {
	if v == nil || v.runtime == nil {
		return nil
	}
	return v.runtime.export(v)
}

func (v *runtimeValue) String() string {
	if v == nil || v.runtime == nil {
		return "undefined"
	}
	return v.runtime.string(v)
}

func (a *adapter) onCallback() *callbackContext {
	if currentThreadID() != a.owner.actorTID {
		return nil
	}
	return a.callback
}

func (a *adapter) RunNested(ctx context.Context, operation func(context.Context) error) error {
	if a.onCallback() == nil {
		return operation(ctx)
	}
	nestedContext, cancel := context.WithCancel(ctx)
	defer cancel()
	if a.activeContext != nil {
		stop := context.AfterFunc(a.activeContext, cancel)
		defer stop()
		if a.activeContext.Err() != nil {
			cancel()
		}
	}
	if err := nestedContext.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- operation(nestedContext) }()
	// The caller's JavaScript stack remains suspended on its original thread.
	// Only this actor's incoming operations can run here: no Page timer or
	// other actor is driven. Synchronous child -> parent callbacks therefore
	// preserve both stack ordering and isolate affinity, including deeper chains.
	for {
		select {
		case err := <-done:
			return err
		case command := <-a.owner.commands:
			if command.outerOnly {
				a.owner.deferred = append(a.owner.deferred, command)
				continue
			}
			command.run(a.owner.actorState)
		}
	}
}

func (a *adapter) RunOnOwner(ctx context.Context, operation func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := a.owner.execute(func(*state) response {
		if err := ctx.Err(); err != nil {
			return response{err: err}
		}
		return response{err: operation(ctx)}
	})
	return err
}

func (a *adapter) Eval(ctx context.Context, source, name string) (engine.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callback := a.onCallback(); callback != nil {
		// A host callback already owns this actor and isolate. Dispatching to
		// the actor again waits for ourselves; evaluate in the current scope.
		isolate := callback.scope.Isolate()
		var finished, watcherDone chan struct{}
		if ctx.Done() != nil {
			finished, watcherDone = make(chan struct{}), make(chan struct{})
			handle := isolate.ThreadSafeHandle()
			go func() {
				defer close(watcherDone)
				select {
				case <-ctx.Done():
					handle.TerminateExecution()
				case <-finished:
				}
			}()
		}
		value, err := a.evalScoped(isolate, callback.ctx, callback.scope.Scope(), source, name)
		if finished != nil {
			close(finished)
			<-watcherDone
		}
		if ctx.Err() != nil {
			// Clearing termination here would let JavaScript continue inside
			// the canceled outer invocation. The actor boundary does cleanup.
			a.nestedTermination = true
			return nil, ctx.Err()
		}
		return value, err
	}
	return a.runContext(ctx, func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		return a.evalScoped(s.isolate, realm, scope, source, name)
	})
}

func (a *adapter) evalScoped(isolate *gov8.Isolate, realm *gov8.Context, scope *gov8.Scope, source, name string) (engine.Value, error) {
	return a.evalScopedCode(isolate, realm, scope, source, name, false)
}

func (a *adapter) EvalBootstrap(ctx context.Context, source, name string) (engine.Value, error) {
	return a.runContext(ctx, func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		return a.evalScopedCode(s.isolate, realm, scope, source, name, true)
	})
}

func (a *adapter) evalScopedCode(isolate *gov8.Isolate, realm *gov8.Context, scope *gov8.Scope, source, name string, reusable bool) (engine.Value, error) {
	if a.profile != nil && os.Getenv("MIMIC_V8_CPU_PROFILE") == "1" && (name == "mimic:webapi-surface" || name == "__pyppeteer_evaluation_script__") {
		finish, err := startNativeProfile(isolate, realm)
		if err != nil {
			return nil, err
		}
		defer func() {
			data, err := finish()
			if err == nil {
				if a.profile.CPUProfiles == nil {
					a.profile.CPUProfiles = map[string]json.RawMessage{}
				}
				a.profile.CPUProfiles[name] = data
			}
		}()
	}
	catcher, err := isolate.NewTryCatch()
	if err != nil {
		return nil, err
	}
	defer catcher.Close()
	var started time.Time
	if a.profile != nil {
		started = time.Now()
	}
	resourceName, err := scope.NewString(name)
	if err != nil {
		return nil, err
	}
	var script *gov8.Script
	var bootstrap *gov8.Function
	var key bootstrapKey
	var cached bool
	if reusable {
		key = bootstrapKeyFor(source, name)
		data := bootstrapCode.get(key)
		var rejected bool
		bootstrap, rejected, err = realm.CompileFunctionAdvanced(scope, source+"\n//# sourceURL="+name, nil, data, catcher)
		cached = data != nil && !rejected
	} else {
		script, err = realm.CompileScriptCompilerSource(scope,
			gov8.NewScriptCompilerSource(source, &gov8.ScriptCompilerOrigin{ResourceName: resourceName, ScriptID: -1}),
			gov8.OptNoCompileOptions, gov8.NoCacheNoReason, catcher)
	}
	if a.profile != nil {
		a.recordCost("compile:"+name, started)
	}
	if err != nil {
		return nil, a.evalError(catcher, scope, realm, name, err)
	}
	if script != nil {
		defer script.Close()
	}
	if a.profile != nil {
		started = time.Now()
	}
	var result gov8.Value
	if bootstrap != nil {
		var ok bool
		global, globalErr := realm.GlobalObject(scope)
		if globalErr != nil {
			return nil, globalErr
		}
		result, ok, err = bootstrap.Call(scope, global.Value)
		if err == nil && !ok {
			err = errors.New("bootstrap execution failed")
		}
	} else {
		result, err = script.Run(scope, catcher)
	}
	if a.profile != nil {
		a.recordCost("execute:"+name, started)
		heap, _ := isolate.GetHeapStatistics()
		a.profile.Heaps[name] = heap
	}
	if err != nil {
		return nil, a.evalError(catcher, scope, realm, name, err)
	}
	// Produce after execution, so functions reached during bootstrap are
	// included rather than lazily compiled again in every new realm.
	if reusable && !cached {
		if data, cacheErr := bootstrap.CreateCodeCache(); cacheErr == nil {
			bootstrapCode.put(key, data)
		}
	}
	return a.persist(scope, result)
}

func (a *adapter) evalError(catcher *gov8.TryCatch, scope *gov8.Scope, realm *gov8.Context, name string, cause error) error {
	err := exceptionError(catcher, scope, realm, name, cause)
	if a.onCallback() != nil {
		if exception, ok, readErr := catcher.Exception(scope); readErr == nil && ok {
			if value, persistErr := a.persist(scope, exception); persistErr == nil {
				return &callException{error: err, value: value}
			}
		}
	}
	return err
}

func (a *adapter) EvalModule(ctx context.Context, source, name string, loader engine.ModuleLoader) (engine.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if loader == nil {
		return nil, errors.New("module loader is nil")
	}
	return a.runContext(ctx, func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		catcher, err := s.isolate.NewTryCatch()
		if err != nil {
			return nil, err
		}
		defer catcher.Close()
		compile := func(moduleSource, resourceName string, tc *gov8.TryCatch) (*gov8.Module, error) {
			if cached := a.moduleCache[resourceName]; cached != nil {
				return cached, nil
			}
			module, compileErr := realm.CompileModule(scope, moduleSource, resourceName, tc)
			if compileErr != nil {
				return nil, compileErr
			}
			a.modules = append(a.modules, module)
			a.moduleNames[module] = resourceName
			a.moduleCache[resourceName] = module
			return module, nil
		}
		// V8 owns the stable, null-prototype import.meta object; the embedder
		// supplies the module's resource URL, including its query and fragment.
		// Keep this tied to module identity, not the currently executing entry:
		// dependencies and later dynamic imports have their own base URLs.
		if err := s.isolate.SetHostInitializeImportMetaObjectCallback(func(cs *gov8.CallbackScope, module *gov8.Module, meta *gov8.Object) error {
			resourceName, ok := a.moduleNames[module]
			if !ok {
				return errors.New("import.meta module has no resource name")
			}
			value, err := cs.NewString(resourceName)
			if err != nil {
				return err
			}
			_, err = cs.ObjectSet(meta.Value, "url", value)
			return err
		}); err != nil {
			return nil, err
		}
		entry, err := compile(source, name, catcher)
		if err != nil {
			return nil, exceptionError(catcher, scope, realm, name, err)
		}
		linked, err := entry.Instantiate(scope, func(request gov8.ModuleResolveRequest) (*gov8.Module, error) {
			referrer := a.moduleNames[request.Referrer]
			dependencySource, resourceName, loadErr := loader(request.Specifier, referrer)
			if loadErr != nil {
				return nil, loadErr
			}
			return compile(dependencySource, resourceName, nil)
		}, catcher)
		if err != nil || !linked {
			if err == nil {
				err = errors.New("V8 rejected module graph instantiation")
			}
			return nil, exceptionError(catcher, scope, realm, name, err)
		}
		if err := s.isolate.SetHostImportModuleDynamicallyCallback(func(request gov8.DynamicImportRequest) (gov8.Promise, error) {
			referrer, err := request.Scope.ToString(request.ResourceName)
			if err != nil {
				return gov8.Promise{}, err
			}
			specifier, err := request.Scope.ToString(request.Specifier)
			if err != nil {
				return gov8.Promise{}, err
			}
			dependencySource, resourceName, err := loader(specifier, referrer)
			if err != nil {
				return gov8.Promise{}, err
			}
			if module := a.moduleCache[resourceName]; module != nil {
				namespace, namespaceErr := module.Namespace(request.Scope.Scope())
				if namespaceErr != nil {
					return gov8.Promise{}, namespaceErr
				}
				resolver, promise, resolverErr := request.Scope.NewCallbackPromiseResolver()
				if resolverErr != nil {
					return gov8.Promise{}, resolverErr
				}
				if _, settleErr := request.Scope.SettleCallbackPromise(resolver, namespace, false); settleErr != nil {
					return gov8.Promise{}, settleErr
				}
				return promise, nil
			}
			var compileDynamic func(string, string) (*gov8.Module, error)
			compileDynamic = func(moduleSource, moduleName string) (*gov8.Module, error) {
				if cached := a.moduleCache[moduleName]; cached != nil {
					return cached, nil
				}
				module, compileErr := realm.CompileModule(request.Scope.Scope(), moduleSource, moduleName, nil)
				if compileErr != nil {
					return nil, compileErr
				}
				a.modules = append(a.modules, module)
				a.moduleNames[module] = moduleName
				a.moduleCache[moduleName] = module
				return module, nil
			}
			module, err := compileDynamic(dependencySource, resourceName)
			if err != nil {
				return gov8.Promise{}, err
			}
			linked, err := module.Instantiate(request.Scope.Scope(), func(importRequest gov8.ModuleResolveRequest) (*gov8.Module, error) {
				childSource, childName, loadErr := loader(importRequest.Specifier, a.moduleNames[importRequest.Referrer])
				if loadErr != nil {
					return nil, loadErr
				}
				return compileDynamic(childSource, childName)
			}, nil)
			if err != nil || !linked {
				if err == nil {
					err = errors.New("V8 rejected dynamic module graph instantiation")
				}
				return gov8.Promise{}, err
			}
			if _, err := module.Evaluate(request.Scope.Scope(), nil); err != nil {
				return gov8.Promise{}, err
			}
			namespace, err := module.Namespace(request.Scope.Scope())
			if err != nil {
				return gov8.Promise{}, err
			}
			resolver, promise, err := request.Scope.NewCallbackPromiseResolver()
			if err != nil {
				return gov8.Promise{}, err
			}
			if _, err := request.Scope.SettleCallbackPromise(resolver, namespace, false); err != nil {
				return gov8.Promise{}, err
			}
			return promise, nil
		}); err != nil {
			return nil, err
		}
		promise, err := entry.Evaluate(scope, catcher)
		if err != nil {
			return nil, exceptionError(catcher, scope, realm, name, err)
		}
		return a.persist(scope, promise.Value)
	})
}

func (a *adapter) Set(name string, value any) error {
	_, err := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.marshal(scope, realm, value)
		if err != nil {
			return nil, err
		}
		global, err := realm.GlobalObject(scope)
		if err != nil {
			return nil, err
		}
		ok, err := global.SetByName(scope, realm, name, local)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("V8 rejected global property %q", name)
		}
		return nil, nil
	})
	return err
}

func (a *adapter) Get(name string) engine.Value {
	if callback := a.onCallback(); callback != nil {
		scope := callback.scope.Scope()
		global, err := callback.ctx.GlobalObject(scope)
		if err != nil {
			return nil
		}
		local, ok, err := global.GetByName(scope, callback.ctx, name)
		if err != nil || !ok {
			return nil
		}
		value, _ := a.persist(scope, local)
		return value
	}
	value, _ := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		global, err := realm.GlobalObject(scope)
		if err != nil {
			return nil, err
		}
		local, ok, err := global.GetByName(scope, realm, name)
		if err != nil || !ok {
			return nil, err
		}
		return a.persist(scope, local)
	})
	return value
}

func (a *adapter) Value(value any) engine.Value {
	if callback := a.onCallback(); callback != nil {
		return &runtimeValue{runtime: a, borrowed: true, host: value, hostSet: true}
	}
	result, _ := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.marshal(scope, realm, value)
		if err != nil {
			return nil, err
		}
		return a.persist(scope, local)
	})
	return result
}

func (a *adapter) GetProperty(object engine.Value, name string) engine.Value {
	if callback := a.onCallback(); callback != nil {
		local, err := a.localCallback(object)
		if err != nil {
			return nil
		}
		result, ok, err := callback.scope.ObjectGet(local, name)
		if err != nil || !ok {
			return nil
		}
		global, persistErr := a.newGlobal(callback.scope.Scope(), result)
		if persistErr != nil {
			return nil
		}
		return &runtimeValue{runtime: a, global: global, local: result, borrowed: true, callbackID: callback.id}
	}
	result, _ := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.local(scope, object)
		if err != nil {
			return nil, err
		}
		obj, err := gov8.AsObject(local)
		if err != nil {
			return nil, err
		}
		property, ok, err := obj.GetByName(scope, realm, name)
		if err != nil || !ok {
			return nil, err
		}
		return a.persist(scope, property)
	})
	return result
}

func (a *adapter) SetProperty(object engine.Value, name string, value any) error {
	if callback := a.onCallback(); callback != nil {
		objectLocal, err := a.localCallback(object)
		if err != nil {
			return err
		}
		valueLocal, err := a.marshalCallback(callback.scope, value)
		if err != nil {
			return err
		}
		ok, err := callback.scope.ObjectSet(objectLocal, name, valueLocal)
		if err == nil && !ok {
			err = fmt.Errorf("V8 rejected property %q", name)
		}
		return err
	}
	_, err := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		objectLocal, err := a.local(scope, object)
		if err != nil {
			return nil, err
		}
		obj, err := gov8.AsObject(objectLocal)
		if err != nil {
			return nil, err
		}
		valueLocal, err := a.marshal(scope, realm, value)
		if err != nil {
			return nil, err
		}
		ok, err := obj.SetByName(scope, realm, name, valueLocal)
		if err == nil && !ok {
			err = fmt.Errorf("V8 rejected property %q", name)
		}
		return nil, err
	})
	return err
}

func (a *adapter) TypeOf(value engine.Value) string {
	operation := func(scope *gov8.Scope, local gov8.Value) string {
		// IsFunction classifies callable HTMLDDA objects as functions even though
		// JavaScript typeof must return undefined. Ask V8 for the actual operator.
		kind, err := local.TypeOf(scope)
		if err != nil {
			return "undefined"
		}
		text, err := kind.StringValue()
		if err != nil {
			return "undefined"
		}
		return text
	}
	if callback := a.onCallback(); callback != nil {
		if v, ok := value.(*runtimeValue); ok && v.hostSet {
			return goTypeOf(v.host)
		}
		local, err := a.localCallback(value)
		if err != nil {
			return "undefined"
		}
		return operation(callback.scope.Scope(), local)
	}
	var result = "undefined"
	_, _ = a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.local(scope, value)
		if err == nil {
			result = operation(scope, local)
		}
		return nil, err
	})
	return result
}

func (a *adapter) StrictEqual(left, right engine.Value) bool {
	if a.onCallback() != nil {
		l, le := a.localCallback(left)
		r, re := a.localCallback(right)
		if le != nil || re != nil {
			return false
		}
		equal, _ := l.StrictEquals(r)
		return equal
	}
	var equal bool
	_, _ = a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		l, err := a.local(scope, left)
		if err != nil {
			return nil, err
		}
		r, err := a.local(scope, right)
		if err != nil {
			return nil, err
		}
		equal, err = l.StrictEquals(r)
		return nil, err
	})
	return equal
}

type callException struct {
	error
	value engine.Value
}

func (a *adapter) callError(catcher *gov8.TryCatch, scope *gov8.Scope, realm *gov8.Context) error {
	err := exceptionError(catcher, scope, realm, "JavaScript callback", errors.New("call threw"))
	if exception, ok, readErr := catcher.Exception(scope); readErr == nil && ok {
		if value, persistErr := a.persist(scope, exception); persistErr == nil {
			return &callException{error: err, value: value}
		}
	}
	return err
}

func (a *adapter) Call(ctx context.Context, function, this engine.Value, args ...engine.Value) (engine.Value, error) {
	if callback := a.onCallback(); callback != nil {
		catcher, err := callback.scope.Isolate().NewTryCatch()
		if err != nil {
			return nil, err
		}
		defer catcher.Close()
		fn, err := a.localCallback(function)
		if err != nil {
			return nil, err
		}
		receiver, err := a.localCallbackOrUndefined(callback.scope, this)
		if err != nil {
			return nil, err
		}
		argv := make([]gov8.Value, len(args))
		for i := range args {
			argv[i], err = a.localCallback(args[i])
			if err != nil {
				return nil, err
			}
		}
		result, ok, err := callback.scope.CallFunction(fn, receiver, argv)
		if caught, _ := catcher.HasCaught(); caught {
			return nil, a.callError(catcher, callback.scope.Scope(), callback.ctx)
		}
		if err != nil || !ok {
			if err == nil {
				err = errors.New("JavaScript callback failed")
			}
			return nil, err
		}
		global, persistErr := a.newGlobal(callback.scope.Scope(), result)
		if persistErr != nil {
			return nil, persistErr
		}
		return &runtimeValue{runtime: a, global: global, local: result, borrowed: true, callbackID: callback.id}, nil
	}
	return a.runContext(ctx, func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		catcher, err := s.isolate.NewTryCatch()
		if err != nil {
			return nil, err
		}
		defer catcher.Close()
		fnValue, err := a.local(scope, function)
		if err != nil {
			return nil, err
		}
		fn, ok, err := gov8.AsFunction(fnValue, realm)
		if err != nil || !ok {
			return nil, fmt.Errorf("value is not callable")
		}
		receiver, err := a.localOrUndefined(scope, this)
		if err != nil {
			return nil, err
		}
		argv := make([]gov8.Value, len(args))
		for i := range args {
			argv[i], err = a.local(scope, args[i])
			if err != nil {
				return nil, err
			}
		}
		result, ok, err := fn.Call(scope, receiver, argv...)
		if caught, _ := catcher.HasCaught(); caught {
			return nil, a.callError(catcher, scope, realm)
		}
		if err != nil || !ok {
			if err == nil {
				err = errors.New("JavaScript callback failed")
			}
			return nil, err
		}
		return a.persist(scope, result)
	})
}

func (a *adapter) Function(function engine.Function) any { return hostFunction{function: function} }

// TransientFunction borrows callback arguments for synchronous host operations.
// The function must not retain this/args in browser state or scheduled tasks.
// Ordinary Function preserves the existing persistent-value contract.
func (a *adapter) TransientFunction(function engine.Function) any {
	return hostFunction{function: function, transient: true}
}

// PackedFunction is for synchronous primitive-argument hosts which ignore this.
// It does not retain arguments or defer mutations. Other runtimes can use the
// ordinary Function contract; the packing is private to this adapter.
func (a *adapter) PackedFunction(function engine.Function, signature string) any {
	return hostFunction{function: function, transient: true, packed: signature}
}

func (a *adapter) NewPromise() engine.Promise {
	if a.promiseFactory == nil {
		err := errors.New("V8 promise factory is unavailable")
		return engine.Promise{Resolve: func(any) error { return err }, Reject: func(any) error { return err }}
	}
	value, err := a.Call(context.Background(), a.promiseFactory, nil)
	if err != nil {
		return engine.Promise{Resolve: func(any) error { return err }, Reject: func(any) error { return err }}
	}
	promise := a.GetProperty(value, "0")
	resolve := a.GetProperty(value, "1")
	reject := a.GetProperty(value, "2")
	settle := func(function engine.Value, value any) error {
		argument := a.Value(value)
		_, err := a.Call(context.Background(), function, nil, argument)
		return err
	}
	return engine.Promise{Value: promise, Resolve: func(v any) error { return settle(resolve, v) }, Reject: func(v any) error { return settle(reject, v) }}
}

func (a *adapter) Await(value engine.Value) (engine.Value, bool, error) {
	var result engine.Value
	var settled bool
	_, err := a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.local(scope, value)
		if err != nil {
			return nil, err
		}
		isPromise, err := local.IsPromise()
		if err != nil {
			return nil, err
		}
		if !isPromise {
			result, settled = value, true
			return nil, nil
		}
		promise, err := gov8.AsPromise(local)
		if err != nil {
			return nil, err
		}
		state, err := promise.State()
		if err != nil {
			return nil, err
		}
		if state == gov8.PromisePending {
			return nil, nil
		}
		settled = true
		localResult, err := promise.Result(scope)
		if err != nil {
			return nil, err
		}
		if state == gov8.PromiseRejected {
			return nil, fmt.Errorf("promise rejected: %s", localResultString(localResult, realm))
		}
		result, err = a.persist(scope, localResult)
		return nil, err
	})
	return result, settled, err
}

func (a *adapter) SetTimeSource(now func() time.Time) {
	a.now = now
	if now == nil {
		return
	}
	_ = a.Set("__mimicDateNow", a.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		// ECMAScript time values are integral milliseconds. performance.now()
		// remains the independent high-resolution monotonic clock.
		return a.Value(a.now().UnixMilli()), nil
	}))
	_, _ = a.Eval(context.Background(), `(()=>{const mimicDateNow=globalThis.__mimicDateNow;delete globalThis.__mimicDateNow;const NativeDate=Date;const MimicDate=new Proxy(NativeDate,{apply(target,thisArg,args){return args.length?Reflect.apply(target,thisArg,args):new NativeDate(mimicDateNow()).toString()},construct(target,args,newTarget){return Reflect.construct(target,args.length?args:[mimicDateNow()],newTarget)}});Object.defineProperty(MimicDate,'now',{value:()=>mimicDateNow(),writable:true,configurable:true});Object.defineProperty(MimicDate.prototype,'constructor',{value:MimicDate,writable:true,configurable:true});globalThis.Date=MimicDate})()`, "mimic-clock.js")
}

func (a *adapter) MicrotaskCheckpoint() error {
	return a.MicrotaskCheckpointContext(context.Background())
}

func (a *adapter) MicrotaskCheckpointContext(ctx context.Context) error {
	if a.onCallback() != nil {
		// Nested script cleanup is still inside the outer JavaScript stack.
		// Its jobs run at the outer task boundary, not inside the host call.
		return ctx.Err()
	}
	_, err := a.runContext(ctx, func(s *state, _ *gov8.Context, _ *gov8.Scope) (engine.Value, error) {
		// Native asynchronous compilation (notably WebAssembly) posts foreground
		// tasks outside the Promise microtask queue. Pump them without blocking,
		// on the isolate owner, before completing the browser checkpoint.
		pending, err := s.isolate.HasPendingBackgroundTasks()
		if err != nil {
			return nil, err
		}
		a.nativePending = pending
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			ran, err := s.isolate.PumpMessageLoop(false)
			if err != nil {
				return nil, err
			}
			if !ran {
				break
			}
			a.nativePending = true
			if err := s.isolate.PerformMicrotaskCheckpoint(); err != nil {
				return nil, err
			}
		}
		if err := s.isolate.PerformMicrotaskCheckpoint(); err != nil {
			return nil, err
		}
		// A Promise reaction can itself start native asynchronous work.
		pending, err = s.isolate.HasPendingBackgroundTasks()
		a.nativePending = a.nativePending || pending
		return nil, err
	})
	return err
}

// NativeTasksPending requests another browser task boundary while native work
// is outstanding. No background goroutine may enter the JavaScript isolate.
func (a *adapter) NativeTasksPending() bool {
	if a.onCallback() != nil {
		return a.nativePending
	}
	var pending bool
	_, _ = a.run(func(_ *state, _ *gov8.Context, _ *gov8.Scope) (engine.Value, error) {
		pending = a.nativePending
		return nil, nil
	})
	return pending
}

func (a *adapter) SetGlobalAccessObserver(observer func(name string, supported bool)) {
	a.observer = observer
}

func (a *adapter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()
	_, _ = a.runCommand(func(_ *state, _ *gov8.Context, _ *gov8.Scope) (engine.Value, error) {
		for i := len(a.modules) - 1; i >= 0; i-- {
			_ = a.modules[i].Close()
		}
		for _, global := range a.globals {
			_ = global.Close()
		}
		a.globals = nil
		a.modules = nil
		a.moduleCache = nil
		a.moduleNames = nil
		if a.packedStore != nil {
			_ = a.packedStore.Close()
			a.packedStore = nil
		}
		a.packedBuffer = nil
		a.packedMemory = nil
		a.packedFactories = nil
		a.packedFrames = nil
		return nil, nil
	}, true)
	a.mu.Lock()
	a.closed = true
	a.mu.Unlock()
	return a.owner.Dispose()
}

type realmOperation func(*state, *gov8.Context, *gov8.Scope) (engine.Value, error)

func (a *adapter) run(operation realmOperation) (engine.Value, error) {
	return a.runCommand(operation, false)
}

func (a *adapter) runCommand(operation realmOperation, outerOnly bool) (engine.Value, error) {
	value, err := a.owner.executeCommand(func(s *state) response {
		realm := s.realms[a.realm.id]
		if realm == nil {
			return response{err: errors.New("V8 realm is closed")}
		}
		scope, err := s.isolate.NewScope()
		if err != nil {
			return response{err: err}
		}
		defer scope.Close()
		previousIsolate := a.activeIsolate
		a.activeIsolate = s.isolate
		a.runDepth++
		result, err := operation(s, realm, scope)
		a.runDepth--
		if a.nestedTermination && a.runDepth == 0 {
			_ = s.isolate.CancelTerminateExecution()
			a.nestedTermination = false
		}
		a.activeIsolate = previousIsolate
		return response{value: result, err: err}
	}, outerOnly)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return value.(engine.Value), nil
}

func (a *adapter) runContext(ctx context.Context, operation realmOperation) (engine.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		previousContext := a.activeContext
		a.activeContext = ctx
		defer func() { a.activeContext = previousContext }()
		if ctx.Done() == nil {
			return operation(s, realm, scope)
		}
		// Join the watcher on the isolate thread before another operation or
		// disposal can begin. Failed dispatch creates no watcher at all.
		finished, watcherDone := make(chan struct{}), make(chan struct{})
		handle := s.isolate.ThreadSafeHandle()
		go func() {
			defer close(watcherDone)
			select {
			case <-ctx.Done():
				handle.TerminateExecution()
			case <-finished:
			}
		}()
		result, err := operation(s, realm, scope)
		close(finished)
		<-watcherDone
		if ctx.Err() != nil {
			if a.runDepth == 1 {
				_ = s.isolate.CancelTerminateExecution()
			} else {
				a.nestedTermination = true
			}
			return nil, ctx.Err()
		}
		return result, err
	})
}

func (a *adapter) newGlobal(scope *gov8.Scope, local gov8.Value) (*gov8.Global, error) {
	global, err := gov8.NewGlobal(scope, local)
	if err == nil {
		a.globals = append(a.globals, global)
	}
	return global, err
}

func (a *adapter) persist(scope *gov8.Scope, local gov8.Value) (engine.Value, error) {
	global, err := a.newGlobal(scope, local)
	if err != nil {
		return nil, err
	}
	return &runtimeValue{runtime: a, global: global}, nil
}

func (a *adapter) local(scope *gov8.Scope, value engine.Value) (gov8.Value, error) {
	v, ok := value.(*runtimeValue)
	if !ok || v == nil || v.runtime != a || v.global == nil {
		return gov8.Value{}, errors.New("value belongs to another JavaScript engine")
	}
	return v.global.ToLocal(scope)
}

func (a *adapter) localOrUndefined(scope *gov8.Scope, value engine.Value) (gov8.Value, error) {
	if value == nil {
		return scope.Undefined()
	}
	return a.local(scope, value)
}

func (a *adapter) localCallback(value engine.Value) (gov8.Value, error) {
	v, ok := value.(*runtimeValue)
	if !ok || v == nil || v.runtime != a {
		return gov8.Value{}, errors.New("callback value is not local to this V8 invocation")
	}
	if v.hostSet {
		callback := a.onCallback()
		if callback == nil {
			return gov8.Value{}, errors.New("host value escaped its V8 callback")
		}
		return callbackValue(callback.scope, callback.ctx, callback.result, v.host)
	}
	callback := a.onCallback()
	if callback == nil {
		return gov8.Value{}, errors.New("not in V8 callback")
	}
	if v.borrowed && v.callbackID == callback.id {
		return v.local, nil
	}
	if v.global != nil {
		return v.global.ToLocal(callback.scope.Scope())
	}
	return gov8.Value{}, errors.New("borrowed V8 value escaped without a persistent handle")
}

func (a *adapter) localCallbackOrUndefined(scope *gov8.CallbackScope, value engine.Value) (gov8.Value, error) {
	if value == nil {
		return a.marshalCallback(scope, nil)
	}
	return a.localCallback(value)
}

func (a *adapter) marshal(scope *gov8.Scope, realm *gov8.Context, value any) (gov8.Value, error) {
	if engineValue, ok := value.(engine.Value); ok {
		return a.local(scope, engineValue)
	}
	if function, ok := value.(hostFunction); ok {
		if function.packed != "" {
			return a.makePackedFunction(scope, realm, function)
		}
		return a.makeFunction(scope, realm, function.function, function.name, function.transient)
	}
	if primitiveValue, ok, err := marshalPrimitive(scope, value); ok || err != nil {
		return primitiveValue, err
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			break
		}
		object, err := scope.NewObject(realm)
		if err != nil {
			return gov8.Value{}, err
		}
		iter := rv.MapRange()
		for iter.Next() {
			value := iter.Value().Interface()
			if function, ok := value.(hostFunction); ok {
				function.name = iter.Key().String()
				value = function
			}
			member, err := a.marshal(scope, realm, value)
			if err != nil {
				return gov8.Value{}, err
			}
			ok, err := object.SetByName(scope, realm, iter.Key().String(), member)
			if err != nil || !ok {
				return gov8.Value{}, err
			}
		}
		return object.Value, nil
	case reflect.Slice, reflect.Array:
		elements := make([]gov8.Value, rv.Len())
		for i := range elements {
			var err error
			elements[i], err = a.marshal(scope, realm, rv.Index(i).Interface())
			if err != nil {
				return gov8.Value{}, err
			}
		}
		array, err := scope.NewArrayWithElements(realm, elements)
		if err != nil {
			return gov8.Value{}, err
		}
		return array.Value, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return gov8.Value{}, err
	}
	quoted, _ := json.Marshal(string(encoded))
	script, err := realm.Compile(scope, "JSON.parse("+string(quoted)+")", nil)
	if err != nil {
		return gov8.Value{}, err
	}
	defer script.Close()
	return script.Run(scope, nil)
}

func (a *adapter) makeFunction(scope *gov8.Scope, realm *gov8.Context, function engine.Function, name string, transient bool) (gov8.Value, error) {
	if a.activeIsolate == nil {
		return gov8.Value{}, errors.New("V8 isolate is not active")
	}
	fn, err := a.activeIsolate.NewFunction(scope, realm, func(cs *gov8.CallbackScope, args gov8.FunctionCallbackArguments, rv gov8.ReturnValue) {
		if a.profile != nil {
			started := time.Now()
			defer a.recordCost("host:"+name, started)
		}
		var argumentStart time.Time
		if a.profile != nil && a.profile.Detailed {
			argumentStart = time.Now()
		}
		previous := a.callback
		a.callbackSeq++
		if a.processorSamples != nil && a.callbackSeq%1024 == 0 {
			processor, _, _ := diagnosticProcessor.Call()
			a.processorSamples[processor]++
		}
		callbackID := a.callbackSeq
		a.callback = &callbackContext{scope: cs, ctx: realm, result: rv, id: callbackID}
		defer func() { a.callback = previous }()
		var scratch *transientFrame
		var wrapped []engine.Value
		if transient && args.Length() <= 8 {
			n := len(a.transientFrames)
			if n > 0 {
				scratch = a.transientFrames[n-1]
				a.transientFrames[n-1] = nil
				a.transientFrames = a.transientFrames[:n-1]
			} else {
				scratch = new(transientFrame)
			}
			wrapped = scratch.args[:args.Length()]
			defer func() {
				*scratch = transientFrame{}
				if len(a.transientFrames) < 8 {
					a.transientFrames = append(a.transientFrames, scratch)
				}
			}()
		} else {
			wrapped = make([]engine.Value, args.Length())
		}
		for i := range wrapped {
			local, e := args.Get(i)
			if e != nil {
				return
			}
			var global *gov8.Global
			if !transient {
				global, e = a.newGlobal(cs.Scope(), local)
				if e != nil {
					return
				}
			}
			value := runtimeValue{runtime: a, global: global, local: local, borrowed: true, callbackID: callbackID}
			if scratch != nil {
				scratch.values[i] = value
				wrapped[i] = &scratch.values[i]
			} else {
				wrapped[i] = &runtimeValue{runtime: a, global: global, local: local, borrowed: true, callbackID: callbackID}
			}
		}
		thisObject, e := args.This()
		if e != nil {
			return
		}
		var thisGlobal *gov8.Global
		if !transient {
			thisGlobal, e = a.newGlobal(cs.Scope(), thisObject.Value)
			if e != nil {
				return
			}
		}
		var thisValue *runtimeValue
		if scratch != nil {
			thisValue = &scratch.values[8]
		} else {
			thisValue = new(runtimeValue)
		}
		*thisValue = runtimeValue{runtime: a, global: thisGlobal, local: thisObject.Value, borrowed: true, callbackID: callbackID}
		if !argumentStart.IsZero() {
			a.recordCost("callback:arguments", argumentStart)
		}
		var bodyStart time.Time
		if a.profile != nil && a.profile.Detailed {
			bodyStart = time.Now()
		}
		result, e := function(thisValue, wrapped)
		if !bodyStart.IsZero() {
			a.recordCost("callback:body-inclusive", bodyStart)
			defer a.recordCost("callback:return", time.Now())
		}
		if e != nil {
			// A reentrant JavaScript call may throw any value. Preserve its identity
			// when it crosses this host callback instead of constructing a new Error.
			var thrown *callException
			if errors.As(e, &thrown) {
				if exception, valueErr := a.localCallback(thrown.value); valueErr == nil {
					_ = cs.ThrowException(exception)
					return
				}
			}
			exception, _ := cs.NewError(e.Error())
			_ = cs.ThrowException(exception)
			return
		}
		if result == nil {
			_ = rv.SetUndefined()
			return
		}
		if wrapped, ok := result.(*runtimeValue); ok && wrapped.hostSet {
			local, valueErr := callbackValue(cs, realm, rv, wrapped.host)
			if valueErr != nil {
				exception, _ := cs.NewError(valueErr.Error())
				_ = cs.ThrowException(exception)
				return
			}
			_ = rv.Set(local)
			return
		}
		local, e := a.localCallback(result)
		if e != nil {
			exception, _ := cs.NewError(e.Error())
			_ = cs.ThrowException(exception)
			return
		}
		_ = rv.Set(local)
	}, nil)
	if err != nil {
		return gov8.Value{}, err
	}
	return fn.Value, nil
}

func marshalPrimitive(scope *gov8.Scope, value any) (gov8.Value, bool, error) {
	switch value := value.(type) {
	case nil:
		v, e := scope.Null()
		return v, true, e
	case string:
		v, e := scope.NewString(value)
		return v, true, e
	case bool:
		v, e := scope.Boolean(value)
		return v, true, e
	case int:
		v, e := scope.Number(float64(value))
		return v, true, e
	case int32:
		v, e := scope.Int32(value)
		return v, true, e
	case int64:
		v, e := scope.Number(float64(value))
		return v, true, e
	case uint:
		v, e := scope.Number(float64(value))
		return v, true, e
	case uint32:
		v, e := scope.Uint32(value)
		return v, true, e
	case uint64:
		v, e := scope.Number(float64(value))
		return v, true, e
	case float32:
		v, e := scope.Number(float64(value))
		return v, true, e
	case float64:
		v, e := scope.Number(value)
		return v, true, e
	}
	return gov8.Value{}, false, nil
}

func (a *adapter) marshalCallback(scope *gov8.CallbackScope, value any) (gov8.Value, error) {
	if engineValue, ok := value.(engine.Value); ok {
		return a.localCallback(engineValue)
	}
	callback := a.onCallback()
	if callback == nil {
		return gov8.Value{}, errors.New("not in V8 callback")
	}
	return callbackValue(scope, callback.ctx, callback.result, value)
	/*
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
			object, err := scope.NewObject(); if err != nil { return gov8.Value{}, err }
			iter := rv.MapRange()
			for iter.Next() { member, err := a.marshalCallback(scope, iter.Value().Interface()); if err != nil { return gov8.Value{}, err }; ok, err := scope.ObjectSet(object, iter.Key().String(), member); if err != nil || !ok { return gov8.Value{}, err } }
			return object, nil
		}
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			elements := make([]gov8.Value, rv.Len())
			for i := range elements { var err error; elements[i], err = a.marshalCallback(scope, rv.Index(i).Interface()); if err != nil { return gov8.Value{}, err } }
			return scope.NewArrayWithElements(elements)
		}
		return gov8.Value{}, fmt.Errorf("unsupported callback value %T", value)
	*/
}

func numberAsFloat(value any) (float64, bool) {
	switch value := value.(type) {
	case int:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case float32:
		return float64(value), true
	case float64:
		return value, true
	}
	return 0, false
}

func (a *adapter) export(value *runtimeValue) any {
	if a.profile != nil && a.profile.Detailed {
		defer a.recordCost("conversion:export", time.Now())
	}
	if value.hostSet {
		return value.host
	}
	if callback := a.onCallback(); callback != nil && value.borrowed {
		return exportCallback(value.local, callback)
	}
	var exported any
	_, _ = a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.local(scope, value)
		if err != nil {
			return nil, err
		}
		exported = exportLocalPrimitive(local, realm)
		if _, opaque := exported.(opaqueValue); opaque {
			exported = exportJSON(scope, realm, local, value)
		}
		return nil, nil
	})
	return exported
}

func exportLocalPrimitive(value gov8.Value, realm *gov8.Context) any {
	// DOM hosts predominantly exchange strings and numeric node identities.
	if yes, _ := value.IsString(); yes {
		result, _ := value.StringValue()
		return result
	}
	if yes, _ := value.IsNumber(); yes {
		result, ok, _ := value.NumberValue(realm)
		if ok && !math.IsNaN(result) && !math.IsInf(result, 0) {
			return result
		}
		return nil
	}
	if yes, _ := value.IsNullOrUndefined(); yes {
		return nil
	}
	if yes, _ := value.IsBoolean(); yes {
		result, _ := value.BooleanValue()
		return result
	}
	if yes, _ := value.IsBigInt(); yes {
		result, _ := value.ToString(realm)
		return result
	}
	// Object export is performed by JSON in runtimeValue.String fallback. The
	// full recursive path is handled outside callbacks below.
	return opaqueValue{value: value}
}

type opaqueValue struct{ value gov8.Value }

func (a *adapter) string(value *runtimeValue) string {
	if a.profile != nil && a.profile.Detailed {
		defer a.recordCost("conversion:string", time.Now())
	}
	if value.hostSet {
		return fmt.Sprint(value.host)
	}
	if callback := a.onCallback(); callback != nil && value.borrowed {
		if yes, _ := value.local.IsString(); yes {
			result, _ := value.local.StringValue()
			return result
		}
		result, _ := callback.scope.ToString(value.local)
		return result
	}
	result := "undefined"
	_, _ = a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		local, err := a.local(scope, value)
		if err == nil {
			result, err = local.ToString(realm)
		}
		return nil, err
	})
	return result
}

func localResultString(value gov8.Value, realm *gov8.Context) string {
	result, _ := value.ToString(realm)
	return result
}

func callbackValue(scope *gov8.CallbackScope, realm *gov8.Context, result gov8.ReturnValue, value any) (gov8.Value, error) {
	// DOM result lists are plain by-value records. One native JSON parse replaces
	// one FFI call per property/array slot; arbitrary host objects keep the
	// existing recursive conversion (not all Go values have JSON semantics).
	var projection any
	switch value := value.(type) {
	case []int64:
		if value == nil {
			value = []int64{}
		}
		projection = value
	case []map[string]any, map[string]any:
		if callbackJSONEligible(value) {
			projection = callbackJSONProjection(value)
		}
	}
	if projection != nil {
		encoded, err := json.Marshal(projection)
		if err == nil {
			text, err := scope.NewString(string(encoded))
			if err != nil {
				return gov8.Value{}, err
			}
			return gov8.JSONParse(realm, scope.Scope(), text, nil)
		}
		// Non-finite numbers and unsupported values retain the recursive path.
	}
	switch value := value.(type) {
	case nil:
		_ = result.SetNull()
	case string:
		local, err := scope.NewString(value)
		if err != nil {
			return gov8.Value{}, err
		}
		return local, nil
	case bool:
		_ = result.SetBool(value)
	case int:
		_ = result.SetFloat64(float64(value))
	case int32:
		_ = result.SetInt32(value)
	case int64:
		_ = result.SetFloat64(float64(value))
	case uint:
		_ = result.SetFloat64(float64(value))
	case uint32:
		_ = result.SetUint32(value)
	case uint64:
		_ = result.SetFloat64(float64(value))
	case float32:
		_ = result.SetFloat64(float64(value))
	case float64:
		_ = result.SetFloat64(value)
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
			object, err := scope.NewObject()
			if err != nil {
				return gov8.Value{}, err
			}
			iter := rv.MapRange()
			for iter.Next() {
				member, err := callbackValue(scope, realm, result, iter.Value().Interface())
				if err != nil {
					return gov8.Value{}, err
				}
				ok, err := scope.ObjectSet(object, iter.Key().String(), member)
				if err != nil || !ok {
					return gov8.Value{}, err
				}
			}
			return object, nil
		}
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			elements := make([]gov8.Value, rv.Len())
			for i := range elements {
				var err error
				elements[i], err = callbackValue(scope, realm, result, rv.Index(i).Interface())
				if err != nil {
					return gov8.Value{}, err
				}
			}
			return scope.NewArrayWithElements(elements)
		}
		return gov8.Value{}, fmt.Errorf("unsupported callback value %T", value)
	}
	return result.Get()
}

// Preserve callback marshaling semantics: nil slices/maps are empty JS
// collections, and byte slices are numeric arrays rather than base64 strings.
// Do not let JSON silently serialize engine values, custom structs or marshalers
// that the recursive engine conversion does not support.
func callbackJSONEligible(value any) bool {
	switch value.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return false
		}
		iter := rv.MapRange()
		for iter.Next() {
			if !callbackJSONEligible(iter.Value().Interface()) {
				return false
			}
		}
		return true
	case reflect.Array, reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			if !callbackJSONEligible(rv.Index(i).Interface()) {
				return false
			}
		}
		return true
	}
	return false
}

func callbackJSONProjection(value any) any {
	if value == nil {
		return nil
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return value
		}
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = callbackJSONProjection(iter.Value().Interface())
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = callbackJSONProjection(rv.Index(i).Interface())
		}
		return out
	default:
		return value
	}
}

func exportJSON(scope *gov8.Scope, realm *gov8.Context, value gov8.Value, fallback engine.Value) any {
	script, err := realm.Compile(scope, `(value=>JSON.stringify(value))`, nil)
	if err != nil {
		return fallback
	}
	defer script.Close()
	fnValue, err := script.Run(scope, nil)
	if err != nil {
		return fallback
	}
	fn, ok, err := gov8.AsFunction(fnValue, realm)
	if err != nil || !ok {
		return fallback
	}
	undefined, _ := scope.Undefined()
	encoded, ok, err := fn.Call(scope, undefined, value)
	if err != nil || !ok {
		return fallback
	}
	if absent, _ := encoded.IsUndefined(); absent {
		return fallback
	}
	text, err := encoded.ToString(realm)
	if err != nil {
		return fallback
	}
	var exported any
	if json.Unmarshal([]byte(text), &exported) != nil {
		return fallback
	}
	return exported
}

func exportCallback(value gov8.Value, callback *callbackContext) any {
	primitive := exportLocalPrimitive(value, callback.ctx)
	if _, opaque := primitive.(opaqueValue); !opaque {
		return primitive
	}
	global, err := callback.scope.CurrentContextGlobal()
	if err != nil {
		return nil
	}
	jsonObject, ok, err := callback.scope.ObjectGet(global, "JSON")
	if err != nil || !ok {
		return nil
	}
	stringify, ok, err := callback.scope.ObjectGet(jsonObject, "stringify")
	if err != nil || !ok {
		return nil
	}
	encoded, ok, err := callback.scope.CallFunction(stringify, jsonObject, []gov8.Value{value})
	if err != nil || !ok {
		return nil
	}
	var text string
	if isString, _ := encoded.IsString(); isString {
		text, err = encoded.StringValue()
	} else {
		text, err = callback.scope.ToString(encoded)
	}
	if err != nil || text == "undefined" {
		return nil
	}
	var exported any
	if json.Unmarshal([]byte(text), &exported) != nil {
		return nil
	}
	return exported
}

func goTypeOf(value any) string {
	if value == nil {
		return "object"
	}
	switch value.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case int, int32, int64, uint, uint32, uint64, float32, float64:
		return "number"
	}
	return "object"
}

var _ engine.Factory = Factory{}
var _ engine.Runtime = (*adapter)(nil)
