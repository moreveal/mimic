//go:build windows && amd64

package gov8

import (
	"errors"
	"unsafe"
)

// SetSupportsLegacyWireFormat controls acceptance of headerless and old
// structured-clone wire formats. It must be called before ReadHeader or
// ReadValue. V8 treats late mutation as an invalid state; Go rejects it before
// crossing the native boundary. Repeated calls before reading are allowed and
// the last value wins.
func (vd *ValueDeserializer) SetSupportsLegacyWireFormat(enabled bool) error {
	if err := vd.check(); err != nil {
		return err
	}
	if vd.readStarted {
		return errors.New("gov8: legacy wire-format support must be configured before reading")
	}
	return setDeserializerLegacy(vd.handle, 0, enabled)
}

// GetWireFormatVersion reports the detected version. Versioned data becomes
// meaningful after ReadHeader; accepted headerless data reports version 0.
func (vd *ValueDeserializer) GetWireFormatVersion() (uint32, error) {
	if err := vd.check(); err != nil {
		return 0, err
	}
	return deserializerWireVersion(vd.handle, 0)
}

// SetSupportsLegacyWireFormat is the delegate-backed counterpart of
// ValueDeserializer.SetSupportsLegacyWireFormat.
func (vd *DelegateValueDeserializer) SetSupportsLegacyWireFormat(enabled bool) error {
	if err := vd.check(); err != nil {
		return err
	}
	if vd.readStarted {
		return errors.New("gov8: legacy wire-format support must be configured before reading")
	}
	return setDeserializerLegacy(vd.handle, 1, enabled)
}

func setDeserializerLegacy(handle uintptr, family uintptr, enabled bool) error {
	mode := uintptr(0)
	if enabled {
		mode = 1
	}
	return callErr("ValueDeserializer.SetSupportsLegacyWireFormat",
		proc("gov8_swlr_deserializer_set_legacy"), handle, family, mode)
}

func deserializerWireVersion(handle uintptr, family uintptr) (uint32, error) {
	var out uint32
	if err := callErr("ValueDeserializer.GetWireFormatVersion",
		proc("gov8_swlr_deserializer_wire_version"), handle, family,
		uintptr(unsafe.Pointer(&out))); err != nil {
		return 0, err
	}
	return out, nil
}
