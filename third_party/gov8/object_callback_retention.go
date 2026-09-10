//go:build windows && amd64

package gov8

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// AccessorConfiguration is the Go counterpart of v8::AccessorConfiguration.
// Getter is required. Data is retained by the isolate and is returned
// verbatim by PropertyCallbackArguments.Data even when its creation scope has
// closed. A zero Data value means no associated data (callbacks observe
// undefined).
type AccessorConfiguration struct {
	Getter    AccessorGetterCallback
	Setter    AccessorSetterCallback
	Data      Value
	Attribute PropertyAttribute
}

// LazyDataPropertyConfiguration controls Object::SetLazyDataProperty. The
// getter is called until it completes successfully, after which V8 replaces
// the lazy property with the returned value (undefined when the callback did
// not set ReturnValue). Data has the same isolate-owned retention semantics as
// AccessorConfiguration.Data.
type LazyDataPropertyConfiguration struct {
	Getter               AccessorGetterCallback
	Data                 Value
	Attribute            PropertyAttribute
	GetterSideEffectType SideEffectType
	SetterSideEffectType SideEffectType
}

var (
	lazyDataPropertyProcOnce sync.Once
	lazyDataPropertyProcAddr uintptr
)

func resolveLazyDataPropertyProc() {
	lazyDataPropertyProcAddr = proc("gov8_ocr_object_set_lazy_data_property_direct").Addr()
}

// lazyReceiverArgs validates one lazy-property receiver operation while
// avoiding repeated affinity checks after the receiver has established the
// isolate's owner thread. Context and scope state cannot change concurrently
// on that thread.
func (o *Object) lazyReceiverArgs(s *Scope, c *Context, key Value) (uintptr, error) {
	if err := o.check(); err != nil {
		return 0, err
	}
	if c == nil || c.iso != o.iso {
		return 0, foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return 0, err
	}
	if s == nil || s.iso != o.iso {
		return 0, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, err
	}
	if key.h == 0 {
		return 0, fmt.Errorf("gov8: zero value handle")
	}
	if key.iso != o.iso {
		return 0, foreignIsolate("key")
	}
	if key.sc == nil || key.sc.closed {
		return 0, fmt.Errorf("gov8: scope used after Close")
	}
	return sh, nil
}

func validPropertyAttribute(attr PropertyAttribute) error {
	if attr & ^PropertyAttribute(AttrReadOnly|AttrDontEnum|AttrDontDelete) != 0 {
		return fmt.Errorf("gov8: invalid property attributes %#x", uint8(attr))
	}
	return nil
}

func validSideEffectType(side SideEffectType) error {
	if side > SideEffectHasSideEffectToReceiver {
		return fmt.Errorf("gov8: invalid side-effect type %d", side)
	}
	return nil
}

// SetAccessorWithConfiguration installs an instance-level accessor. Unlike
// the lower-level V8 signature, the pinned rusty_v8 configuration always has
// a getter; Go rejects a missing getter before crossing the callback ABI.
func (o *Object) SetAccessorWithConfiguration(s *Scope, c *Context, key Value, configuration AccessorConfiguration) (bool, error) {
	sh, err := o.receiverArgs(s, c)
	if err != nil {
		return false, err
	}
	if err := o.nameArg(key); err != nil {
		return false, err
	}
	if configuration.Getter == nil {
		return false, fmt.Errorf("gov8: accessor configuration requires a getter")
	}
	if err := validPropertyAttribute(configuration.Attribute); err != nil {
		return false, err
	}
	handle, err := registerAccessorCallbacks(o.iso, configuration.Getter, configuration.Setter, configuration.Data)
	if err != nil {
		return false, err
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		dropHostCallback(handle)
		return false, errLostCallbackRegistration
	}
	var okv int32
	withSetter := uintptr(0)
	if configuration.Setter != nil {
		withSetter = 1
	}
	r1, _, _ := proc("gov8_ocr_object_set_accessor_configuration").Call(
		o.iso.handleAssumingCheck(), c.handle, sh, o.h, key.h, entry.ctx,
		withSetter, uintptr(configuration.Attribute), uintptr(unsafe.Pointer(&okv)))
	if int64(r1) < 0 {
		dropHostCallback(handle)
		return false, shimError("Object.SetAccessorWithConfiguration", r1)
	}
	if okv != 1 {
		dropHostCallback(handle)
	}
	return okv == 1, nil
}

// SetLazyDataPropertyWithConfiguration installs a lazy data property with
// explicit callback data, attributes, and debugger side-effect metadata.
// V8 152 CHECK-fails for a setter side-effect type of HasNoSideEffect; the Go
// API turns that fatal-only precondition into a deterministic error.
func (o *Object) SetLazyDataPropertyWithConfiguration(s *Scope, c *Context, key Value, configuration LazyDataPropertyConfiguration) (bool, error) {
	sh, err := o.lazyReceiverArgs(s, c, key)
	if err != nil {
		return false, err
	}
	// The shim validates the dynamic Go Value as a V8 Name at the same boundary
	// that consumes it. Avoid a separate DLL round trip for the same predicate.
	if configuration.Getter == nil {
		return false, errNilLazyGetter
	}
	if err := validPropertyAttribute(configuration.Attribute); err != nil {
		return false, err
	}
	if err := validSideEffectType(configuration.GetterSideEffectType); err != nil {
		return false, err
	}
	if err := validSideEffectType(configuration.SetterSideEffectType); err != nil {
		return false, err
	}
	if configuration.SetterSideEffectType == SideEffectHasNoSideEffect {
		return false, fmt.Errorf("gov8: lazy setter side-effect type HasNoSideEffect is invalid")
	}
	handle, entry, cacheKey, created, err := registerLazyGetter(o.iso, configuration.Getter, configuration.Data)
	if err != nil {
		return false, err
	}
	if entry == nil {
		if created {
			dropNewLazyGetter(handle, cacheKey)
		}
		return false, errLostCallbackRegistration
	}
	lazyDataPropertyProcOnce.Do(resolveLazyDataPropertyProc)
	r1, _, _ := syscall.Syscall9(lazyDataPropertyProcAddr, 9,
		o.iso.handleAssumingCheck(), c.handle, sh, o.h, key.h, entry.ctx,
		uintptr(configuration.Attribute), uintptr(configuration.GetterSideEffectType),
		uintptr(configuration.SetterSideEffectType))
	if int64(r1) < 0 {
		if created {
			dropNewLazyGetter(handle, cacheKey)
		}
		return false, shimError("Object.SetLazyDataPropertyWithConfiguration", r1)
	}
	if r1 != 1 && created {
		dropNewLazyGetter(handle, cacheKey)
	}
	return r1 == 1, nil
}

// SetLazyDataPropertyWithData is the positional counterpart of rusty_v8's
// set_lazy_data_property_with_data.
func (o *Object) SetLazyDataPropertyWithData(s *Scope, c *Context, key Value, getter AccessorGetterCallback, data Value, attr PropertyAttribute, getterSideEffectType, setterSideEffectType SideEffectType) (bool, error) {
	return o.SetLazyDataPropertyWithConfiguration(s, c, key, LazyDataPropertyConfiguration{
		Getter:               getter,
		Data:                 data,
		Attribute:            attr,
		GetterSideEffectType: getterSideEffectType,
		SetterSideEffectType: setterSideEffectType,
	})
}

// SetDataWithAttr adds supported V8 Data to every object created from this
// template. JavaScript primitives and nested FunctionTemplate or
// ObjectTemplate values are accepted. JSReceiver values and internal metadata
// are rejected before V8's fatal Template::Set boundary.
func (t *ObjectTemplate) SetDataWithAttr(key string, data Data, attr PropertyAttribute) error {
	if t == nil {
		return fmt.Errorf("gov8: nil object template")
	}
	if err := t.check(); err != nil {
		return err
	}
	return setTemplateDataWithAttr("ObjectTemplate.SetDataWithAttr", t.iso, t.sc, t.h, 0, key, data, attr)
}

// SetDataWithAttr adds supported V8 Data to the function object instantiated
// from this template. It has the same safety boundary as ObjectTemplate's
// method.
func (t *FunctionTemplate) SetDataWithAttr(key string, data Data, attr PropertyAttribute) error {
	if t == nil {
		return fmt.Errorf("gov8: nil function template")
	}
	if err := t.check(); err != nil {
		return err
	}
	return setTemplateDataWithAttr("FunctionTemplate.SetDataWithAttr", t.iso, t.sc, t.h, 1, key, data, attr)
}

func setTemplateDataWithAttr(op string, iso *Isolate, scope *Scope, template uintptr, templateKind uintptr, key string, data Data, attr PropertyAttribute) error {
	if iso == nil || scope == nil || template == 0 {
		return fmt.Errorf("gov8: invalid template")
	}
	if err := scope.check(); err != nil {
		return err
	}
	if err := data.check(); err != nil {
		return err
	}
	if data.iso != iso {
		return foreignIsolate("data")
	}
	isFunctionTemplate, err := data.IsFunctionTemplate()
	if err != nil {
		return err
	}
	isObjectTemplate, err := data.IsObjectTemplate()
	if err != nil {
		return err
	}
	isPrimitive, err := data.IsPrimitive()
	if err != nil {
		return err
	}
	if !isFunctionTemplate && !isObjectTemplate && !isPrimitive {
		return fmt.Errorf("gov8: template data must be a primitive Value, FunctionTemplate, or ObjectTemplate; JSReceiver and internal metadata values are unsafe")
	}
	if err := validPropertyAttribute(attr); err != nil {
		return err
	}
	k, err := scope.NewString(key)
	if err != nil {
		return err
	}
	return callErr(op, proc("gov8_ocr_template_set_data_with_attr"),
		iso.handleAssumingCheck(), scope.handle, template, templateKind,
		k.h, data.h, uintptr(attr))
}

// SetAccessorWithConfiguration installs a native accessor on every instance
// produced by this ObjectTemplate. The callback data is isolate-retained and
// Attribute is applied to the instantiated property. Keys are strings in the
// current Go template surface; the pinned crate additionally accepts Symbols.
func (t *ObjectTemplate) SetAccessorWithConfiguration(key string, configuration AccessorConfiguration) error {
	if t == nil {
		return fmt.Errorf("gov8: nil object template")
	}
	if err := t.check(); err != nil {
		return err
	}
	if configuration.Getter == nil {
		return fmt.Errorf("gov8: accessor configuration requires a getter")
	}
	if err := validPropertyAttribute(configuration.Attribute); err != nil {
		return err
	}
	handle, err := registerAccessorCallbacks(t.iso, configuration.Getter, configuration.Setter, configuration.Data)
	if err != nil {
		return err
	}
	keyValue, err := t.sc.NewString(key)
	if err != nil {
		dropHostCallback(handle)
		return err
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		dropHostCallback(handle)
		return errLostCallbackRegistration
	}
	withSetter := uintptr(0)
	if configuration.Setter != nil {
		withSetter = 1
	}
	err = callErr("ObjectTemplate.SetAccessorWithConfiguration",
		proc("gov8_object_template_set_native_data_property"),
		t.iso.handleAssumingCheck(), t.sc.handle, t.h, keyValue.h, entry.ctx,
		withSetter, uintptr(configuration.Attribute))
	if err != nil {
		dropHostCallback(handle)
	}
	return err
}
