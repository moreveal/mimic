package browser

import (
	"sort"

	"github.com/moreveal/mimic/internal/engine"
)

// Named buckets are shared by an origin in a Context. Open handles retain a
// deleted bucket only for their realm lifetime, exactly like unlinked storage.
// Records cross the binding as immutable snapshots; compare-and-swap commits
// preserve atomic batches across independently executing Pages.
type cacheBucket struct {
	id      uint64
	version uint64
	records string
}

func addCacheHosts(r *Realm, h map[string]any) {
	h["cacheStorage"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		c := r.agent.Page().ctx
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.cacheNames == nil {
			c.cacheNames = map[string]map[string]*cacheBucket{}
		}
		names := c.cacheNames[r.origin]
		if names == nil {
			names = map[string]*cacheBucket{}
			c.cacheNames[r.origin] = names
		}
		switch strarg(a, 0) {
		case "open":
			name := strarg(a, 1)
			b := names[name]
			if b == nil {
				c.cacheSequence++
				b = &cacheBucket{id: c.cacheSequence, records: "[]"}
				names[name] = b
			}
			if r.cacheHandles == nil {
				r.cacheHandles = map[uint64]*cacheBucket{}
			}
			r.cacheHandles[b.id] = b
			return r.val(b.id), nil
		case "keys":
			keys := make([]string, 0, len(names))
			for key := range names {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool { return names[keys[i]].id < names[keys[j]].id })
			return r.val(keys), nil
		case "has":
			return r.val(names[strarg(a, 1)] != nil), nil
		case "delete":
			name := strarg(a, 1)
			existed := names[name] != nil
			delete(names, name)
			return r.val(existed), nil
		case "read", "write":
			b := r.cacheHandles[uint64(numarg(a, 1))]
			if b == nil {
				return nil, nil
			}
			if strarg(a, 0) == "read" {
				return r.val(map[string]any{"version": b.version, "records": b.records}), nil
			}
			if b.version != uint64(numarg(a, 2)) {
				return r.val(false), nil
			}
			b.records = strarg(a, 3)
			b.version++
			return r.val(true), nil
		}
		return nil, nil
	})
}
