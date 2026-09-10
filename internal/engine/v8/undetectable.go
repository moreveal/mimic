//go:build windows && amd64

package v8

import (
	"fmt"
	"strconv"

	"github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

// NewUndetectableObject creates a callable HTMLDDA exotic object. The native
// handlers keep JS state in callback Data, rooted by the isolate's registry.
func (a *adapter) NewUndetectableObject(handler engine.Value) (engine.Value, error) {
	makeObject := func(iso *gov8.Isolate, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		data, err := a.local(scope, handler)
		if err != nil {
			return nil, err
		}
		ot, err := iso.NewObjectTemplate(scope)
		if err != nil {
			return nil, err
		}
		if err = ot.MarkAsUndetectable(); err != nil {
			return nil, err
		}
		named := gov8.NamedPropertyHandlerConfig{Data: data}
		// Real own properties override named collection entries, but cross-realm
		// forwarding objects may need to intercept their local target properties.
		handlerObj, err := gov8.AsObject(data)
		if err != nil {
			return nil, fmt.Errorf("HTMLDDA handler must be an object")
		}
		nonMasking, _, err := handlerObj.GetByName(scope, realm, "nonMasking")
		if err != nil {
			return nil, err
		}
		useNonMasking, err := nonMasking.BooleanValue()
		if err != nil {
			return nil, err
		}
		if useNonMasking {
			named.Flags = gov8.HandlerFlagNonMasking
		}
		named.Query = ddaQuery
		named.Getter = func(cs *gov8.CallbackScope, k gov8.Value, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return ddaProperty(cs, args, rv, "get", k)
		}
		named.Setter = func(cs *gov8.CallbackScope, k, v gov8.Value, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return ddaProperty(cs, args, rv, "set", k, v)
		}
		named.Deleter = func(cs *gov8.CallbackScope, k gov8.Value, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return ddaProperty(cs, args, rv, "deleteProperty", k)
		}
		named.Descriptor = func(cs *gov8.CallbackScope, k gov8.Value, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return ddaProperty(cs, args, rv, "getOwnPropertyDescriptor", k)
		}
		named.Definer = func(cs *gov8.CallbackScope, k gov8.Value, d gov8.CallbackPropertyDescriptor, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			value, err := ddaDescriptor(cs, d)
			if err != nil {
				ddaThrow(cs, err)
				return gov8.InterceptedYes
			}
			return ddaProperty(cs, args, rv, "defineProperty", k, value)
		}
		named.Enumerator = func(cs *gov8.CallbackScope, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) {
			ddaKeys(cs, args, rv, false)
		}
		indexed := gov8.IndexedPropertyHandlerConfig{Data: data}
		key := func(cs *gov8.CallbackScope, i uint32) gov8.Value {
			k, _ := cs.NewString(strconv.FormatUint(uint64(i), 10))
			return k
		}
		indexed.Query = func(cs *gov8.CallbackScope, i uint32, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return ddaQuery(cs, key(cs, i), args, rv)
		}
		indexed.Getter = func(cs *gov8.CallbackScope, i uint32, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return named.Getter(cs, key(cs, i), args, rv)
		}
		indexed.Setter = func(cs *gov8.CallbackScope, i uint32, v gov8.Value, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return named.Setter(cs, key(cs, i), v, args, rv)
		}
		indexed.Deleter = func(cs *gov8.CallbackScope, i uint32, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return named.Deleter(cs, key(cs, i), args, rv)
		}
		indexed.Descriptor = func(cs *gov8.CallbackScope, i uint32, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return named.Descriptor(cs, key(cs, i), args, rv)
		}
		indexed.Definer = func(cs *gov8.CallbackScope, i uint32, d gov8.CallbackPropertyDescriptor, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
			return named.Definer(cs, key(cs, i), d, args, rv)
		}
		indexed.Enumerator = func(cs *gov8.CallbackScope, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) {
			ddaKeys(cs, args, rv, true)
		}
		if err = ot.SetNamedPropertyHandler(named); err != nil {
			return nil, err
		}
		if err = ot.SetIndexedPropertyHandler(indexed); err != nil {
			return nil, err
		}
		err = ot.SetCallAsFunctionHandler(func(cs *gov8.CallbackScope, args gov8.FunctionCallbackArguments, rv gov8.ReturnValue) {
			d, e := args.Data()
			if e != nil {
				ddaThrow(cs, e)
				return
			}
			values := make([]gov8.Value, args.Length())
			for i := range values {
				values[i], e = args.Get(i)
				if e != nil {
					ddaThrow(cs, e)
					return
				}
			}
			list, e := cs.NewArrayWithElements(values)
			if e != nil {
				ddaThrow(cs, e)
				return
			}
			construct, e := cs.Scope().Boolean(args.IsConstructCall())
			if e != nil {
				ddaThrow(cs, e)
				return
			}
			result, ok, e := ddaInvoke(cs, d, "call", list, construct)
			if e != nil {
				ddaThrow(cs, e)
				return
			}
			if ok {
				_ = rv.Set(result)
			}
		}, data)
		if err != nil {
			return nil, err
		}
		object, ok, err := ot.NewInstance(scope, realm)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("V8 rejected HTMLDDA instance")
		}
		return a.persist(scope, object.Value)
	}
	if callback := a.onCallback(); callback != nil {
		return makeObject(callback.scope.Isolate(), callback.ctx, callback.scope.Scope())
	}
	return a.run(func(s *state, realm *gov8.Context, scope *gov8.Scope) (engine.Value, error) {
		return makeObject(s.isolate, realm, scope)
	})
}

type ddaPendingException struct{}

func (ddaPendingException) Error() string { return "JavaScript handler threw" }
func ddaThrow(cs *gov8.CallbackScope, err error) {
	if _, pending := err.(ddaPendingException); pending {
		return
	}
	if e, e2 := cs.NewError(err.Error()); e2 == nil {
		_ = cs.ThrowException(e)
	}
}
func ddaInvoke(cs *gov8.CallbackScope, data gov8.Value, name string, args ...gov8.Value) (result gov8.Value, ok bool, err error) {
	catcher, err := cs.Isolate().NewTryCatch()
	if err != nil {
		return gov8.Value{}, false, err
	}
	defer func() {
		caught, _ := catcher.HasCaught()
		if !caught {
			_ = catcher.Close()
			return
		}
		exception, present, readErr := catcher.Exception(cs.Scope())
		_ = catcher.Close()
		if readErr != nil {
			result, ok, err = gov8.Value{}, false, readErr
			return
		}
		if present {
			_ = cs.ThrowException(exception)
			result, ok, err = gov8.Value{}, false, ddaPendingException{}
		}
	}()
	fn, found, err := cs.ObjectGet(data, name)
	if err != nil || !found {
		return gov8.Value{}, false, err
	}
	undefined, err := fn.IsUndefined()
	if err != nil || undefined {
		return gov8.Value{}, false, err
	}
	return cs.CallFunction(fn, data, args)
}

func ddaProperty(cs *gov8.CallbackScope, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue, name string, values ...gov8.Value) gov8.Intercepted {
	data, err := args.Data()
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	tuple, ok, err := ddaInvoke(cs, data, name, values...)
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	if !ok {
		return gov8.InterceptedNo
	}
	handled, _, err := cs.ObjectGet(tuple, "0")
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	yes, err := handled.BooleanValue()
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	if !yes {
		return gov8.InterceptedNo
	}
	result, _, err := cs.ObjectGet(tuple, "1")
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	_ = rv.Set(result)
	return gov8.InterceptedYes
}
func ddaKeys(cs *gov8.CallbackScope, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue, indexed bool) {
	data, err := args.Data()
	if err != nil {
		ddaThrow(cs, err)
		return
	}
	list, ok, err := ddaInvoke(cs, data, "ownKeys")
	if err != nil {
		ddaThrow(cs, err)
		return
	}
	if !ok {
		return
	}
	length, _, err := cs.ObjectGet(list, "length")
	if err != nil {
		ddaThrow(cs, err)
		return
	}
	n, _, err := cs.IntegerValue(length)
	if err != nil {
		ddaThrow(cs, err)
		return
	}
	keys := []gov8.Value{}
	for i := int64(0); i < n; i++ {
		k, _, e := cs.ObjectGet(list, strconv.FormatInt(i, 10))
		if e != nil {
			ddaThrow(cs, e)
			return
		}
		symbol, e := k.IsSymbol()
		if e != nil {
			ddaThrow(cs, e)
			return
		}
		if symbol {
			if !indexed {
				keys = append(keys, k)
			}
			continue
		}
		str, e := cs.ToString(k)
		if e != nil {
			ddaThrow(cs, e)
			return
		}
		u, e := strconv.ParseUint(str, 10, 32)
		isIndex := e == nil && u < 4294967295 && strconv.FormatUint(u, 10) == str
		if isIndex != indexed {
			continue
		}
		if indexed {
			k, e = cs.Scope().Uint32(uint32(u))
			if e != nil {
				ddaThrow(cs, e)
				return
			}
		}
		keys = append(keys, k)
	}
	result, err := cs.NewArrayWithElements(keys)
	if err != nil {
		ddaThrow(cs, err)
		return
	}
	_ = rv.Set(result)
}
func ddaDescriptor(cs *gov8.CallbackScope, d gov8.CallbackPropertyDescriptor) (gov8.Value, error) {
	obj, err := cs.NewObject()
	if err != nil {
		return obj, err
	}
	if d.HasValue() {
		v, _, e := d.Value()
		if e != nil {
			return obj, e
		}
		if _, e = cs.ObjectSet(obj, "value", v); e != nil {
			return obj, e
		}
	}
	for _, field := range []struct {
		name string
		read func() (gov8.Value, bool, error)
	}{{"get", d.Get}, {"set", d.Set}} {
		v, ok, e := field.read()
		if e != nil {
			return obj, e
		}
		if ok {
			if _, e = cs.ObjectSet(obj, field.name, v); e != nil {
				return obj, e
			}
		}
	}
	for _, field := range []struct {
		name       string
		has, value bool
	}{{"writable", d.HasWritable(), d.Writable()}, {"enumerable", d.HasEnumerable(), d.Enumerable()}, {"configurable", d.HasConfigurable(), d.Configurable()}} {
		if field.has {
			v, e := cs.Scope().Boolean(field.value)
			if e != nil {
				return obj, e
			}
			if _, e = cs.ObjectSet(obj, field.name, v); e != nil {
				return obj, e
			}
		}
	}
	return obj, nil
}

// V8 asks the query interceptor for enumerability during Object.keys/for-in;
// its descriptor interceptor alone does not filter enumerator results.
func ddaQuery(cs *gov8.CallbackScope, key gov8.Value, args gov8.PropertyCallbackArguments, rv gov8.ReturnValue) gov8.Intercepted {
	data, err := args.Data()
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	tuple, ok, err := ddaInvoke(cs, data, "getOwnPropertyDescriptor", key)
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	if !ok {
		return gov8.InterceptedNo
	}
	handled, _, err := cs.ObjectGet(tuple, "0")
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	yes, err := handled.BooleanValue()
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	if !yes {
		return gov8.InterceptedNo
	}
	descriptor, _, err := cs.ObjectGet(tuple, "1")
	if err != nil {
		ddaThrow(cs, err)
		return gov8.InterceptedYes
	}
	var attrs int32
	for _, field := range []struct {
		name string
		flag int32
	}{{"writable", 1}, {"enumerable", 2}, {"configurable", 4}} {
		v, _, e := cs.ObjectGet(descriptor, field.name)
		if e != nil {
			ddaThrow(cs, e)
			return gov8.InterceptedYes
		}
		b, e := v.BooleanValue()
		if e != nil {
			ddaThrow(cs, e)
			return gov8.InterceptedYes
		}
		if !b {
			attrs |= field.flag
		}
	}
	_ = rv.SetInt32(attrs)
	return gov8.InterceptedYes
}
