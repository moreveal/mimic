// Package layoutblitz owns the native derived document used during migration.
// Calls must run on the owning Page's event loop. Separate Pages share no lock.
package layoutblitz

/*
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/native/target/x86_64-pc-windows-gnu/release/libmimic_layout_blitz.a -lws2_32 -luserenv -lbcrypt -lntdll
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/native/target/x86_64-unknown-linux-gnu/release/libmimic_layout_blitz.a -ldl -lpthread -lm
#include <stdint.h>
#include <stddef.h>
typedef struct MimicBlitzHandle MimicBlitzHandle;
typedef struct { float x,y,width,height; } MimicBlitzRect;
MimicBlitzHandle* mimic_blitz_new(uint64_t, uint32_t, uint32_t);
void mimic_blitz_drop(MimicBlitzHandle*);
int32_t mimic_blitz_element(MimicBlitzHandle*, uint64_t, const char*, size_t, const char*, size_t);
int32_t mimic_blitz_attribute(MimicBlitzHandle*, uint64_t, const char*, size_t, const char*, size_t, const char*, size_t);
int32_t mimic_blitz_append(MimicBlitzHandle*, uint64_t, uint64_t);
int32_t mimic_blitz_text(MimicBlitzHandle*, uint64_t, const char*, size_t, uint32_t);
int32_t mimic_blitz_resolve(MimicBlitzHandle*, double, uint64_t*);
int32_t mimic_blitz_rect(MimicBlitzHandle*, uint64_t, MimicBlitzRect*);
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

type Owner struct{ handle *C.MimicBlitzHandle }
type Rect struct{ X, Y, Width, Height float64 }

func New(root uint64, width, height uint32) (*Owner, error) {
	p := C.mimic_blitz_new(C.uint64_t(root), C.uint32_t(width), C.uint32_t(height))
	if p == nil {
		return nil, fmt.Errorf("blitz: document creation failed")
	}
	return &Owner{handle: p}, nil
}

// Close must be called when the canonical Document is disposed, including on
// failed navigation. It is idempotent; native state never outlives that owner.
func (o *Owner) Close() {
	if o.handle != nil {
		C.mimic_blitz_drop(o.handle)
		o.handle = nil
	}
}

func check(status C.int32_t) error {
	if status != 0 {
		return fmt.Errorf("blitz: native transaction failed (%d); resynchronization required", int32(status))
	}
	return nil
}
func chars(s string) *C.char { return (*C.char)(unsafe.Pointer(unsafe.StringData(s))) }

func (o *Owner) Element(id uint64, namespace, name string) error {
	status := C.mimic_blitz_element(o.handle, C.uint64_t(id), chars(namespace), C.size_t(len(namespace)), chars(name), C.size_t(len(name)))
	runtime.KeepAlive(namespace)
	runtime.KeepAlive(name)
	return check(status)
}
func (o *Owner) Attribute(id uint64, namespace, name, value string) error {
	status := C.mimic_blitz_attribute(o.handle, C.uint64_t(id), chars(namespace), C.size_t(len(namespace)), chars(name), C.size_t(len(name)), chars(value), C.size_t(len(value)))
	runtime.KeepAlive(namespace)
	runtime.KeepAlive(name)
	runtime.KeepAlive(value)
	return check(status)
}
func (o *Owner) Append(parent, child uint64) error {
	return check(C.mimic_blitz_append(o.handle, C.uint64_t(parent), C.uint64_t(child)))
}
func (o *Owner) Text(id uint64, value string, create bool) error {
	flag := C.uint32_t(0)
	if create {
		flag = 1
	}
	status := C.mimic_blitz_text(o.handle, C.uint64_t(id), chars(value), C.size_t(len(value)), flag)
	runtime.KeepAlive(value)
	return check(status)
}
func (o *Owner) Resolve(time float64) (uint64, error) {
	var generation C.uint64_t
	err := check(C.mimic_blitz_resolve(o.handle, C.double(time), &generation))
	return uint64(generation), err
}
func (o *Owner) Rect(id uint64) (Rect, error) {
	var rect C.MimicBlitzRect
	err := check(C.mimic_blitz_rect(o.handle, C.uint64_t(id), &rect))
	return Rect{float64(rect.x), float64(rect.y), float64(rect.width), float64(rect.height)}, err
}
