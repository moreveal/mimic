//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

// ExternalReference is one raw address in V8's external-reference table. It
// is comparable, like rusty_v8's Copy union, but opaque so a Go pointer cannot
// be confused with an ordinary Go object reference.
//
// Non-zero raw addresses must identify native code or native data which stays
// valid for every isolate using the reference. A Go heap or stack address does
// not satisfy that contract. NewCallbackExternalReference is the safe way to
// obtain addresses for callbacks implemented by gov8.
type ExternalReference struct {
	address uintptr
}

// NewExternalReference constructs a reference from a native address. Zero is
// the table terminator. The caller owns the pointed-to native allocation and
// must keep it alive for the isolate lifetime.
func NewExternalReference(address uintptr) ExternalReference {
	return ExternalReference{address: address}
}

// Address returns the represented native address.
func (r ExternalReference) Address() uintptr { return r.address }

// IsNull reports whether the reference is V8's table terminator.
func (r ExternalReference) IsNull() bool { return r.address == 0 }

func (r ExternalReference) String() string { return fmt.Sprintf("%#x", r.address) }

// ExternalReferenceCallbackKind selects one process-lifetime native trampoline
// used by gov8's callback implementation. Named and indexed enumerators need
// distinct constants in Go even though rusty_v8 exposes both through the same
// union field type.
type ExternalReferenceCallbackKind int32

const (
	ExternalReferenceFunction ExternalReferenceCallbackKind = iota
	ExternalReferenceNamedGetter
	ExternalReferenceNamedSetter
	ExternalReferenceNamedDefiner
	ExternalReferenceNamedDeleter
	ExternalReferenceNamedQuery
	ExternalReferenceIndexedGetter
	ExternalReferenceIndexedSetter
	ExternalReferenceIndexedDefiner
	ExternalReferenceIndexedDeleter
	ExternalReferenceIndexedQuery
	ExternalReferenceNamedEnumerator
	ExternalReferenceIndexedEnumerator
	ExternalReferenceMessage
)

// NewCallbackExternalReference returns the native shared trampoline address
// for a gov8 callback kind. The result is process-lifetime and may be reused by
// any CreateParams external-reference table.
func NewCallbackExternalReference(kind ExternalReferenceCallbackKind) (ExternalReference, error) {
	if kind < ExternalReferenceFunction || kind > ExternalReferenceMessage {
		return ExternalReference{}, fmt.Errorf("gov8: invalid external-reference callback kind %d", kind)
	}
	if err := loadShim(); err != nil {
		return ExternalReference{}, err
	}
	var address uintptr
	r1, _, _ := proc("gov8_external_reference_callback_address").Call(
		uintptr(kind), uintptr(unsafe.Pointer(&address)))
	if int64(r1) < 0 {
		return ExternalReference{}, shimError("ExternalReference.Callback", r1)
	}
	if address == 0 {
		return ExternalReference{}, fmt.Errorf("gov8: callback external reference %d is null", kind)
	}
	return ExternalReference{address: address}, nil
}

// NewFunctionTemplateFromExternalReference creates the snapshot-portable
// stateless function represented by ExternalReferenceFunction. data must be a
// V8 External; when invoked, the function returns that external pointer as a
// BigInt. V8 remaps both the callback and pointer through the table when the
// function is serialized and loaded in another isolate.
//
// Arbitrary Go FunctionCallback closures are intentionally excluded: their Go
// registry state cannot be serialized into a V8 snapshot.
func (i *Isolate) NewFunctionTemplateFromExternalReference(scope *Scope, callback ExternalReference, data Value) (*FunctionTemplate, error) {
	if err := scope.check(); err != nil {
		return nil, err
	}
	if err := data.check(); err != nil {
		return nil, err
	}
	if scope.iso != i || data.iso != i {
		return nil, foreignIsolate("external-reference function input")
	}
	handle, err := callHandle("ExternalReference.FunctionTemplate",
		proc("gov8_external_reference_function_template_new"),
		i.handleAssumingCheck(), scope.handle, callback.address, data.h)
	if err != nil {
		return nil, err
	}
	return &FunctionTemplate{iso: i, sc: scope, h: handle}, nil
}

// SetExternalReferences configures an optional external-reference table. The
// slice is copied immediately. The shim makes a second native copy, appends a
// null terminator when absent, and retains that table through isolate disposal.
// Calling this with an empty slice is equivalent to rusty_v8's empty Cow: an
// explicit one-element native table containing only the null terminator.
func (p *CreateParams) SetExternalReferences(references []ExternalReference) *CreateParams {
	p.initialize()
	p.externalReferencesSet = true
	p.externalReferences = append(p.externalReferences[:0], references...)
	return p
}

// HasExternalReferences reports whether SetExternalReferences or
// UseEmptyExternalReferences was called. It distinguishes an explicitly empty
// table from the CreateParams default, which leaves the V8 pointer null.
func (p *CreateParams) HasExternalReferences() bool { return p.externalReferencesSet }

func externalReferenceWords(references []ExternalReference) []uintptr {
	words := make([]uintptr, len(references))
	for index, reference := range references {
		words[index] = reference.address
	}
	runtime.KeepAlive(references)
	return words
}

func externalReferenceTableInfo(isolate uintptr) (present bool, length int, first, last uintptr, err error) {
	var nativeLength uintptr
	r1, _, _ := proc("gov8_external_reference_table_info").Call(
		isolate,
		uintptr(unsafe.Pointer(&nativeLength)),
		uintptr(unsafe.Pointer(&first)),
		uintptr(unsafe.Pointer(&last)),
	)
	if int64(r1) < 0 {
		return false, 0, 0, 0, shimError("ExternalReference.TableInfo", r1)
	}
	return r1 != 0, int(nativeLength), first, last, nil
}

func externalReferencesUsed(references []ExternalReference) bool {
	for _, reference := range references {
		return !reference.IsNull()
	}
	return false
}

// NewSnapshotCreatorWithExternalReferences creates a snapshot creator with a
// copied, null-terminated external-reference table. The table stays native and
// live until CreateBlob or Close consumes the creator.
func NewSnapshotCreatorWithExternalReferences(references []ExternalReference) (*SnapshotCreator, error) {
	return newSnapshotCreatorWithExternalReferences(nil, references)
}

// NewSnapshotCreatorFromExistingSnapshotWithExternalReferences is the
// existing-snapshot counterpart of NewSnapshotCreatorWithExternalReferences.
func NewSnapshotCreatorFromExistingSnapshotWithExternalReferences(blob *StartupData, references []ExternalReference) (*SnapshotCreator, error) {
	if err := blob.validForCreation(); err != nil {
		return nil, err
	}
	if blob.requiresExternalReferences && !externalReferencesUsed(references) {
		return nil, errors.New("gov8: snapshot requires a non-empty external-reference table")
	}
	return newSnapshotCreatorWithExternalReferences(blob, references)
}

func newSnapshotCreatorWithExternalReferences(blob *StartupData, references []ExternalReference) (*SnapshotCreator, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	words := externalReferenceWords(references)
	runtime.LockOSThread()
	tid := currentThreadID()
	if err := beginIsolateCreate(); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	var blobPointer, blobLength uintptr
	if blob != nil {
		blobPointer = uintptr(unsafe.Pointer(&blob.bytes[0]))
		blobLength = uintptr(len(blob.bytes))
	}
	var wordsPointer uintptr
	if len(words) != 0 {
		wordsPointer = uintptr(unsafe.Pointer(&words[0]))
	}
	var wrapper, isolateHandle uintptr
	r1, _, _ := proc("gov8_snapshot_creator_new_external_references").Call(
		blobPointer, blobLength, wordsPointer, uintptr(len(words)),
		uintptr(unsafe.Pointer(&wrapper)), uintptr(unsafe.Pointer(&isolateHandle)))
	runtime.KeepAlive(blob)
	runtime.KeepAlive(words)
	if int64(r1) < 0 {
		abandonIsolateCreate()
		runtime.UnlockOSThread()
		return nil, shimError("SnapshotCreator.NewExternalReferences", r1)
	}
	isolate := &Isolate{handle: isolateHandle, tid: tid, advancedExternalReferences: true}
	finishIsolateCreate(isolate)
	return &SnapshotCreator{iso: isolate, wrap: wrapper,
		requiresExternalReferences: externalReferencesUsed(references)}, nil
}

func externalReferenceOnlyParams(params *CreateParams) (*CreateParams, error) {
	if params == nil {
		return nil, errors.New("gov8: snapshot external-reference parameters are nil")
	}
	configuration := *params
	if !configuration.initialized {
		configuration.initialize()
	}
	if !configuration.externalReferencesSet {
		return nil, errors.New("gov8: snapshot external-reference table was not configured")
	}
	if configuration.maxOldGeneration != 0 || configuration.maxYoungGeneration != 0 ||
		configuration.codeRange != 0 || configuration.initialOldGeneration != 0 ||
		configuration.initialYoungGeneration != 0 || configuration.stackLimit != 0 ||
		!configuration.allowAtomicsWait || configuration.counterLookup != nil || configuration.cppGCHeap != nil ||
		configuration.arrayBufferAllocator != nil {
		return nil, errors.New("gov8: snapshot CreateParams options other than external references are not supported by this constructor")
	}
	return &configuration, nil
}

// NewIsolateFromSnapshotWithParams consumes a snapshot with the external
// references configured in params. Other non-default CreateParams options are
// rejected explicitly by this additive constructor rather than ignored.
//
// A blob produced with a non-empty reference table is rejected before V8 when
// params has no table; rusty_v8 reaches a fatal "No external references"
// boundary in that case.
func NewIsolateFromSnapshotWithParams(blob *StartupData, params *CreateParams) (*Isolate, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	if err := blob.validForCreation(); err != nil {
		return nil, err
	}
	configuration, err := externalReferenceOnlyParams(params)
	if err != nil {
		return nil, err
	}
	if blob.requiresExternalReferences && !externalReferencesUsed(configuration.externalReferences) {
		return nil, errors.New("gov8: snapshot requires a non-empty external-reference table")
	}
	words := externalReferenceWords(configuration.externalReferences)
	var wordsPointer uintptr
	if len(words) != 0 {
		wordsPointer = uintptr(unsafe.Pointer(&words[0]))
	}
	runtime.LockOSThread()
	tid := currentThreadID()
	if err := beginIsolateCreate(); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	var holder, isolateHandle uintptr
	r1, _, _ := proc("gov8_isolate_new_snapshot_external_references").Call(
		uintptr(unsafe.Pointer(&blob.bytes[0])), uintptr(len(blob.bytes)),
		wordsPointer, uintptr(len(words)), uintptr(unsafe.Pointer(&holder)),
		uintptr(unsafe.Pointer(&isolateHandle)))
	runtime.KeepAlive(blob)
	runtime.KeepAlive(words)
	if int64(r1) < 0 {
		abandonIsolateCreate()
		runtime.UnlockOSThread()
		return nil, shimError("NewIsolateFromSnapshotWithParams", r1)
	}
	isolate := &Isolate{handle: isolateHandle, tid: tid, advancedExternalReferences: true}
	finishIsolateCreate(isolate)
	blob.trackRef(isolate, holder)
	return isolate, nil
}
