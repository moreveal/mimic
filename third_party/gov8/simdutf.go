//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

var (
	simdutfProcsOnce             sync.Once
	simdutfValidateProc          *syscall.Proc
	simdutfConvertProc           *syscall.Proc
	simdutfMeasureProc           *syscall.Proc
	simdutfBase64Proc            *syscall.Proc
	simdutfValidateUTF8FastAddr  uintptr
	simdutfUTF8ToUTF16LEFastAddr uintptr
	simdutfUTF16LEToUTF8FastAddr uintptr
	simdutfBase64DecodeFastAddr  uintptr
	simdutfBase64EncodeFastAddr  uintptr
)

func ensureSIMDUTFProcs() {
	simdutfProcsOnce.Do(func() {
		simdutfValidateProc = proc("gov8_simdutf_validate")
		simdutfConvertProc = proc("gov8_simdutf_convert")
		simdutfMeasureProc = proc("gov8_simdutf_measure")
		simdutfBase64Proc = proc("gov8_simdutf_base64")
		simdutfValidateUTF8FastAddr = proc("gov8_simdutf_validate_utf8_fast").Addr()
		simdutfUTF8ToUTF16LEFastAddr = proc("gov8_simdutf_utf8_to_utf16le_fast").Addr()
		simdutfUTF16LEToUTF8FastAddr = proc("gov8_simdutf_utf16le_to_utf8_fast").Addr()
		simdutfBase64DecodeFastAddr = proc("gov8_simdutf_base64_decode_fast").Addr()
		simdutfBase64EncodeFastAddr = proc("gov8_simdutf_base64_encode_fast").Addr()
	})
}

// The SIMDUTF-prefixed API maps rusty_v8's simdutf namespace into the gov8
// package. Rust marks conversion destinations and valid-input fast paths
// unsafe. Go instead checks every documented worst-case destination size and
// validates the fast-path input. Destination sizes are checked either in Go
// or by the shim before it invokes an unsafe simdutf primitive. SIMDUTFResult
// continues to represent simdutf data errors; the
// separate Go error reports wrapper/precondition failures.

// SIMDUTFErrorCode is a pinned simdutf error category.
type SIMDUTFErrorCode int32

const (
	SIMDUTFSuccess                SIMDUTFErrorCode = 0
	SIMDUTFHeaderBits             SIMDUTFErrorCode = 1
	SIMDUTFTooShort               SIMDUTFErrorCode = 2
	SIMDUTFTooLong                SIMDUTFErrorCode = 3
	SIMDUTFOverlong               SIMDUTFErrorCode = 4
	SIMDUTFTooLarge               SIMDUTFErrorCode = 5
	SIMDUTFSurrogate              SIMDUTFErrorCode = 6
	SIMDUTFInvalidBase64Character SIMDUTFErrorCode = 7
	SIMDUTFBase64InputRemainder   SIMDUTFErrorCode = 8
	SIMDUTFBase64ExtraBits        SIMDUTFErrorCode = 9
	SIMDUTFOutputBufferTooSmall   SIMDUTFErrorCode = 10
	SIMDUTFOther                  SIMDUTFErrorCode = 11
)

func (c SIMDUTFErrorCode) String() string {
	names := [...]string{"Success", "HeaderBits", "TooShort", "TooLong", "Overlong", "TooLarge", "Surrogate", "InvalidBase64Character", "Base64InputRemainder", "Base64ExtraBits", "OutputBufferTooSmall", "Other"}
	if c < SIMDUTFSuccess || c > SIMDUTFOther {
		return "Other"
	}
	return names[c]
}

// SIMDUTFResult reports either the number of units processed/written or the
// input position at which conversion failed.
type SIMDUTFResult struct {
	Error SIMDUTFErrorCode
	Count int
}

func (r SIMDUTFResult) OK() bool { return r.Error == SIMDUTFSuccess }

func simdutfErrorCode(code int32) SIMDUTFErrorCode {
	if code < int32(SIMDUTFSuccess) || code > int32(SIMDUTFOther) {
		return SIMDUTFOther
	}
	return SIMDUTFErrorCode(code)
}

// SIMDUTFEncoding is a bitmask returned by SIMDUTFDetectEncodings.
type SIMDUTFEncoding int32

const (
	SIMDUTFEncodingUTF8    SIMDUTFEncoding = 1
	SIMDUTFEncodingUTF16LE SIMDUTFEncoding = 2
	SIMDUTFEncodingUTF16BE SIMDUTFEncoding = 4
	SIMDUTFEncodingUTF32LE SIMDUTFEncoding = 8
	SIMDUTFEncodingUTF32BE SIMDUTFEncoding = 16
	SIMDUTFEncodingLatin1  SIMDUTFEncoding = 32
)

// SIMDUTFBase64Options selects alphabet and padding behavior.
type SIMDUTFBase64Options uint64

const (
	SIMDUTFBase64Default SIMDUTFBase64Options = iota
	SIMDUTFBase64URL
	SIMDUTFBase64DefaultNoPadding
	SIMDUTFBase64URLWithPadding
)

// SIMDUTFLastChunkHandling controls incomplete final base64 groups.
type SIMDUTFLastChunkHandling uint64

const (
	SIMDUTFLastChunkLoose SIMDUTFLastChunkHandling = iota
	SIMDUTFLastChunkStrict
	SIMDUTFLastChunkStopBeforePartial
	SIMDUTFLastChunkOnlyFullChunks
)

func slicePointer[T any](values []T) uintptr {
	if len(values) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&values[0]))
}

// sliceUnsafePointer preserves pointer provenance until the syscall expression
// converts it to uintptr. The fixed-arity Windows syscall helpers keep those
// converted pointers alive without allowing a stack copy during the call.
func sliceUnsafePointer[T any](values []T) unsafe.Pointer {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Pointer(&values[0])
}

func simdutfValidate(kind int32, input uintptr, length int, withErrors bool) (bool, SIMDUTFResult, error) {
	ensureSIMDUTFProcs()
	var valid, code int32
	var count uintptr
	with := uintptr(0)
	if withErrors {
		with = 1
	}
	r1, _, _ := simdutfValidateProc.Call(uintptr(kind), input, uintptr(length), with,
		uintptr(unsafe.Pointer(&valid)), uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&count)))
	if int64(r1) < 0 {
		return false, SIMDUTFResult{}, shimError("SIMDUTF.Validate", r1)
	}
	return valid != 0, SIMDUTFResult{Error: simdutfErrorCode(code), Count: int(count)}, nil
}

func SIMDUTFValidateUTF8(input []byte) (bool, error) {
	ensureSIMDUTFProcs()
	r1, _, _ := syscall.Syscall(simdutfValidateUTF8FastAddr, 2,
		uintptr(sliceUnsafePointer(input)), uintptr(len(input)), 0)
	runtime.KeepAlive(input)
	if int64(r1) < 0 {
		return false, shimError("SIMDUTF.ValidateUTF8", r1)
	}
	return r1 != 0, nil
}
func SIMDUTFValidateUTF8WithErrors(input []byte) (SIMDUTFResult, error) {
	_, r, e := simdutfValidate(0, slicePointer(input), len(input), true)
	runtime.KeepAlive(input)
	return r, e
}
func SIMDUTFValidateASCII(input []byte) (bool, error) {
	v, _, e := simdutfValidate(1, slicePointer(input), len(input), false)
	runtime.KeepAlive(input)
	return v, e
}
func SIMDUTFValidateASCIIWithErrors(input []byte) (SIMDUTFResult, error) {
	_, r, e := simdutfValidate(1, slicePointer(input), len(input), true)
	runtime.KeepAlive(input)
	return r, e
}
func SIMDUTFValidateUTF16LE(input []uint16) (bool, error) {
	v, _, e := simdutfValidate(2, slicePointer(input), len(input), false)
	runtime.KeepAlive(input)
	return v, e
}
func SIMDUTFValidateUTF16LEWithErrors(input []uint16) (SIMDUTFResult, error) {
	_, r, e := simdutfValidate(2, slicePointer(input), len(input), true)
	runtime.KeepAlive(input)
	return r, e
}
func SIMDUTFValidateUTF16BE(input []uint16) (bool, error) {
	v, _, e := simdutfValidate(3, slicePointer(input), len(input), false)
	runtime.KeepAlive(input)
	return v, e
}
func SIMDUTFValidateUTF16BEWithErrors(input []uint16) (SIMDUTFResult, error) {
	_, r, e := simdutfValidate(3, slicePointer(input), len(input), true)
	runtime.KeepAlive(input)
	return r, e
}
func SIMDUTFValidateUTF32(input []uint32) (bool, error) {
	v, _, e := simdutfValidate(4, slicePointer(input), len(input), false)
	runtime.KeepAlive(input)
	return v, e
}
func SIMDUTFValidateUTF32WithErrors(input []uint32) (SIMDUTFResult, error) {
	_, r, e := simdutfValidate(4, slicePointer(input), len(input), true)
	runtime.KeepAlive(input)
	return r, e
}

func simdutfCapacity(outputLength, inputLength, multiplier int) error {
	if inputLength > int(^uint(0)>>1)/multiplier {
		return errors.New("gov8: simdutf destination size overflows int")
	}
	if outputLength < inputLength*multiplier {
		return fmt.Errorf("gov8: simdutf destination capacity %d, need %d", outputLength, inputLength*multiplier)
	}
	return nil
}

func simdutfConvert(kind int32, input uintptr, inputLength int, output uintptr, outputLength int) (SIMDUTFResult, error) {
	ensureSIMDUTFProcs()
	var code int32
	var count uintptr
	r1, _, _ := simdutfConvertProc.Call(uintptr(kind), input, uintptr(inputLength), output, uintptr(outputLength),
		uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&count)))
	if int64(r1) < 0 {
		return SIMDUTFResult{}, shimError("SIMDUTF.Convert", r1)
	}
	return SIMDUTFResult{Error: simdutfErrorCode(code), Count: int(count)}, nil
}

func simdutfConvertSlices[I, O any](kind int32, input []I, output []O, multiplier int) (SIMDUTFResult, error) {
	if err := simdutfCapacity(len(output), len(input), multiplier); err != nil {
		return SIMDUTFResult{}, err
	}
	r, err := simdutfConvert(kind, slicePointer(input), len(input), slicePointer(output), len(output))
	runtime.KeepAlive(input)
	runtime.KeepAlive(output)
	return r, err
}

func SIMDUTFConvertUTF8ToUTF16LE(input []byte, output []uint16) (int, error) {
	if err := simdutfCapacity(len(output), len(input), 1); err != nil {
		return 0, err
	}
	ensureSIMDUTFProcs()
	r1, _, _ := syscall.Syscall6(simdutfUTF8ToUTF16LEFastAddr, 4,
		uintptr(sliceUnsafePointer(input)), uintptr(len(input)),
		uintptr(sliceUnsafePointer(output)), uintptr(len(output)), 0, 0)
	runtime.KeepAlive(input)
	runtime.KeepAlive(output)
	if int64(r1) < 0 {
		return 0, shimError("SIMDUTF.ConvertUTF8ToUTF16LE", r1)
	}
	return int(r1), nil
}
func SIMDUTFConvertUTF8ToUTF16LEWithErrors(input []byte, output []uint16) (SIMDUTFResult, error) {
	return simdutfConvertSlices(1, input, output, 1)
}
func SIMDUTFConvertValidUTF8ToUTF16LE(input []byte, output []uint16) (int, error) {
	valid, e := SIMDUTFValidateUTF8(input)
	if e != nil {
		return 0, e
	}
	if !valid {
		return 0, errors.New("gov8: valid UTF-8 conversion received malformed input")
	}
	r, e := simdutfConvertSlices(2, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertUTF16LEToUTF8(input []uint16, output []byte) (int, error) {
	if err := simdutfCapacity(len(output), len(input), 3); err != nil {
		return 0, err
	}
	ensureSIMDUTFProcs()
	r1, _, _ := syscall.Syscall6(simdutfUTF16LEToUTF8FastAddr, 4,
		uintptr(sliceUnsafePointer(input)), uintptr(len(input)),
		uintptr(sliceUnsafePointer(output)), uintptr(len(output)), 0, 0)
	runtime.KeepAlive(input)
	runtime.KeepAlive(output)
	if int64(r1) < 0 {
		return 0, shimError("SIMDUTF.ConvertUTF16LEToUTF8", r1)
	}
	return int(r1), nil
}
func SIMDUTFConvertUTF16LEToUTF8WithErrors(input []uint16, output []byte) (SIMDUTFResult, error) {
	return simdutfConvertSlices(4, input, output, 3)
}
func SIMDUTFConvertValidUTF16LEToUTF8(input []uint16, output []byte) (int, error) {
	valid, e := SIMDUTFValidateUTF16LE(input)
	if e != nil {
		return 0, e
	}
	if !valid {
		return 0, errors.New("gov8: valid UTF-16LE conversion received malformed input")
	}
	r, e := simdutfConvertSlices(5, input, output, 3)
	return r.Count, e
}
func SIMDUTFConvertUTF8ToUTF16BE(input []byte, output []uint16) (int, error) {
	r, e := simdutfConvertSlices(6, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertUTF16BEToUTF8(input []uint16, output []byte) (int, error) {
	r, e := simdutfConvertSlices(7, input, output, 3)
	return r.Count, e
}
func SIMDUTFConvertUTF8ToLatin1(input, output []byte) (int, error) {
	r, e := simdutfConvertSlices(8, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertUTF8ToLatin1WithErrors(input, output []byte) (SIMDUTFResult, error) {
	return simdutfConvertSlices(9, input, output, 1)
}
func SIMDUTFConvertValidUTF8ToLatin1(input, output []byte) (int, error) {
	remaining := input
	for len(remaining) > 0 {
		value, size := utf8.DecodeRune(remaining)
		if value == utf8.RuneError && size == 1 || value > 0xff {
			return 0, errors.New("gov8: valid Latin-1 conversion received unsupported UTF-8 input")
		}
		remaining = remaining[size:]
	}
	r, e := simdutfConvertSlices(10, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertLatin1ToUTF8(input, output []byte) (int, error) {
	r, e := simdutfConvertSlices(11, input, output, 2)
	return r.Count, e
}
func SIMDUTFConvertLatin1ToUTF16LE(input []byte, output []uint16) (int, error) {
	r, e := simdutfConvertSlices(12, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertUTF16LEToLatin1(input []uint16, output []byte) (int, error) {
	for _, v := range input {
		if v > 0xff {
			return 0, errors.New("gov8: Latin-1 conversion received an out-of-range UTF-16 unit")
		}
	}
	r, e := simdutfConvertSlices(13, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertUTF8ToUTF32(input []byte, output []uint32) (int, error) {
	r, e := simdutfConvertSlices(14, input, output, 1)
	return r.Count, e
}
func SIMDUTFConvertUTF32ToUTF8(input []uint32, output []byte) (int, error) {
	r, e := simdutfConvertSlices(15, input, output, 4)
	return r.Count, e
}

func simdutfMeasure(kind int32, input uintptr, length int) (int, error) {
	ensureSIMDUTFProcs()
	var out uintptr
	r1, _, _ := simdutfMeasureProc.Call(uintptr(kind), input, uintptr(length), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("SIMDUTF.Measure", r1)
	}
	return int(out), nil
}
func simdutfMeasureSlice[T any](kind int32, input []T) (int, error) {
	n, e := simdutfMeasure(kind, slicePointer(input), len(input))
	runtime.KeepAlive(input)
	return n, e
}
func SIMDUTFUTF8LengthFromUTF16LE(v []uint16) (int, error)  { return simdutfMeasureSlice(0, v) }
func SIMDUTFUTF8LengthFromUTF16BE(v []uint16) (int, error)  { return simdutfMeasureSlice(1, v) }
func SIMDUTFUTF16LengthFromUTF8(v []byte) (int, error)      { return simdutfMeasureSlice(2, v) }
func SIMDUTFUTF8LengthFromLatin1(v []byte) (int, error)     { return simdutfMeasureSlice(3, v) }
func SIMDUTFLatin1LengthFromUTF8(v []byte) (int, error)     { return simdutfMeasureSlice(4, v) }
func SIMDUTFUTF32LengthFromUTF8(v []byte) (int, error)      { return simdutfMeasureSlice(5, v) }
func SIMDUTFUTF8LengthFromUTF32(v []uint32) (int, error)    { return simdutfMeasureSlice(6, v) }
func SIMDUTFUTF16LengthFromUTF32(v []uint32) (int, error)   { return simdutfMeasureSlice(7, v) }
func SIMDUTFUTF32LengthFromUTF16LE(v []uint16) (int, error) { return simdutfMeasureSlice(8, v) }
func SIMDUTFCountUTF8(v []byte) (int, error)                { return simdutfMeasureSlice(9, v) }
func SIMDUTFCountUTF16LE(v []uint16) (int, error)           { return simdutfMeasureSlice(10, v) }
func SIMDUTFCountUTF16BE(v []uint16) (int, error)           { return simdutfMeasureSlice(11, v) }
func SIMDUTFDetectEncodings(v []byte) (SIMDUTFEncoding, error) {
	n, e := simdutfMeasureSlice(12, v)
	return SIMDUTFEncoding(n), e
}

func validBase64Options(options SIMDUTFBase64Options) bool {
	return options <= SIMDUTFBase64URLWithPadding
}
func validLastChunk(last SIMDUTFLastChunkHandling) bool {
	return last <= SIMDUTFLastChunkOnlyFullChunks
}
func simdutfBase64(kind int32, input, output []byte, logicalLength int, options SIMDUTFBase64Options, last SIMDUTFLastChunkHandling) (SIMDUTFResult, error) {
	if !validBase64Options(options) || !validLastChunk(last) {
		return SIMDUTFResult{}, errors.New("gov8: invalid simdutf base64 option")
	}
	ensureSIMDUTFProcs()
	var code int32
	var count uintptr
	r1, _, _ := simdutfBase64Proc.Call(uintptr(kind), slicePointer(input), uintptr(logicalLength), slicePointer(output), uintptr(len(output)), uintptr(options), uintptr(last), uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&count)))
	runtime.KeepAlive(input)
	runtime.KeepAlive(output)
	if int64(r1) < 0 {
		return SIMDUTFResult{}, shimError("SIMDUTF.Base64", r1)
	}
	return SIMDUTFResult{Error: simdutfErrorCode(code), Count: int(count)}, nil
}
func SIMDUTFMaximalBinaryLengthFromBase64(input []byte) (int, error) {
	r, e := simdutfBase64(0, input, nil, len(input), SIMDUTFBase64Default, SIMDUTFLastChunkLoose)
	return r.Count, e
}
func SIMDUTFBase64ToBinary(input, output []byte, options SIMDUTFBase64Options, last SIMDUTFLastChunkHandling) (SIMDUTFResult, error) {
	// The shim computes the content-aware maximum and rejects a short output
	// before calling simdutf. Keeping the check in the same boundary crossing
	// avoids measuring the input twice on every decode.
	if !validBase64Options(options) || !validLastChunk(last) {
		return SIMDUTFResult{}, errors.New("gov8: invalid simdutf base64 option")
	}
	ensureSIMDUTFProcs()
	var code int32
	r1, _, _ := syscall.Syscall9(simdutfBase64DecodeFastAddr, 7,
		uintptr(sliceUnsafePointer(input)), uintptr(len(input)),
		uintptr(sliceUnsafePointer(output)), uintptr(len(output)),
		uintptr(options), uintptr(last), uintptr(unsafe.Pointer(&code)), 0, 0)
	runtime.KeepAlive(input)
	runtime.KeepAlive(output)
	if int64(r1) < 0 {
		return SIMDUTFResult{}, shimError("SIMDUTF.Base64", r1)
	}
	return SIMDUTFResult{Error: simdutfErrorCode(code), Count: int(r1)}, nil
}

// SIMDUTFBase64LengthFromBinary accepts uint64 so Windows amd64 callers can
// observe the full size_t boundary behavior characterized by the Rust oracle.
func SIMDUTFBase64LengthFromBinary(length uint64, options SIMDUTFBase64Options) (uint64, error) {
	if !validBase64Options(options) {
		return 0, errors.New("gov8: invalid simdutf base64 option")
	}
	ensureSIMDUTFProcs()
	var code int32
	var count uintptr
	r1, _, _ := simdutfBase64Proc.Call(2, 0, uintptr(length), 0, 0, uintptr(options),
		uintptr(SIMDUTFLastChunkLoose), uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&count)))
	if int64(r1) < 0 {
		return 0, shimError("SIMDUTF.Base64LengthFromBinary", r1)
	}
	return uint64(count), nil
}
func SIMDUTFBinaryToBase64(input, output []byte, options SIMDUTFBase64Options) (int, error) {
	// As with decoding, the shim combines the exact size check and conversion
	// so encoding crosses the DLL boundary only once.
	if !validBase64Options(options) {
		return 0, errors.New("gov8: invalid simdutf base64 option")
	}
	ensureSIMDUTFProcs()
	r1, _, _ := syscall.Syscall6(simdutfBase64EncodeFastAddr, 5,
		uintptr(sliceUnsafePointer(input)), uintptr(len(input)),
		uintptr(sliceUnsafePointer(output)), uintptr(len(output)), uintptr(options), 0)
	runtime.KeepAlive(input)
	runtime.KeepAlive(output)
	if int64(r1) < 0 {
		return 0, shimError("SIMDUTF.Base64", r1)
	}
	return int(r1), nil
}
