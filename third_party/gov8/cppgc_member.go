//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// CppGCMemberEdges is a copied observation of the two graph edges embedded in
// a gov8 cppgc object. No native pointer escapes: absent or GC-cleared edges
// are represented by the corresponding Present field being false.
type CppGCMemberEdges struct {
	Strong        CppGCObjectSnapshot
	StrongPresent bool
	Weak          CppGCObjectSnapshot
	WeakPresent   bool
	SameTarget    bool
}

func cppgcStrongOwner(owner *CppGCPersistent) (*cppgcPersistentHandle, error) {
	if owner == nil || owner.persistent == nil {
		return nil, fmt.Errorf("gov8: nil cppgc member owner")
	}
	handle := owner.persistent
	if handle.weak {
		return nil, fmt.Errorf("gov8: cppgc member owner must be a strong persistent")
	}
	if handle.iso == nil {
		return nil, fmt.Errorf("gov8: cppgc member owner has no isolate")
	}
	if err := handle.iso.check(); err != nil {
		return nil, err
	}
	return handle, nil
}

func (owner *CppGCPersistent) setMember(child *CppGCObject, weak bool) error {
	handle, err := cppgcStrongOwner(owner)
	if err != nil {
		return err
	}
	iso, scopeHandle, wrapperHandle, tag, registryID, err := cppgcObjectPersistentInputs(child)
	if err != nil {
		return err
	}
	if iso != handle.iso {
		return foreignIsolate("cppgc member child")
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if handle.closed || handle.handle == 0 {
		return fmt.Errorf("gov8: cppgc member owner used after Close")
	}
	r1, _, _ := proc("gov8_cppgc_member_set").Call(
		handle.handle, handle.iso.handleAssumingCheck(), scopeHandle,
		wrapperHandle, uintptr(tag), uintptr(registryID), boolWord(weak))
	if int64(r1) < 0 {
		return shimError("CppGCPersistent.SetMember", r1)
	}
	return nil
}

func (owner *CppGCPersistent) clearMember(weak bool) error {
	handle, err := cppgcStrongOwner(owner)
	if err != nil {
		return err
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if handle.closed || handle.handle == 0 {
		return fmt.Errorf("gov8: cppgc member owner used after Close")
	}
	r1, _, _ := proc("gov8_cppgc_member_clear").Call(
		handle.handle, handle.iso.handleAssumingCheck(), boolWord(weak))
	if int64(r1) < 0 {
		return shimError("CppGCPersistent.ClearMember", r1)
	}
	return nil
}

func (owner *CppGCPersistent) member(weak bool) (snapshot CppGCObjectSnapshot, registryID uint64, present bool, err error) {
	handle, err := cppgcStrongOwner(owner)
	if err != nil {
		return snapshot, 0, false, err
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if handle.closed || handle.handle == 0 {
		return snapshot, 0, false, fmt.Errorf("gov8: cppgc member owner used after Close")
	}
	var objectID int32
	var tag uint16
	var nativePresent int32
	r1, _, _ := proc("gov8_cppgc_member_get").Call(
		handle.handle, handle.iso.handleAssumingCheck(), boolWord(weak),
		uintptr(unsafe.Pointer(&registryID)), uintptr(unsafe.Pointer(&objectID)),
		uintptr(unsafe.Pointer(&tag)), uintptr(unsafe.Pointer(&nativePresent)))
	if int64(r1) < 0 {
		return snapshot, 0, false, shimError("CppGCPersistent.Member", r1)
	}
	if nativePresent == 0 {
		return snapshot, 0, false, nil
	}
	if nativePresent != 1 || registryID == 0 || !liveCppGCRegistration(registryID, handle.iso) {
		return snapshot, 0, false, fmt.Errorf("gov8: cppgc member returned stale metadata")
	}
	return CppGCObjectSnapshot{ObjectID: objectID, Tag: CppGCTag(tag)}, registryID, true, nil
}

// SetStrongMember assigns the owner's traced strong edge. child must be a live
// gov8 managed object from the same isolate and current Scope.
func (owner *CppGCPersistent) SetStrongMember(child *CppGCObject) error {
	return owner.setMember(child, false)
}

// ClearStrongMember makes the owner's strong edge empty. Collection remains
// asynchronous and occurs only at a later cppgc collection.
func (owner *CppGCPersistent) ClearStrongMember() error {
	return owner.clearMember(false)
}

// StrongMember returns copied metadata for the strong target, if present.
func (owner *CppGCPersistent) StrongMember() (CppGCObjectSnapshot, bool, error) {
	snapshot, _, present, err := owner.member(false)
	return snapshot, present, err
}

// SetWeakMember assigns the owner's traced weak edge. It never keeps child
// alive and is automatically cleared when child is collected.
func (owner *CppGCPersistent) SetWeakMember(child *CppGCObject) error {
	return owner.setMember(child, true)
}

// ClearWeakMember makes the owner's weak edge empty.
func (owner *CppGCPersistent) ClearWeakMember() error {
	return owner.clearMember(true)
}

// WeakMember returns copied metadata for the weak target, if it has not been
// collected.
func (owner *CppGCPersistent) WeakMember() (CppGCObjectSnapshot, bool, error) {
	snapshot, _, present, err := owner.member(true)
	return snapshot, present, err
}

// MemberEdges returns both copied edge observations and whether they identify
// the same live allocation.
func (owner *CppGCPersistent) MemberEdges() (CppGCMemberEdges, error) {
	strong, strongID, strongPresent, err := owner.member(false)
	if err != nil {
		return CppGCMemberEdges{}, err
	}
	weak, weakID, weakPresent, err := owner.member(true)
	if err != nil {
		return CppGCMemberEdges{}, err
	}
	return CppGCMemberEdges{
		Strong: strong, StrongPresent: strongPresent,
		Weak: weak, WeakPresent: weakPresent,
		SameTarget: strongPresent && weakPresent && strongID == weakID,
	}, nil
}
