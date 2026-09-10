//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	errCRDTPClosed   = errors.New("gov8: CRDTP value is closed")
	errCRDTPConsumed = errors.New("gov8: CRDTP value was consumed")

	crdtpCoreProcsOnce           sync.Once
	crdtpBytesCopyAddr           uintptr
	crdtpBytesCopyDeleteAddr     uintptr
	crdtpSerializableViewAddr    uintptr
	crdtpSerializableDeleteAddr  uintptr
	crdtpJSONToCBORSizedAddr     uintptr
	crdtpCBORToJSONSizedAddr     uintptr
	crdtpDispatchResponseNewAddr uintptr
)

func ensureCRDTPCoreProcs() {
	crdtpCoreProcsOnce.Do(func() {
		crdtpBytesCopyAddr = proc("gov8_crdtp_bytes_copy").Addr()
		crdtpBytesCopyDeleteAddr = proc("gov8_crdtp_bytes_copy_delete").Addr()
		crdtpSerializableViewAddr = proc("gov8_crdtp_serializable_view").Addr()
		crdtpSerializableDeleteAddr = proc("gov8_crdtp_serializable_delete").Addr()
		crdtpJSONToCBORSizedAddr = proc("gov8_crdtp_json_to_cbor_sized").Addr()
		crdtpCBORToJSONSizedAddr = proc("gov8_crdtp_cbor_to_json_sized").Addr()
		crdtpDispatchResponseNewAddr = proc("gov8_crdtp_response_new").Addr()
	})
}

// CRDTPJSONToCBOR converts UTF-8 JSON to the canonical CBOR representation
// used by the Chrome DevTools protocol. ok is false for malformed input.
// The result never aliases native memory or input.
func CRDTPJSONToCBOR(input []byte) (result []byte, ok bool, err error) {
	return crdtpConvert("CRDTPJSONToCBOR", "gov8_crdtp_json_to_cbor", input)
}

// CRDTPCBORToJSON converts CRDTP CBOR to UTF-8 JSON. ok is false for malformed
// input. The result never aliases native memory or input.
func CRDTPCBORToJSON(input []byte) (result []byte, ok bool, err error) {
	return crdtpConvert("CRDTPCBORToJSON", "gov8_crdtp_cbor_to_json", input)
}

func crdtpConvert(op, export string, input []byte) ([]byte, bool, error) {
	if err := loadShim(); err != nil {
		return nil, false, err
	}
	ensureCRDTPCoreProcs()
	convertAddr := crdtpJSONToCBORSizedAddr
	if export == "gov8_crdtp_cbor_to_json" {
		convertAddr = crdtpCBORToJSONSizedAddr
	}
	var output [2]uintptr
	status, _, _ := syscall.Syscall6(convertAddr, 4,
		uintptr(sliceUnsafePointer(input)), uintptr(len(input)), uintptr(unsafe.Pointer(&output[0])),
		uintptr(unsafe.Pointer(&output[1])), 0, 0)
	runtime.KeepAlive(input)
	runtime.KeepAlive(&output)
	if int64(status) < 0 {
		return nil, false, shimError(op, status)
	}
	if status == 0 {
		return nil, false, nil
	}
	if status != 1 || output[0] == 0 {
		return nil, false, fmt.Errorf("gov8: %s returned invalid status %d", op, status)
	}
	bytes, err := takeCRDTPBytesSized(output[0], output[1])
	return bytes, err == nil, err
}

func takeCRDTPBytes(handle uintptr) (result []byte, err error) {
	if handle == 0 {
		return nil, errors.New("gov8: null CRDTP byte buffer")
	}
	ensureCRDTPCoreProcs()
	owned := handle
	defer func() {
		if owned != 0 {
			closeErr := callErr("CRDTPBytes.Close", proc("gov8_crdtp_bytes_delete"), owned)
			if closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	length, _, _ := syscall.Syscall(crdtpBytesCopyAddr, 3, handle, 0, 0)
	if int64(length) < 0 {
		return nil, shimError("CRDTPBytes.Len", length)
	}
	maxInt := uintptr(^uint(0) >> 1)
	if length > maxInt {
		return nil, errors.New("gov8: CRDTP byte length exceeds max int")
	}
	owned = 0 // takeCRDTPBytesSized assumes ownership, including error cleanup.
	return takeCRDTPBytesSized(handle, length)
}

func takeCRDTPBytesSized(handle, length uintptr) (result []byte, err error) {
	if handle == 0 {
		return nil, errors.New("gov8: null CRDTP byte buffer")
	}
	owned := handle
	defer func() {
		if owned != 0 {
			closeErr := callErr("CRDTPBytes.Close", proc("gov8_crdtp_bytes_delete"), owned)
			if closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	maxInt := uintptr(^uint(0) >> 1)
	if length > maxInt {
		return nil, errors.New("gov8: CRDTP byte length exceeds max int")
	}
	result = make([]byte, int(length))
	status, _, _ := syscall.Syscall(crdtpBytesCopyDeleteAddr, 3,
		handle, uintptr(sliceUnsafePointer(result)), uintptr(len(result)))
	runtime.KeepAlive(result)
	if int64(status) < 0 {
		return nil, shimError("CRDTPBytes.Copy", status)
	}
	if status != length {
		return nil, errors.New("gov8: CRDTP byte length changed while copying")
	}
	owned = 0
	return result, nil
}

// CRDTPDispatchable is an owned parsed protocol message. Construction copies
// both inputs, so they may be modified or released immediately afterward.
// A malformed message still produces a value whose OK method reports false;
// accessors reject such a value instead of invoking upstream preconditioned
// accessors.
type CRDTPDispatchable struct {
	mu     sync.Mutex
	handle uintptr
	closed bool
	active bool
}

func NewCRDTPDispatchable(cbor, associatedData []byte) (*CRDTPDispatchable, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_dispatchable_new").Call(
		slicePointer(cbor), uintptr(len(cbor)), slicePointer(associatedData),
		uintptr(len(associatedData)), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(cbor)
	runtime.KeepAlive(associatedData)
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return nil, shimError("NewCRDTPDispatchable", status)
	}
	if out == 0 {
		return nil, errors.New("gov8: NewCRDTPDispatchable returned null")
	}
	return &CRDTPDispatchable{handle: out}, nil
}

func (d *CRDTPDispatchable) withHandle() (uintptr, error) {
	if d == nil {
		return 0, errors.New("gov8: nil CRDTPDispatchable")
	}
	if d.closed || d.handle == 0 {
		return 0, errCRDTPClosed
	}
	if d.active {
		return 0, errors.New("gov8: CRDTP Dispatchable is active")
	}
	return d.handle, nil
}

func (d *CRDTPDispatchable) OK() (bool, error) {
	if d == nil {
		return false, errors.New("gov8: nil CRDTPDispatchable")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	handle, err := d.withHandle()
	if err != nil {
		return false, err
	}
	var out int32
	status, _, _ := proc("gov8_crdtp_dispatchable_ok").Call(
		handle, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return false, shimError("CRDTPDispatchable.OK", status)
	}
	if out != 0 && out != 1 {
		return false, fmt.Errorf("gov8: invalid CRDTP Dispatchable OK value %d", out)
	}
	return out == 1, nil
}

func (d *CRDTPDispatchable) CallID() (id int32, has bool, err error) {
	if d == nil {
		return 0, false, errors.New("gov8: nil CRDTPDispatchable")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	handle, err := d.withHandle()
	if err != nil {
		return 0, false, err
	}
	var hasValue, value int32
	status, _, _ := proc("gov8_crdtp_dispatchable_call_id").Call(
		handle, uintptr(unsafe.Pointer(&hasValue)), uintptr(unsafe.Pointer(&value)))
	runtime.KeepAlive(&hasValue)
	runtime.KeepAlive(&value)
	if int64(status) < 0 {
		return 0, false, shimError("CRDTPDispatchable.CallID", status)
	}
	if hasValue != 0 && hasValue != 1 {
		return 0, false, fmt.Errorf("gov8: invalid CRDTP has-call-ID value %d", hasValue)
	}
	return value, hasValue == 1, nil
}

func (d *CRDTPDispatchable) Method() ([]byte, error)         { return d.bytes(0) }
func (d *CRDTPDispatchable) SessionID() ([]byte, error)      { return d.bytes(1) }
func (d *CRDTPDispatchable) Params() ([]byte, error)         { return d.bytes(2) }
func (d *CRDTPDispatchable) AssociatedData() ([]byte, error) { return d.bytes(3) }

func (d *CRDTPDispatchable) MethodString() (string, error) {
	method, err := d.Method()
	return string(method), err
}

func (d *CRDTPDispatchable) bytes(kind uintptr) ([]byte, error) {
	if d == nil {
		return nil, errors.New("gov8: nil CRDTPDispatchable")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	handle, err := d.withHandle()
	if err != nil {
		return nil, err
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_dispatchable_bytes").Call(
		handle, kind, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return nil, shimError("CRDTPDispatchable.Accessor", status)
	}
	return takeCRDTPBytes(out)
}

func (d *CRDTPDispatchable) Close() error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	if d.active {
		d.mu.Unlock()
		return errors.New("gov8: CRDTP Dispatchable is active")
	}
	d.closed = true
	handle := d.handle
	d.handle = 0
	d.mu.Unlock()
	if handle == 0 {
		return nil
	}
	return callErr("CRDTPDispatchable.Close", proc("gov8_crdtp_dispatchable_delete"), handle)
}

// CRDTPDispatchResponse is an owned protocol dispatch result. Error-response
// and error-notification artifacts consume it exactly once.
type CRDTPDispatchResponse struct {
	mu       sync.Mutex
	handle   uintptr
	closed   bool
	consumed bool
}

type crdtpResponseKind int32

const (
	crdtpSuccess crdtpResponseKind = iota
	crdtpFallThrough
	crdtpParseError
	crdtpInvalidRequest
	crdtpMethodNotFound
	crdtpInvalidParams
	crdtpServerError
)

func NewCRDTPSuccessResponse() (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpSuccess, "")
}
func NewCRDTPFallThroughResponse() (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpFallThrough, "")
}
func NewCRDTPParseError(message string) (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpParseError, message)
}
func NewCRDTPInvalidRequest(message string) (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpInvalidRequest, message)
}
func NewCRDTPMethodNotFound(message string) (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpMethodNotFound, message)
}
func NewCRDTPInvalidParams(message string) (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpInvalidParams, message)
}
func NewCRDTPServerError(message string) (*CRDTPDispatchResponse, error) {
	return newCRDTPDispatchResponse(crdtpServerError, message)
}

func newCRDTPDispatchResponse(kind crdtpResponseKind, message string) (*CRDTPDispatchResponse, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	ensureCRDTPCoreProcs()
	var messageData unsafe.Pointer
	if len(message) != 0 {
		messageData = unsafe.Pointer(unsafe.StringData(message))
	}
	var out uintptr
	status, _, _ := syscall.Syscall6(crdtpDispatchResponseNewAddr, 4,
		uintptr(kind), uintptr(messageData), uintptr(len(message)), uintptr(unsafe.Pointer(&out)), 0, 0)
	runtime.KeepAlive(message)
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return nil, shimError("NewCRDTPDispatchResponse", status)
	}
	if out == 0 {
		return nil, errors.New("gov8: NewCRDTPDispatchResponse returned null")
	}
	return &CRDTPDispatchResponse{handle: out}, nil
}

func (r *CRDTPDispatchResponse) withHandle() (uintptr, error) {
	if r == nil {
		return 0, errors.New("gov8: nil CRDTPDispatchResponse")
	}
	if r.consumed {
		return 0, errCRDTPConsumed
	}
	if r.closed || r.handle == 0 {
		return 0, errCRDTPClosed
	}
	return r.handle, nil
}

func (r *CRDTPDispatchResponse) query(kind uintptr) (int32, error) {
	if r == nil {
		return 0, errors.New("gov8: nil CRDTPDispatchResponse")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	handle, err := r.withHandle()
	if err != nil {
		return 0, err
	}
	var out int32
	status, _, _ := proc("gov8_crdtp_response_query").Call(
		handle, kind, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return 0, shimError("CRDTPDispatchResponse.Query", status)
	}
	return out, nil
}

func (r *CRDTPDispatchResponse) IsSuccess() (bool, error) {
	v, err := r.query(0)
	return v == 1, err
}
func (r *CRDTPDispatchResponse) IsError() (bool, error) {
	v, err := r.query(1)
	return v == 1, err
}
func (r *CRDTPDispatchResponse) IsFallThrough() (bool, error) {
	v, err := r.query(2)
	return v == 1, err
}
func (r *CRDTPDispatchResponse) Code() (int32, error) { return r.query(3) }

func (r *CRDTPDispatchResponse) Message() (string, error) {
	if r == nil {
		return "", errors.New("gov8: nil CRDTPDispatchResponse")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	handle, err := r.withHandle()
	if err != nil {
		return "", err
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_response_message").Call(
		handle, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return "", shimError("CRDTPDispatchResponse.Message", status)
	}
	bytes, err := takeCRDTPBytes(out)
	return string(bytes), err
}

func (r *CRDTPDispatchResponse) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.consumed {
		return nil
	}
	r.closed = true
	handle := r.handle
	r.handle = 0
	if handle == 0 {
		return nil
	}
	return callErr("CRDTPDispatchResponse.Close", proc("gov8_crdtp_response_delete"), handle)
}

// CRDTPSerializable is an owned lazily serialized protocol artifact. Bytes
// returns a fresh copy on every call. Passing it as params consumes it once.
type CRDTPSerializable struct {
	mu              sync.Mutex
	handle          uintptr
	callbackView    uintptr
	callbackViewLen uintptr
	closed          bool
	consumed        bool
}

func (s *CRDTPSerializable) withHandle() (uintptr, error) {
	if s == nil {
		return 0, errors.New("gov8: nil CRDTPSerializable")
	}
	if s.consumed {
		return 0, errCRDTPConsumed
	}
	if s.closed || s.handle == 0 {
		return 0, errCRDTPClosed
	}
	return s.handle, nil
}

func (s *CRDTPSerializable) Bytes() ([]byte, error) {
	if s == nil {
		return nil, errors.New("gov8: nil CRDTPSerializable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	handle, err := s.withHandle()
	if err != nil {
		return nil, err
	}
	if s.callbackView != 0 {
		maxInt := uintptr(^uint(0) >> 1)
		if s.callbackViewLen > maxInt {
			return nil, errors.New("gov8: CRDTP Serializable length exceeds max int")
		}
		result := make([]byte, int(s.callbackViewLen))
		if s.callbackViewLen != 0 {
			copy(result, unsafe.Slice((*byte)(abiWordToPtr(s.callbackView)), int(s.callbackViewLen)))
		}
		runtime.KeepAlive(s)
		return result, nil
	}
	ensureCRDTPCoreProcs()
	var view [2]uintptr
	status, _, _ := syscall.Syscall(crdtpSerializableViewAddr, 3,
		handle, uintptr(unsafe.Pointer(&view[0])), uintptr(unsafe.Pointer(&view[1])))
	runtime.KeepAlive(&view)
	if int64(status) < 0 {
		return nil, shimError("CRDTPSerializable.Bytes", status)
	}
	if status != 0 {
		return nil, fmt.Errorf("gov8: CRDTP Serializable returned invalid status %d", status)
	}
	maxInt := uintptr(^uint(0) >> 1)
	if view[1] > maxInt {
		return nil, errors.New("gov8: CRDTP Serializable length exceeds max int")
	}
	if view[0] == 0 {
		return nil, errors.New("gov8: CRDTP Serializable returned a null byte view")
	}
	result := make([]byte, int(view[1]))
	if view[1] != 0 {
		copy(result, unsafe.Slice((*byte)(abiWordToPtr(view[0])), int(view[1])))
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(s)
	return result, nil
}

func (s *CRDTPSerializable) clearCallbackView() {
	s.mu.Lock()
	s.callbackView = 0
	s.callbackViewLen = 0
	s.mu.Unlock()
}

func (s *CRDTPSerializable) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.consumed {
		return nil
	}
	s.closed = true
	handle := s.handle
	s.handle = 0
	if handle == 0 {
		return nil
	}
	ensureCRDTPCoreProcs()
	status, _, _ := syscall.Syscall(crdtpSerializableDeleteAddr, 1, handle, 0, 0)
	if int64(status) < 0 {
		return shimError("CRDTPSerializable.Close", status)
	}
	return nil
}

func CreateCRDTPErrorResponse(callID int32, response *CRDTPDispatchResponse) (*CRDTPSerializable, error) {
	return consumeCRDTPResponse("CreateCRDTPErrorResponse", "gov8_crdtp_create_error_response", callID, response)
}

func CreateCRDTPErrorNotification(response *CRDTPDispatchResponse) (*CRDTPSerializable, error) {
	return consumeCRDTPResponse("CreateCRDTPErrorNotification", "gov8_crdtp_create_error_notification", 0, response)
}

func consumeCRDTPResponse(op, export string, callID int32, response *CRDTPDispatchResponse) (*CRDTPSerializable, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("gov8: nil CRDTPDispatchResponse")
	}
	response.mu.Lock()
	defer response.mu.Unlock()
	handle, err := response.withHandle()
	if err != nil {
		return nil, err
	}
	var out uintptr
	var consumed int32
	var status uintptr
	if export == "gov8_crdtp_create_error_response" {
		status, _, _ = proc(export).Call(uintptr(callID), handle,
			uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&consumed)))
	} else {
		status, _, _ = proc(export).Call(handle,
			uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&consumed)))
	}
	if consumed == 1 {
		response.consumed = true
		response.handle = 0
	} else if consumed != 0 {
		return nil, fmt.Errorf("gov8: %s returned invalid consumed state %d", op, consumed)
	}
	if int64(status) < 0 {
		return nil, shimError(op, status)
	}
	if consumed != 1 || out == 0 {
		return nil, fmt.Errorf("gov8: %s returned invalid ownership result", op)
	}
	return &CRDTPSerializable{handle: out}, nil
}

func CreateCRDTPResponse(callID int32, params *CRDTPSerializable) (*CRDTPSerializable, error) {
	return consumeCRDTPParams("CreateCRDTPResponse", "gov8_crdtp_create_response", "", callID, params)
}

// CreateCRDTPNotification creates a notification and consumes params. Unlike
// rusty_v8's CString-shaped API, Go rejects an interior NUL with an error
// before native entry and leaves params usable.
func CreateCRDTPNotification(method string, params *CRDTPSerializable) (*CRDTPSerializable, error) {
	if strings.IndexByte(method, 0) >= 0 {
		return nil, errors.New("gov8: CRDTP notification method contains NUL")
	}
	return consumeCRDTPParams("CreateCRDTPNotification", "gov8_crdtp_create_notification", method, 0, params)
}

func consumeCRDTPParams(op, export, method string, callID int32, params *CRDTPSerializable) (*CRDTPSerializable, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	if params != nil {
		params.mu.Lock()
		defer params.mu.Unlock()
	}
	var paramsHandle uintptr
	if params != nil {
		var err error
		paramsHandle, err = params.withHandle()
		if err != nil {
			return nil, err
		}
	}
	var out uintptr
	var consumed int32
	var status uintptr
	if export == "gov8_crdtp_create_response" {
		status, _, _ = proc(export).Call(uintptr(callID), paramsHandle,
			uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&consumed)))
	} else {
		methodBytes := []byte(method)
		status, _, _ = proc(export).Call(slicePointer(methodBytes), uintptr(len(methodBytes)),
			paramsHandle, uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&consumed)))
		runtime.KeepAlive(methodBytes)
	}
	if params != nil {
		if consumed == 1 {
			params.consumed = true
			params.handle = 0
		} else if consumed != 0 {
			return nil, fmt.Errorf("gov8: %s returned invalid consumed state %d", op, consumed)
		}
	} else if consumed != 0 {
		return nil, fmt.Errorf("gov8: %s consumed absent params", op)
	}
	if int64(status) < 0 {
		return nil, shimError(op, status)
	}
	if (params != nil && consumed != 1) || out == 0 {
		return nil, fmt.Errorf("gov8: %s returned invalid ownership result", op)
	}
	return &CRDTPSerializable{handle: out}, nil
}
