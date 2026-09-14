package network

import (
	"bytes"
	"encoding/base64"
	"errors"
	"sync"
	"unicode/utf8"
)

// Retained cache and inspector bodies share immutable storage. Retention still
// follows the existing HTTP cache rules and 128-entry request history policy;
// this store does not introduce a byte budget or file-backed storage.
type BodyStorageStats struct {
	ResidentBytes int64 `json:"resident_bytes"`
	StoredBytes   int64 `json:"stored_bytes"`
	Bodies        int   `json:"bodies"`
}

func (s *SessionState) BodyStorageStats() BodyStorageStats {
	store := s.responseBodies
	store.mu.Lock()
	defer store.mu.Unlock()
	return BodyStorageStats{ResidentBytes: store.resident, StoredBytes: store.resident, Bodies: len(store.bodies)}
}

type bodyStore struct {
	mu       sync.Mutex
	writers  sync.WaitGroup
	resident int64
	bodies   map[*storedBody]struct{}
	closed   bool
}

type storedBody struct {
	mu    sync.RWMutex
	store *bodyStore
	refs  int
	data  []byte
	size  int64
	text  bool
}

func newBodyStore() *bodyStore {
	return &bodyStore{bodies: make(map[*storedBody]struct{})}
}

func (s *bodyStore) put(data []byte) (*storedBody, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("response body storage is closed")
	}
	s.writers.Add(1)
	s.mu.Unlock()
	defer s.writers.Done()
	// Copy without holding a Context-wide lock. Public Response.Body and legacy
	// interceptors remain free to mutate their independent representation.
	body := &storedBody{store: s, refs: 1, data: bytes.Clone(data), size: int64(len(data)), text: utf8.Valid(data)}
	s.mu.Lock()
	s.bodies[body] = struct{}{}
	s.resident += body.size
	closed := s.closed
	s.mu.Unlock()
	if closed {
		body.release()
		return nil, errors.New("response body storage is closed")
	}
	return body, nil
}

func (b *storedBody) retain() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.refs == 0 {
		return false
	}
	b.refs++
	return true
}

func (b *storedBody) release() { b.drop(false) }

func (b *storedBody) drop(force bool) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.refs == 0 {
		return
	}
	b.refs--
	if !force && b.refs != 0 {
		return
	}
	b.refs, b.data = 0, nil
	b.store.mu.Lock()
	delete(b.store.bodies, b)
	b.store.resident -= b.size
	b.store.mu.Unlock()
}

// A legacy After interceptor may mutate bytes in place. Compare its final
// representation against immutable storage before sharing it again. This does
// not rely on a racy check for whether the CDP interceptor is currently active.
func (b *storedBody) matches(data []byte) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.refs != 0 && bytes.Equal(b.data, data)
}

func (b *storedBody) copyBytes() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.refs == 0 {
		return nil, errors.New("response body storage was released")
	}
	return bytes.Clone(b.data), nil
}

// protocolBody projects directly to CDP's required wire value instead of first
// creating an independent full raw-body copy.
func (b *storedBody) protocolBody() (string, bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.refs == 0 {
		return "", false, errors.New("response body storage was released")
	}
	if b.text {
		return string(b.data), false, nil
	}
	return base64.StdEncoding.EncodeToString(b.data), true, nil
}

func (s *bodyStore) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	// Admission closes before Wait: no later writer can be added to an empty
	// group. Pending copies finish or discard their retained representation.
	s.writers.Wait()
	s.mu.Lock()
	bodies := make([]*storedBody, 0, len(s.bodies))
	for body := range s.bodies {
		bodies = append(bodies, body)
	}
	s.mu.Unlock()
	for _, body := range bodies {
		body.drop(true)
	}
}

func (s *bodyStore) retainResponse(res Response) (*storedBody, error) {
	if res.sharedBody != nil && res.sharedBody.store == s && res.sharedBody.retain() {
		return res.sharedBody, nil
	}
	return s.put(res.Body)
}
