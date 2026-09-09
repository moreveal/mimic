package gojaengine

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/moreveal/mimic/internal/engine"
)

type Factory struct{}

func (Factory) New() engine.Runtime { return &runtime{vm: goja.New()} }

type value struct{ v goja.Value }

func (v value) Export() any    { return v.v.Export() }
func (v value) String() string { return v.v.String() }

type runtime struct{ vm *goja.Runtime }

func (r *runtime) Eval(ctx context.Context, source, name string) (engine.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	finished := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			select {
			case <-finished:
				return
			default:
				r.vm.Interrupt(ctx.Err())
			}
		case <-finished:
		}
	}()
	v, err := r.vm.RunScript(name, source)
	close(finished)
	<-watcherDone
	r.vm.ClearInterrupt()
	if err != nil {
		return nil, jsError(err)
	}
	return value{v}, nil
}

func nativeValue(v any) any {
	if wrapped, ok := v.(value); ok {
		return wrapped.v
	}
	return v
}

func (r *runtime) Set(name string, v any) error { return r.vm.Set(name, nativeValue(v)) }
func (r *runtime) Get(name string) engine.Value { return value{r.vm.Get(name)} }
func (r *runtime) Value(v any) engine.Value     { return value{r.vm.ToValue(nativeValue(v))} }
func (r *runtime) GetProperty(v engine.Value, name string) engine.Value {
	return value{unwrap(v).ToObject(r.vm).Get(name)}
}
func (r *runtime) SetProperty(v engine.Value, name string, x any) error {
	return unwrap(v).ToObject(r.vm).Set(name, nativeValue(x))
}
func (r *runtime) TypeOf(v engine.Value) string {
	x := unwrap(v)
	if x == nil || goja.IsUndefined(x) {
		return "undefined"
	}
	if goja.IsNull(x) {
		return "object"
	}
	if _, ok := goja.AssertFunction(x); ok {
		return "function"
	}
	// Exporting an object reads enumerable properties and can invoke getters.
	// JavaScript typeof never observes those properties or converts the object.
	if _, ok := x.(*goja.Object); ok {
		return "object"
	}
	if _, ok := x.(*goja.Symbol); ok {
		return "symbol"
	}
	switch x.Export().(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case int, int32, int64, float32, float64:
		return "number"
	case *big.Int:
		return "bigint"
	default:
		return "object"
	}
}

func (r *runtime) StrictEqual(a, b engine.Value) bool {
	return unwrap(a).StrictEquals(unwrap(b))
}

func unwrap(v engine.Value) goja.Value {
	if v == nil {
		return goja.Undefined()
	}
	if x, ok := v.(value); ok {
		return x.v
	}
	panic("value belongs to another JavaScript engine")
}

func (r *runtime) Call(ctx context.Context, fn, this engine.Value, args ...engine.Value) (engine.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, ok := goja.AssertFunction(unwrap(fn))
	if !ok {
		return nil, fmt.Errorf("value is not callable")
	}
	a := make([]goja.Value, len(args))
	for i := range args {
		a[i] = unwrap(args[i])
	}
	finished := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			select {
			case <-finished:
				return
			default:
				r.vm.Interrupt(ctx.Err())
			}
		case <-finished:
		}
	}()
	v, err := f(unwrap(this), a...)
	close(finished)
	<-watcherDone
	r.vm.ClearInterrupt()
	if err != nil {
		return nil, jsError(err)
	}
	return value{v}, nil
}

func jsError(err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if cause, ok := interrupted.Value().(error); ok {
			return fmt.Errorf("%w: %s", cause, interrupted.String())
		}
		return fmt.Errorf("JavaScript execution interrupted: %v", interrupted.Value())
	}
	if exception, ok := err.(*goja.Exception); ok {
		return errors.New(strings.TrimSpace(exception.String()))
	}
	return err
}

func (r *runtime) Function(fn engine.Function) any {
	return func(call goja.FunctionCall) goja.Value {
		args := make([]engine.Value, len(call.Arguments))
		for i := range call.Arguments {
			args[i] = value{call.Arguments[i]}
		}
		v, err := fn(value{call.This}, args)
		if err != nil {
			panic(r.vm.NewTypeError("%s", err))
		}
		return unwrap(v)
	}
}

func (r *runtime) SetGlobalAccessObserver(observer func(name string, supported bool)) {
	target := r.vm.GlobalObject()
	seen := make(map[string]struct{})
	report := func(property string, supported bool) {
		if _, ok := seen[property]; ok {
			return
		}
		seen[property] = struct{}{}
		observer(property, supported)
	}
	reflect := r.vm.Get("Reflect").ToObject(r.vm)
	hasFn, _ := goja.AssertFunction(reflect.Get("has"))
	has := func(property string) bool {
		v, err := hasFn(goja.Undefined(), target, r.vm.ToValue(property))
		return err == nil && v.ToBoolean()
	}
	proxy := r.vm.NewProxy(target, &goja.ProxyTrapConfig{
		Has: func(_ *goja.Object, property string) bool {
			supported := has(property)
			report(property, supported)
			return supported
		},
		Get: func(_ *goja.Object, property string, _ goja.Value) goja.Value {
			supported := has(property)
			report(property, supported)
			return target.Get(property)
		},
	})
	r.vm.SetGlobalObject(r.vm.ToValue(proxy).(*goja.Object))
}

func (r *runtime) NewPromise() engine.Promise {
	p, resolve, reject := r.vm.NewPromise()
	return engine.Promise{
		Value:   value{r.vm.ToValue(p)},
		Resolve: func(v any) error { return resolve(v) },
		Reject:  func(v any) error { return reject(v) },
	}
}
func (r *runtime) Await(v engine.Value) (engine.Value, bool, error) {
	raw := unwrap(v)
	// goja can return a nil Value while a realm is being replaced by an
	// asynchronous navigation. Treat it as JavaScript undefined at the neutral
	// engine boundary instead of leaking a Go nil-pointer panic.
	if raw == nil {
		return value{goja.Undefined()}, true, nil
	}
	p, ok := raw.Export().(*goja.Promise)
	if !ok {
		return v, true, nil
	}
	switch p.State() {
	case goja.PromiseStateFulfilled:
		return value{p.Result()}, true, nil
	case goja.PromiseStateRejected:
		return nil, true, fmt.Errorf("promise rejected: %s", p.Result().String())
	default:
		return v, false, nil
	}
}
func (r *runtime) SetTimeSource(now func() time.Time) { r.vm.SetTimeSource(now) }

// goja drains its Promise job queue when control returns from RunScript or a
// host call. An empty script provides an explicit browser scheduler checkpoint.
func (r *runtime) MicrotaskCheckpoint() error {
	_, err := r.vm.RunString("void 0")
	return err
}
func (r *runtime) Close() error { return nil }

func (r *runtime) DetachArrayBuffer(v engine.Value) error {
	object, ok := unwrap(v).(*goja.Object)
	if !ok || object.ExportType() != reflect.TypeOf(goja.ArrayBuffer{}) {
		return errors.New("expected ArrayBuffer")
	}
	buffer, ok := object.Export().(goja.ArrayBuffer)
	if !ok {
		return errors.New("expected ArrayBuffer")
	}
	buffer.Detach()
	return nil
}
