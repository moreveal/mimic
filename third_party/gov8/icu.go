//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"
	"unsafe"
)

// ICUCommonDataError reports the UErrorCode returned by ICU while installing
// a common-data package. The numeric code intentionally matches rusty_v8's
// Result<(), i32> error value.
type ICUCommonDataError struct {
	Code int32
}

func (e *ICUCommonDataError) Error() string {
	return fmt.Sprintf("gov8: ICU common-data installation failed (UErrorCode %d)", e.Code)
}

// ICUSetCommonData78 installs an ICU 78 common-data package. ICU retains
// successful packages for process lifetime. gov8 therefore makes a 16-byte
// aligned native copy; the caller may immediately reuse or release data.
//
// As with rusty_v8, data must contain a complete ICU common-data package.
func ICUSetCommonData78(data []byte) error {
	_, err := icuSetCommonData78(data)
	return err
}

func icuSetCommonData78(data []byte) (pointerMod16 int32, err error) {
	var code int32
	var dataPointer uintptr
	if len(data) != 0 {
		dataPointer = uintptr(unsafe.Pointer(&data[0]))
	}
	r1, _, _ := proc("gov8_icu_set_common_data_78").Call(
		dataPointer,
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&code)),
		uintptr(unsafe.Pointer(&pointerMod16)),
	)
	runtime.KeepAlive(data)
	if int64(r1) < 0 {
		return 0, shimError("ICUSetCommonData78", r1)
	}
	if code != 0 {
		return pointerMod16, &ICUCommonDataError{Code: code}
	}
	return pointerMod16, nil
}

func icuGetDefault(kind uintptr, operation string) (string, error) {
	var output [1024]byte
	r1, _, _ := proc("gov8_icu_get_default").Call(
		kind,
		uintptr(unsafe.Pointer(&output[0])),
		uintptr(len(output)),
	)
	if int64(r1) < 0 {
		return "", shimError(operation, r1)
	}
	length := int(r1)
	if length < 0 || length > len(output) {
		return "", errors.New("gov8: ICU returned an invalid string length")
	}
	if !utf8.Valid(output[:length]) {
		return "", errors.New("gov8: ICU returned invalid UTF-8")
	}
	return string(output[:length]), nil
}

// ICUGetLanguageTag returns ICU's process-wide default locale as a BCP 47
// language tag.
func ICUGetLanguageTag() (string, error) {
	return icuGetDefault(0, "ICUGetLanguageTag")
}

func icuCString(value, name string) ([]byte, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return nil, fmt.Errorf("gov8: %s contains NUL", name)
	}
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("gov8: %s is not valid UTF-8", name)
	}
	bytes := make([]byte, len(value)+1)
	copy(bytes, value)
	return bytes, nil
}

// ICUSetDefaultLocale sets ICU's process-wide default locale. Unlike
// rusty_v8, which panics while constructing a CString, Go reports interior NUL
// and invalid UTF-8 inputs as errors.
func ICUSetDefaultLocale(locale string) error {
	bytes, err := icuCString(locale, "ICU locale")
	if err != nil {
		return err
	}
	r1, _, _ := proc("gov8_icu_set_default_locale").Call(
		uintptr(unsafe.Pointer(&bytes[0])),
	)
	runtime.KeepAlive(bytes)
	if int64(r1) < 0 {
		return shimError("ICUSetDefaultLocale", r1)
	}
	return nil
}

// ICUGetDefaultTimeZone returns ICU's process-wide default time-zone ID.
func ICUGetDefaultTimeZone() (string, error) {
	return icuGetDefault(1, "ICUGetDefaultTimeZone")
}

// ICUSetDefaultTimeZone installs a process-wide ICU time-zone ID. accepted is
// false, with the prior default unchanged, for unknown IDs and interior NULs.
// Invalid UTF-8 is reported as a Go error because Rust strings cannot contain
// it. Isolates which have observed dates must separately receive a date/time
// configuration change notification, matching rusty_v8's contract.
func ICUSetDefaultTimeZone(timeZoneID string) (accepted bool, err error) {
	if strings.IndexByte(timeZoneID, 0) >= 0 {
		return false, nil
	}
	bytes, err := icuCString(timeZoneID, "ICU time-zone ID")
	if err != nil {
		return false, err
	}
	var acceptedFlag int32
	r1, _, _ := proc("gov8_icu_set_default_time_zone").Call(
		uintptr(unsafe.Pointer(&bytes[0])),
		uintptr(unsafe.Pointer(&acceptedFlag)),
	)
	runtime.KeepAlive(bytes)
	if int64(r1) < 0 {
		return false, shimError("ICUSetDefaultTimeZone", r1)
	}
	return acceptedFlag != 0, nil
}
