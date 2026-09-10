//go:build windows && amd64

package gov8

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Advanced strings, external strings and BigInt: the surface pinned by the
// advanced string/BigInt oracle slice (16 fixture checks).
//
// Mapping from the pinned crate (semantics preserved, names idiomatic):
//
//	string::latin1_to_utf8       -> Latin1ToUTF8 (safe slices)
//	String::MAX_LENGTH            -> StringMaxLength
//	String::empty                 -> (*Scope).EmptyString
//	String::new_from_utf8         -> (*Scope).NewStringFromUTF8
//	String::new_from_one_byte     -> (*Scope).NewStringFromOneByte
//	String::new_from_two_byte     -> (*Scope).NewStringFromTwoByte
//	String::concat                -> (*Scope).ConcatString
//	String::is_onebyte            -> Value.IsOneByte
//	String::contains_only_onebyte -> Value.ContainsOnlyOneByte
//	String::is_external[_onebyte/twobyte]
//	                              -> Value.IsExternalString / IsExternalOneByte /
//	                                 IsExternalTwoByte (Value.IsExternal keeps
//	                                 its v8::Value::IsExternal meaning)
//	write_v2                      -> Value.WriteTwoByte
//	write_one_byte_v2             -> Value.WriteOneByte
//	write_utf8_v2                 -> Value.WriteUTF8
//	ValueView                     -> StringView (deterministic Close)
//	new_external_onebyte_static   -> (*Scope).NewExternalOneByteStringStatic
//	create_external_onebyte_const -> CreateExternalOneByteConst +
//	                                 (*Scope).NewStringFromOneByteConst
//	new_external_onebyte          -> (*Scope).NewExternalOneByteString (owned)
//	new_external_onebyte_raw      -> (*Scope).NewExternalOneByteStringRaw
//	get_external_*_resource       -> Value.GetExternal...Resource getters
//	BigInt::new_from_words        -> (*Scope).BigIntFromWords
//	BigInt::u64_value             -> Value.BigIntUint64
//	BigInt::word_count            -> Value.BigIntWordCount
//	BigInt::to_words_array        -> Value.BigIntToWords
// Intentional deviations (documented, semantics-preserving):
//
//   - Latin1ToUTF8 replaces the Rust helper's unsafe pointer/capacity
//     preconditions with slices, rejects short output before writing, and
//     deterministically supports overlapping slices by copying the input.
//   - Ownership: the engine must never hold a Go pointer (the FFI is
//     syscall-based and a retained Go pointer can be collected or moved
//     behind the engine's back). Every byte buffer handed to an external
//     string is COPIED into shim-owned memory at creation; Go releases its
//     reference when the call returns. The four Rust ownership flavors keep
//     their observable lifetimes:
//       static:  the copy is process-lifetime (never freed), the resource
//                object is deleted by the engine after finalization;
//       const:   one shared resource with a no-op Dispose, usable from any
//                number of isolates, process-lifetime (the Rust object is a
//                'static; there is deliberately no Go Close for it);
//       owned:   the copy is freed when the engine finalizes the string;
//       raw:     the copy carries an integer registry id; when the engine
//                finalizes the string the registered Go deleter observes
//                (payload pointer, length) exactly once and the shim frees
//                the copy after the callback returns.
//   - Raw deleters receive the shim copy's address, not embedder memory:
//     the Rust raw API hands buffer ownership to the (Rust) embedder; Go
//     callbacks must be pure Go (they run inside GC / isolate teardown)
//     and cannot own engine memory, so the shim keeps ownership and the
//     callback is an exact-once notification.
//   - Writes validate offset/range/buffer bounds in the wrapper AND the
//     shim before the engine is touched. The release engine only DCHECKs
//     these ranges (String::WriteToFlat reads past the string unchecked),
//     so the Rust UB boundary (offset + buffer over the string length;
//     kNullTerminate into a zero-capacity UTF-8 buffer) is a Go error
//     here, never undefined behavior. In-range observations are unchanged.
//   - StringView is released by explicit Close (deterministic, like every
//     other resource in this module); there is no finalizer. A view must
//     be closed before its creating scope closes and before any engine
//     work that could collect or move the viewed string (the pinned V8
//     ValueView contract; the view holds a no-GC scope).
//   - Encoding values follow the C++ v8::String::Encoding enum (TwoByte=0,
//     Unknown=1, OneByte=8). The Rust Encoding enum maps TwoByte to 0x2,
//     which the engine never produces; matching the C++ values avoids that
//     invalid upstream enum behavior.

// NewStringType mirrors v8::NewStringType.
type NewStringType uint8

const (
	// StringNormal creates a new string with fresh storage.
	StringNormal NewStringType = iota
	// StringInternalized hints old-generation allocation and deduplication
	// of identical strings ("Aside from performance implications there are
	// no differences between the two creation modes" - v8-primitive.h).
	StringInternalized
)

// WriteFlags mirror v8::String::WriteFlags.
type WriteFlags int32

const (
	// WriteNullTerminate appends a NUL terminator. The buffer must have
	// space for it (validated before the engine call).
	WriteNullTerminate WriteFlags = 1 << iota
	// WriteReplaceInvalidUTF8 makes WriteUTF8 emit U+FFFD for lone
	// surrogate code units instead of their raw 3-byte CESU-8 encoding.
	WriteReplaceInvalidUTF8
)

// StringEncoding mirrors the C++ v8::String::Encoding values reported by
// GetExternalStringResourceBase.
type StringEncoding int32

const (
	StringEncodingTwoByte StringEncoding = 0x0
	StringEncodingUnknown StringEncoding = 0x1
	StringEncodingOneByte StringEncoding = 0x8
)

// Latin1ToUTF8 converts Latin-1 bytes to UTF-8 in output and returns the
// number of bytes written. The pinned Rust helper requires output to provide
// the worst-case capacity of two bytes per input byte; this safe Go shape
// validates that precondition and leaves output untouched on failure.
func Latin1ToUTF8(input, output []byte) (int, error) {
	if len(input) > math.MaxInt/2 {
		return 0, fmt.Errorf("gov8: Latin1ToUTF8: input too large")
	}
	required := len(input) * 2
	if len(output) < required {
		return 0, fmt.Errorf("gov8: Latin1ToUTF8: output capacity %d is less than required %d", len(output), required)
	}

	// Copy first when the slices overlap. Forward expansion can otherwise
	// overwrite unread input bytes; the Rust pointer API makes non-overlap a
	// caller safety obligation, while the safe Go API handles it deterministically.
	if slicesOverlap(input, output[:required]) {
		input = append([]byte(nil), input...)
	}

	written := 0
	for _, c := range input {
		if c < 0x80 {
			output[written] = c
			written++
			continue
		}
		output[written] = 0xc0 | c>>6
		output[written+1] = 0x80 | c&0x3f
		written += 2
	}
	return written, nil
}

func slicesOverlap(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	a0 := uintptr(unsafe.Pointer(unsafe.SliceData(a)))
	b0 := uintptr(unsafe.Pointer(unsafe.SliceData(b)))
	return a0 < b0+uintptr(len(b)) && b0 < a0+uintptr(len(a))
}

// requireBigInt validates that v is a live BigInt local. Rust's typed Local
// prevents this misuse at compile time; Go validates before any BigInt FFI.
func (v Value) requireBigInt() error {
	if _, err := typedCast(v, v.IsBigInt, "BigInt"); err != nil {
		return err
	}
	return nil
}

// StringMaxLength returns v8::String::kMaxLength (536870888 on 64-bit
// targets): the maximum string length the engine accepts, and the bound
// every creation entry point validates.
func StringMaxLength() (int, error) {
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_sb_string_max_length").Call()
	if int64(r1) < 0 {
		return 0, shimError("StringMaxLength", r1)
	}
	return int(int64(r1)), nil
}

// --- creation -----------------------------------------------------------------

// EmptyString returns the canonical empty string (the engine's read-only
// empty-string root, not a fresh allocation).
func (s *Scope) EmptyString() (Value, error) {
	return s.constructPrimitive("EmptyString", primitiveEmptyString, 0)
}

// NewStringFromUTF8 creates a string from UTF-8 bytes. Invalid sequences
// are replaced with U+FFFD by the engine (lossy). Inputs longer than
// StringMaxLength bytes are rejected by the engine with a recoverable
// error (no exception is pending).
func (s *Scope) NewStringFromUTF8(data []byte, t NewStringType) (Value, error) {
	return s.newStringFromBuf("NewStringFromUTF8", "gov8_sb_string_new_from_utf8",
		data, t)
}

// NewStringFromOneByte creates a string from one-byte (Latin-1) bytes.
func (s *Scope) NewStringFromOneByte(data []byte, t NewStringType) (Value, error) {
	return s.newStringFromBuf("NewStringFromOneByte", "gov8_sb_string_new_from_one_byte",
		data, t)
}

// NewStringFromTwoByte creates a string from UTF-16 code units. The engine
// counts units against StringMaxLength. Latin-1-representable content
// collapses to a one-byte representation (observable via IsOneByte).
func (s *Scope) NewStringFromTwoByte(units []uint16, t NewStringType) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	if len(units) > math.MaxInt32 {
		return Value{}, fmt.Errorf("gov8: NewStringFromTwoByte: input longer than int32 range")
	}
	var p uintptr
	if len(units) > 0 {
		p = uintptr(unsafe.Pointer(&units[0]))
	}
	var out uintptr
	r1, _, _ := proc("gov8_sb_string_new_from_two_byte").Call(
		s.iso.handleAssumingCheck(), s.handle, p, uintptr(len(units)),
		uintptr(uint8(t)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("NewStringFromTwoByte", r1)
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

func (s *Scope) newStringFromBuf(op, procName string, b []byte, t NewStringType) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	if len(b) > math.MaxInt32 {
		return Value{}, fmt.Errorf("gov8: %s: input longer than int32 range", op)
	}
	var p uintptr
	if len(b) > 0 {
		p = uintptr(unsafe.Pointer(&b[0]))
	}
	var out uintptr
	r1, _, _ := proc(procName).Call(
		s.iso.handleAssumingCheck(), s.handle, p, uintptr(len(b)),
		uintptr(uint8(t)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError(op, r1)
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

// unitsToBytes reinterprets a uint16 slice as its little-endian byte
// representation without copying (x64 is little-endian; the engine reads
// UTF-16LE code units).
func unitsToBytes(units []uint16) []byte {
	if len(units) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&units[0])), len(units)*2)
}

// ConcatString concatenates two strings (String::concat). Both operands
// must be strings of the receiver's isolate. A result beyond
// StringMaxLength fails with a recoverable error.
func (s *Scope) ConcatString(left, right Value) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	if err := left.requireString(); err != nil {
		return Value{}, err
	}
	if err := right.requireString(); err != nil {
		return Value{}, err
	}
	if left.iso != s.iso {
		return Value{}, foreignIsolate("left")
	}
	if right.iso != s.iso {
		return Value{}, foreignIsolate("right")
	}
	var out uintptr
	r1, _, _ := proc("gov8_sb_string_concat").Call(
		s.iso.handleAssumingCheck(), s.handle, left.h, right.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("ConcatString", r1)
	}
	if out == 0 {
		// The engine returned an empty Local (the crate maps this to None):
		// the concatenated result would exceed StringMaxLength.
		return Value{}, fmt.Errorf("gov8: ConcatString failed (result longer than StringMaxLength?)")
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

// --- representation predicates ----------------------------------------------------

// IsOneByte reports whether the string is known (without reading it) to be
// one-byte encoded. False negatives are possible.
func (v Value) IsOneByte() (bool, error) {
	if err := v.requireString(); err != nil {
		return false, err
	}
	return v.sbPredicate("gov8_sb_string_is_one_byte")
}

// ContainsOnlyOneByte reports whether every code unit of the string fits
// one byte. It may read the entire string, so it never has false negatives.
func (v Value) ContainsOnlyOneByte() (bool, error) {
	if err := v.requireString(); err != nil {
		return false, err
	}
	return v.sbPredicate("gov8_sb_string_contains_only_one_byte")
}

// IsExternalString reports whether the string is backed by an external
// resource (of either encoding).
func (v Value) IsExternalString() (bool, error) {
	if err := v.requireString(); err != nil {
		return false, err
	}
	return v.sbPredicate("gov8_sb_string_is_external")
}

// IsExternalOneByte reports whether the string is external and one-byte.
func (v Value) IsExternalOneByte() (bool, error) {
	if err := v.requireString(); err != nil {
		return false, err
	}
	return v.sbPredicate("gov8_sb_string_is_external_onebyte")
}

// IsExternalTwoByte reports whether the string is external and two-byte.
func (v Value) IsExternalTwoByte() (bool, error) {
	if err := v.requireString(); err != nil {
		return false, err
	}
	return v.sbPredicate("gov8_sb_string_is_external_twobyte")
}

func (v Value) sbPredicate(op string) (bool, error) {
	if err := requireInitialized(); err != nil {
		return false, err
	}
	r1, _, _ := proc(op).Call(v.iso.handleAssumingCheck(), v.h)
	if int64(r1) < 0 {
		return false, shimError(op, r1)
	}
	return r1 == 1, nil
}

// --- writes -------------------------------------------------------------------------

// WriteTwoByte writes up to len(buf) UTF-16 code units of the string
// starting at code-unit offset into buf (String::Write). It returns the
// number of units written: the minimum of the remaining string length and
// the buffer capacity. offset must be within [0, Length]; a buffer too
// small for the requested range plus the NUL (when WriteNullTerminate is
// set) is an error before any engine call.
func (v Value) WriteTwoByte(offset int, buf []uint16, flags WriteFlags) (int, error) {
	n, err := v.writeRange("WriteTwoByte", offset, len(buf), flags&WriteNullTerminate != 0)
	if err != nil {
		return 0, err
	}
	b := unitsToBytes(buf)
	r1, _, _ := proc("gov8_sb_string_write").Call(
		v.iso.handleAssumingCheck(), v.h,
		uintptr(uint32(offset)), uintptr(uint32(n)),
		uintptr(unsafe.Pointer(unsafe.SliceData(b))), uintptr(len(b)),
		uintptr(int32(flags)))
	if int64(r1) < 0 {
		return 0, shimError("WriteTwoByte", r1)
	}
	return n, nil
}

// WriteOneByte writes up to len(buf) one-byte (Latin-1) characters
// starting at code-unit offset (String::WriteOneByte). Two-byte content is
// truncated to its low byte per unit. Returns the number of bytes written.
func (v Value) WriteOneByte(offset int, buf []byte, flags WriteFlags) (int, error) {
	n, err := v.writeRange("WriteOneByte", offset, len(buf), flags&WriteNullTerminate != 0)
	if err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_sb_string_write_one_byte").Call(
		v.iso.handleAssumingCheck(), v.h,
		uintptr(uint32(offset)), uintptr(uint32(n)),
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))), uintptr(len(buf)),
		uintptr(int32(flags)))
	if int64(r1) < 0 {
		return 0, shimError("WriteOneByte", r1)
	}
	return n, nil
}

// writeRange validates a unit write before the engine is touched: offset
// within the string, n = min(remaining, capacity), and capacity covering
// the NUL when requested. It returns the clamped count n.
func (v Value) writeRange(op string, offset, capacity int, nullTerminate bool) (int, error) {
	if offset < 0 {
		return 0, fmt.Errorf("gov8: %s: negative offset %d", op, offset)
	}
	if capacity < 0 {
		return 0, fmt.Errorf("gov8: %s: negative buffer capacity", op)
	}
	strLen, err := v.Length()
	if err != nil {
		return 0, err
	}
	if offset > strLen {
		return 0, fmt.Errorf("gov8: %s: offset %d beyond string length %d", op, offset, strLen)
	}
	n := strLen - offset
	if n > capacity {
		n = capacity
	}
	if nullTerminate && n+1 > capacity {
		return 0, fmt.Errorf("gov8: %s: buffer capacity %d does not hold %d units plus the NUL terminator",
			op, capacity, n)
	}
	return n, nil
}

// WriteUTF8 encodes the string as UTF-8 into buf (String::WriteUtf8). It
// never writes partial sequences: when the next character does not fit,
// encoding stops. n counts the bytes written (including the NUL when
// WriteNullTerminate is set, which requires capacity >= 1); processed
// counts the UTF-16 code units consumed.
func (v Value) WriteUTF8(buf []byte, flags WriteFlags) (n int, processed int, err error) {
	if err := v.requireString(); err != nil {
		return 0, 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, 0, err
	}
	if flags & ^(WriteNullTerminate|WriteReplaceInvalidUTF8) != 0 {
		return 0, 0, fmt.Errorf("gov8: WriteUTF8: invalid WriteFlags")
	}
	if flags&WriteNullTerminate != 0 && len(buf) < 1 {
		return 0, 0, fmt.Errorf("gov8: WriteUTF8: WriteNullTerminate requires buffer capacity >= 1")
	}
	var processedOut int64
	r1, _, _ := proc("gov8_sb_string_write_utf8").Call(
		v.iso.handleAssumingCheck(), v.h,
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))), uintptr(len(buf)),
		uintptr(int32(flags)), uintptr(unsafe.Pointer(&processedOut)))
	if int64(r1) < 0 {
		return 0, 0, shimError("WriteUTF8", r1)
	}
	return int(int64(r1)), int(processedOut), nil
}

// --- StringView -----------------------------------------------------------------------

// StringView is a direct view onto a string's contents (v8::String::
// ValueView). The view does not copy: it is only valid while no GC or
// allocation can move the viewed string, which means (a) all engine work
// waits until Close and (b) the view is closed before the scope that
// created it. Reading the contents goes through Copy, which the shim
// performs under the view's own no-GC scope in a single call, so no raw
// engine pointer ever reaches Go. Close is the only release mechanism
// (deterministic, like every resource in this module).
type StringView struct {
	iso    *Isolate
	src    Value
	handle uintptr
	closed bool
}

// NewStringView opens a view onto the string's current contents
// (flattening it if needed). v must be a string of the receiver's isolate.
func (s *Scope) NewStringView(v Value) (*StringView, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	if err := v.requireString(); err != nil {
		return nil, err
	}
	if v.iso != s.iso {
		return nil, foreignIsolate("value")
	}
	h, err := callHandle("NewStringView", proc("gov8_sb_value_view_construct"),
		s.iso.handleAssumingCheck(), v.h)
	if err != nil {
		return nil, err
	}
	return &StringView{iso: s.iso, src: v, handle: h}, nil
}

// check validates the view's state and the creating scope's liveness (the
// view's stored Local lives in that scope's slot storage).
func (sv *StringView) check() error {
	if sv.closed {
		return fmt.Errorf("gov8: string view used after Close")
	}
	if err := sv.src.sc.check(); err != nil {
		return err
	}
	if sv.handle == 0 {
		return fmt.Errorf("gov8: zero string view handle")
	}
	return nil
}

// Info reports the view's encoding and length (in code units for two-byte
// content, bytes for one-byte content).
func (sv *StringView) Info() (oneByte bool, length int, err error) {
	if err := sv.check(); err != nil {
		return false, 0, err
	}
	var ob, n int64
	r1, _, _ := proc("gov8_sb_value_view_info").Call(
		sv.handle, uintptr(unsafe.Pointer(&ob)), uintptr(unsafe.Pointer(&n)))
	if int64(r1) < 0 {
		return false, 0, shimError("StringView.Info", r1)
	}
	return ob == 1, int(n), nil
}

// Copy copies the view's raw contents into buf: bytes for one-byte
// content, little-endian UTF-16 code units for two-byte content. It
// returns the number of bytes copied. The copy happens inside one shim
// call under the view's no-GC scope.
func (sv *StringView) Copy(buf []byte) (int, error) {
	if err := sv.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_sb_value_view_copy").Call(
		sv.handle, uintptr(unsafe.Pointer(unsafe.SliceData(buf))),
		uintptr(len(buf)))
	if int64(r1) < 0 {
		return 0, shimError("StringView.Copy", r1)
	}
	return int(int64(r1)), nil
}

// Bytes returns the view's raw contents (see Copy for the encoding) and
// the encoding flag, allocating an exact-size buffer.
func (sv *StringView) Bytes() (data []byte, oneByte bool, err error) {
	oneByte, length, err := sv.Info()
	if err != nil {
		return nil, false, err
	}
	unit := 2
	if oneByte {
		unit = 1
	}
	buf := make([]byte, length*unit)
	n, err := sv.Copy(buf)
	if err != nil {
		return nil, false, err
	}
	return buf[:n], oneByte, nil
}

// Close releases the view. It must be called before the creating scope
// closes and before any further engine work on the isolate.
func (sv *StringView) Close() error {
	if sv.closed {
		return fmt.Errorf("gov8: string view already closed")
	}
	if err := sv.src.sc.check(); err != nil {
		// The scope is gone or unusable; the engine view object would
		// still leak, so report instead of marking closed.
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	r1, _, _ := proc("gov8_sb_value_view_destruct").Call(sv.handle)
	sv.closed = true
	if int64(r1) < 0 {
		return shimError("StringView.Close", r1)
	}
	return nil
}

// --- external strings -------------------------------------------------------------------

// NewExternalOneByteStringStatic creates an external one-byte string over
// a copy of data that lives for the rest of the process (static
// semantics). The resource object itself is engine-owned and released
// after finalization.
func (s *Scope) NewExternalOneByteStringStatic(data []byte) (Value, error) {
	return s.newExternalString("NewExternalOneByteStringStatic",
		"gov8_sb_external_onebyte_static_new", data)
}

// NewExternalTwoByteStringStatic is the two-byte counterpart.
func (s *Scope) NewExternalTwoByteStringStatic(units []uint16) (Value, error) {
	return s.newExternalStringUnits("NewExternalTwoByteStringStatic",
		"gov8_sb_external_twobyte_static_new", units)
}

// NewExternalOneByteString creates an external one-byte string owning a
// copy of data; the engine frees the copy when it finalizes the string.
func (s *Scope) NewExternalOneByteString(data []byte) (Value, error) {
	return s.newExternalString("NewExternalOneByteString",
		"gov8_sb_external_onebyte_owned_new", data)
}

// NewExternalTwoByteString is the two-byte counterpart.
func (s *Scope) NewExternalTwoByteString(units []uint16) (Value, error) {
	return s.newExternalStringUnits("NewExternalTwoByteString",
		"gov8_sb_external_twobyte_owned_new", units)
}

func (s *Scope) newExternalString(op, procName string, b []byte) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	var p uintptr
	if len(b) > 0 {
		p = uintptr(unsafe.Pointer(&b[0]))
	}
	h, err := callHandle(op, proc(procName), s.iso.handleAssumingCheck(),
		s.handle, p, uintptr(len(b)))
	if err != nil {
		return Value{}, err
	}
	return Value{iso: s.iso, sc: s, h: h}, nil
}

// newExternalStringUnits is the two-byte variant: the shim length argument
// counts UTF-16 code units (the shim copies len*2 bytes).
func (s *Scope) newExternalStringUnits(op, procName string, units []uint16) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	var p uintptr
	if len(units) > 0 {
		p = uintptr(unsafe.Pointer(&units[0]))
	}
	h, err := callHandle(op, proc(procName), s.iso.handleAssumingCheck(),
		s.handle, p, uintptr(len(units)))
	if err != nil {
		return Value{}, err
	}
	return Value{iso: s.iso, sc: s, h: h}, nil
}

// ExternalOneByteConst is a build-time-style external one-byte resource
// (the Go analog of the crate's OneByteConst): created once from ASCII
// data, shareable across isolates, never disposed, process-lifetime.
// There is deliberately no Close: matching the Rust &'static, the
// resource must outlive every external string created from it in any
// isolate, and V8 may finalize those long after the embedding code's
// references are gone.
type ExternalOneByteConst struct {
	data   string
	handle uintptr
}

// CreateExternalOneByteConst creates a shared const resource from ASCII
// data. The data is copied; the Go string is not retained by the engine.
func CreateExternalOneByteConst(data string) (*ExternalOneByteConst, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	for i := 0; i < len(data); i++ {
		if data[i] > 0x7F {
			return nil, fmt.Errorf("gov8: CreateExternalOneByteConst: byte %d is not ASCII", i)
		}
	}
	var b []byte
	if len(data) > 0 {
		b = []byte(data)
	}
	var p uintptr
	if len(b) > 0 {
		p = uintptr(unsafe.Pointer(&b[0]))
	}
	h, err := callHandle("CreateExternalOneByteConst",
		proc("gov8_sb_external_onebyte_const_create"), p, uintptr(len(b)))
	if err != nil {
		return nil, err
	}
	// Keep b alive until the shim's copy completed.
	runtime.KeepAlive(&b[0])
	return &ExternalOneByteConst{data: data, handle: h}, nil
}

// Data returns the resource's ASCII contents (the as_str analog).
func (r *ExternalOneByteConst) Data() string { return r.data }

// NewStringFromOneByteConst creates an external string over the shared
// const resource. Safe on any number of isolates (the resource's Dispose
// is a no-op, so one isolate's finalization cannot destroy another
// isolate's string data).
func (s *Scope) NewStringFromOneByteConst(r *ExternalOneByteConst) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	if r == nil || r.handle == 0 {
		return Value{}, fmt.Errorf("gov8: nil or invalid const resource")
	}
	h, err := callHandle("NewStringFromOneByteConst",
		proc("gov8_sb_external_onebyte_const_string_new"),
		s.iso.handleAssumingCheck(), s.handle, r.handle)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: s.iso, sc: s, h: h}, nil
}

// ExternalStringDeleter observes the release of a raw external string's
// buffer: exactly one call with the payload address and its length, at
// the first forced major GC after the last strong reference drops (or
// during isolate disposal while the string is alive). It must be pure Go:
// it runs inside engine GC / teardown, where re-entering the engine is
// not permitted. The shim frees the buffer after the callback returns;
// the callback must not retain the address.
type ExternalStringDeleter func(data uintptr, length int)

// sbDeleterRegistry maps integer ids to raw-external deleter callbacks.
// Entries self-remove on the single invocation.
var sbDeleterRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]ExternalStringDeleter
}{entries: make(map[int64]ExternalStringDeleter)}

var (
	sbDeleterOnce sync.Once
	sbDeleterErr  error
)

// goSbDeleterDispatch is the single trampoline the engine retains; created
// once via syscall.NewCallback (pinned for the process lifetime).
var goSbDeleterDispatch = syscall.NewCallback(func(id, data, length uintptr) uintptr {
	sbDeleterRegistry.mu.Lock()
	fn, ok := sbDeleterRegistry.entries[int64(id)]
	if ok {
		delete(sbDeleterRegistry.entries, int64(id))
	}
	sbDeleterRegistry.mu.Unlock()
	if ok && fn != nil {
		fn(data, int(length))
	}
	return 1
})

func installSbDeleterEntry() error {
	sbDeleterOnce.Do(func() {
		sbDeleterErr = callErr("SbDeleterEntry",
			proc("gov8_sb_external_deleter_set_entry"), goSbDeleterDispatch)
	})
	return sbDeleterErr
}

// NewExternalOneByteStringRaw creates an external one-byte string over a
// copy of data whose release is observed by deleter. It returns the
// payload address the engine will report to the deleter (the pinned
// pointer-identity observation; the address identifies shim-owned memory
// and must not be dereferenced or freed by Go).
func (s *Scope) NewExternalOneByteStringRaw(data []byte, deleter ExternalStringDeleter) (Value, uintptr, error) {
	return s.newExternalStringRaw("NewExternalOneByteStringRaw",
		"gov8_sb_external_onebyte_raw_new", data, deleter)
}

// NewExternalTwoByteStringRaw is the two-byte counterpart (length in code
// units, as reported by the engine and the deleter).
func (s *Scope) NewExternalTwoByteStringRaw(units []uint16, deleter ExternalStringDeleter) (Value, uintptr, error) {
	return s.newExternalStringRawUnits("NewExternalTwoByteStringRaw",
		"gov8_sb_external_twobyte_raw_new", units, deleter)
}

func (s *Scope) newExternalStringRaw(op, procName string, b []byte, deleter ExternalStringDeleter) (Value, uintptr, error) {
	if err := s.check(); err != nil {
		return Value{}, 0, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, 0, err
	}
	if deleter == nil {
		return Value{}, 0, fmt.Errorf("gov8: %s: nil deleter", op)
	}
	if err := installSbDeleterEntry(); err != nil {
		return Value{}, 0, err
	}
	sbDeleterRegistry.mu.Lock()
	sbDeleterRegistry.next++
	id := sbDeleterRegistry.next
	sbDeleterRegistry.entries[id] = deleter
	sbDeleterRegistry.mu.Unlock()

	var p uintptr
	if len(b) > 0 {
		p = uintptr(unsafe.Pointer(&b[0]))
	}
	var payload uintptr
	h, err := callHandle(op, proc(procName), s.iso.handleAssumingCheck(),
		s.handle, p, uintptr(len(b)), uintptr(id), uintptr(unsafe.Pointer(&payload)))
	if err != nil {
		sbDeleterRegistry.mu.Lock()
		delete(sbDeleterRegistry.entries, id)
		sbDeleterRegistry.mu.Unlock()
		return Value{}, 0, err
	}
	return Value{iso: s.iso, sc: s, h: h}, payload, nil
}

// newExternalStringRawUnits is the two-byte variant: the shim length
// argument counts UTF-16 code units (the shim copies len*2 bytes).
func (s *Scope) newExternalStringRawUnits(op, procName string, units []uint16, deleter ExternalStringDeleter) (Value, uintptr, error) {
	if err := s.check(); err != nil {
		return Value{}, 0, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, 0, err
	}
	if deleter == nil {
		return Value{}, 0, fmt.Errorf("gov8: %s: nil deleter", op)
	}
	if err := installSbDeleterEntry(); err != nil {
		return Value{}, 0, err
	}
	sbDeleterRegistry.mu.Lock()
	sbDeleterRegistry.next++
	id := sbDeleterRegistry.next
	sbDeleterRegistry.entries[id] = deleter
	sbDeleterRegistry.mu.Unlock()

	var p uintptr
	if len(units) > 0 {
		p = uintptr(unsafe.Pointer(&units[0]))
	}
	var payload uintptr
	h, err := callHandle(op, proc(procName), s.iso.handleAssumingCheck(),
		s.handle, p, uintptr(len(units)), uintptr(id), uintptr(unsafe.Pointer(&payload)))
	if err != nil {
		sbDeleterRegistry.mu.Lock()
		delete(sbDeleterRegistry.entries, id)
		sbDeleterRegistry.mu.Unlock()
		return Value{}, 0, err
	}
	return Value{iso: s.iso, sc: s, h: h}, payload, nil
}

// GetExternalOneByteStringResource resolves the one-byte external
// resource (base getter + encoding cast). ok is false for plain strings
// and two-byte externals. data echoes the resource's bytes through the
// engine's virtual accessors; resource identifies the resource object for
// pointer-identity observations and must not be dereferenced.
func (v Value) GetExternalOneByteStringResource() (resource uintptr, data []byte, ok bool, err error) {
	return v.externalResourceText("GetExternalOneByteStringResource",
		"gov8_sb_external_onebyte_resource")
}

func (v Value) externalResourceText(op, procName string) (uintptr, []byte, bool, error) {
	if err := v.requireString(); err != nil {
		return 0, nil, false, err
	}
	if err := requireInitialized(); err != nil {
		return 0, nil, false, err
	}
	var res uintptr
	var n int64
	// Size probe first (cap 0): the shim reports the required size through
	// n with the errNoMemory status.
	r1, _, _ := proc(procName).Call(v.iso.handleAssumingCheck(), v.h,
		uintptr(unsafe.Pointer(&res)), 0, 0, uintptr(unsafe.Pointer(&n)))
	if int64(r1) == errNoMemory {
		// Expected on the probe path; n now holds the required size.
	} else if int64(r1) < 0 {
		return 0, nil, false, shimError(op, r1)
	}
	if res == 0 {
		return 0, nil, false, nil
	}
	if n <= 0 {
		return res, []byte{}, true, nil
	}
	buf := make([]byte, n)
	p := uintptr(unsafe.Pointer(&buf[0]))
	r1, _, _ = proc(procName).Call(v.iso.handleAssumingCheck(), v.h,
		uintptr(unsafe.Pointer(&res)), p, uintptr(n), uintptr(unsafe.Pointer(&n)))
	if int64(r1) < 0 {
		return 0, nil, false, shimError(op, r1)
	}
	if n < 0 || int(n) > len(buf) {
		n = int64(len(buf))
	}
	return res, buf[:n], true, nil
}

// GetExternalStringResource resolves the generic (two-byte-typed)
// resource; ok is false for one-byte externals and plain strings in this
// pinned build.
func (v Value) GetExternalStringResource() (resource uintptr, ok bool, err error) {
	if err := v.requireString(); err != nil {
		return 0, false, err
	}
	if err := requireInitialized(); err != nil {
		return 0, false, err
	}
	var res uintptr
	r1, _, _ := proc("gov8_sb_external_string_resource_generic").Call(
		v.iso.handleAssumingCheck(), v.h, uintptr(unsafe.Pointer(&res)))
	if int64(r1) < 0 {
		return 0, false, shimError("GetExternalStringResource", r1)
	}
	return res, res != 0, nil
}

// GetExternalStringResourceBase resolves the resource base and reports
// the engine's Encoding of the string (valid for plain strings too, where
// resource is 0).
func (v Value) GetExternalStringResourceBase() (resource uintptr, encoding StringEncoding, ok bool, err error) {
	if err := v.requireString(); err != nil {
		return 0, 0, false, err
	}
	if err := requireInitialized(); err != nil {
		return 0, 0, false, err
	}
	var res uintptr
	var enc int32
	r1, _, _ := proc("gov8_sb_external_string_resource_base").Call(
		v.iso.handleAssumingCheck(), v.h,
		uintptr(unsafe.Pointer(&res)), uintptr(unsafe.Pointer(&enc)))
	if int64(r1) < 0 {
		return 0, 0, false, shimError("GetExternalStringResourceBase", r1)
	}
	return res, StringEncoding(enc), res != 0, nil
}

// --- BigInt ---------------------------------------------------------------------------

// BigIntFromWords builds (-1)^sign * sum(words[i] * 2^(64*i)). The context
// is required (the engine API resolves the current context for allocation
// and the over-limit throw). A word count beyond the engine's BigInt
// maximum fails with a pending JS RangeError: with tc non-nil the
// exception is observable there and the error is returned; with tc nil a
// shim-internal TryCatch observes it.
func (s *Scope) BigIntFromWords(c *Context, sign bool, words []uint64, tc *TryCatch) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if err := requireInitialized(); err != nil {
		return Value{}, err
	}
	if c == nil || c.iso != s.iso {
		return Value{}, foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return Value{}, err
	}
	tcv, err := tcArg(s.iso, tc)
	if err != nil {
		return Value{}, err
	}
	if len(words) > math.MaxInt32 {
		return Value{}, fmt.Errorf("gov8: BigIntFromWords: word count exceeds int32 range")
	}
	var p uintptr
	if len(words) > 0 {
		p = uintptr(unsafe.Pointer(&words[0]))
	}
	signv := uintptr(0)
	if sign {
		signv = 1
	}
	var out uintptr
	r1, _, _ := proc("gov8_sb_bigint_new_from_words").Call(
		s.iso.handleAssumingCheck(), c.handle, s.handle, tcv, signv, p,
		uintptr(len(words)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("BigIntFromWords", r1)
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

// BigIntUint64 returns the unsigned 64-bit view of a BigInt and whether
// the conversion was lossless (false when truncated or negative).
func (v Value) BigIntUint64() (value uint64, lossless bool, err error) {
	if err := v.requireBigInt(); err != nil {
		return 0, false, err
	}
	if err := requireInitialized(); err != nil {
		return 0, false, err
	}
	var out uint64
	var lossy int32
	r1, _, _ := proc("gov8_sb_bigint_u64_value").Call(
		v.iso.handleAssumingCheck(), v.h,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&lossy)))
	if int64(r1) < 0 {
		return 0, false, shimError("BigIntUint64", r1)
	}
	return out, lossy == 1, nil
}

// BigIntWordCount returns the number of 64-bit words the BigInt occupies.
func (v Value) BigIntWordCount() (int, error) {
	if err := v.requireBigInt(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_sb_bigint_word_count").Call(
		v.iso.handleAssumingCheck(), v.h)
	if int64(r1) < 0 {
		return 0, shimError("BigIntWordCount", r1)
	}
	return int(int64(r1)), nil
}

// BigIntToWords writes the BigInt's absolute-value words into buf (little
// endian) and reports the sign bit. The engine truncates to the buffer
// capacity; the returned subslice covers exactly the words written, and
// buffer bytes beyond it are untouched. A zero BigInt writes nothing.
func (v Value) BigIntToWords(buf []uint64) (sign bool, words []uint64, err error) {
	if err := v.requireBigInt(); err != nil {
		return false, nil, err
	}
	if err := requireInitialized(); err != nil {
		return false, nil, err
	}
	if len(buf) > math.MaxInt32 {
		return false, nil, fmt.Errorf("gov8: BigIntToWords: capacity exceeds int32 range")
	}
	var signOut, written int64
	r1, _, _ := proc("gov8_sb_bigint_to_words_array").Call(
		v.iso.handleAssumingCheck(), v.h,
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))), uintptr(len(buf)),
		uintptr(unsafe.Pointer(&signOut)), uintptr(unsafe.Pointer(&written)))
	if int64(r1) < 0 {
		return false, nil, shimError("BigIntToWords", r1)
	}
	if written < 0 || int(written) > len(buf) {
		written = int64(len(buf))
	}
	return signOut == 1, buf[:written], nil
}

// HasTerminated is provided by terminate.go (the core TryCatch already
// exposes the predicate this slice's range-error check pins to false).
