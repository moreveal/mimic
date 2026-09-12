package browser

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/engine"
)

// opfsStore owns bytes and locks for one origin in one BrowserContext. Handles
// contain identifiers, never engine values, and survive document replacement.
type opfsStore struct {
	mu     sync.Mutex
	next   int
	nodes  map[int]*opfsNode
	access map[int]*opfsAccess
}
type opfsNode struct {
	id, parent int
	name, kind string
	store      string
	children   map[string]int
	data       []byte
	modified   int64
	deleted    bool
}
type opfsAccess struct {
	node     int
	owner    *opfsOwner
	mode     string
	data     []byte
	writable bool
}

// Owner dispatch and teardown share a lock: a late callback cannot add a store
// after close has released that owner's access handles. Store locks are acquired
// only inside this owner lock, never the reverse.
type opfsOwner struct {
	mu     sync.Mutex
	closed bool
	stores map[*opfsStore]bool
}

func (o *opfsOwner) dispatch(s *opfsStore, op string, id int, value any) any {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return opfsError("InvalidStateError")
	}
	if o.stores == nil {
		o.stores = map[*opfsStore]bool{}
	}
	o.stores[s] = true
	return s.call(o, op, id, value)
}

func newOPFSStore() *opfsStore {
	return &opfsStore{next: 1, nodes: map[int]*opfsNode{1: {id: 1, kind: "directory", store: uuid.NewString(), children: map[string]int{}}}, access: map[int]*opfsAccess{}}
}
func (c *Context) opfs(origin string) *opfsStore {
	c.storageMu.Lock()
	defer c.storageMu.Unlock()
	if c.files == nil {
		c.files = map[string]*opfsStore{}
	}
	if c.files[origin] == nil {
		c.files[origin] = newOPFSStore()
	}
	return c.files[origin]
}
func (o *opfsOwner) close() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.closed = true
	for s := range o.stores {
		s.mu.Lock()
		for id, a := range s.access {
			if a.owner == o {
				delete(s.access, id)
			}
		}
		s.mu.Unlock()
	}
	o.stores = nil
}
func installOPFSHost(host map[string]any, rt engine.Runtime, c *Context, origin func() string) *opfsOwner {
	o := &opfsOwner{stores: map[*opfsStore]bool{}}
	host["fileSystem"] = rt.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		s := c.opfs(origin())
		return rt.Value(o.dispatch(s, strarg(args, 0), int(numarg(args, 1)), arg(args, 2))), nil
	})
	return o
}
func opfsError(name string) any { return map[string]any{"error": name} }
func (n *opfsNode) info() any {
	return map[string]any{"id": n.id, "name": n.name, "kind": n.kind, "store": n.store}
}
func (s *opfsStore) locked(id int) bool {
	for _, a := range s.access {
		if a.node == id {
			return true
		}
	}
	for _, n := range s.nodes {
		if !n.deleted && n.parent == id && s.locked(n.id) {
			return true
		}
	}
	return false
}
func (s *opfsStore) remove(id int) {
	n := s.nodes[id]
	n.deleted = true
	for _, child := range n.children {
		s.remove(child)
	}
	n.data = nil
	n.children = nil
}
func (s *opfsStore) call(owner *opfsOwner, op string, id int, value any) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _ := value.(map[string]any)
	number := func(key string) int {
		v, _ := p[key].(float64)
		if v == 0 {
			if x, ok := p[key].(int64); ok {
				return int(x)
			}
		}
		return int(v)
	}
	str := func(key string) string { v, _ := p[key].(string); return v }
	flag := func(key string) bool { v, _ := p[key].(bool); return v }
	if op == "restore" {
		if n := s.nodes[id]; n != nil && str("store") == n.store {
			return n.info()
		}
		return opfsError("DataCloneError")
	}
	if op == "root" {
		return s.nodes[1].info()
	}
	if op == "close" || op == "abort" {
		if a := s.access[id]; a != nil && a.owner == owner {
			if op == "close" && a.writable {
				n := s.nodes[a.node]
				n.data = a.data
				n.modified = time.Now().UnixMilli()
			}
			delete(s.access, id)
		}
		return nil
	}
	if op == "read" || op == "write" || op == "truncate" || op == "size" || op == "flush" {
		a := s.access[id]
		if a == nil || a.owner != owner {
			return opfsError("InvalidStateError")
		}
		n := s.nodes[a.node]
		if a.writable {
			staged := *n
			staged.data = a.data
			n = &staged
			defer func() { a.data = n.data }()
		}
		if a.mode == "read-only" && op != "read" && op != "size" {
			return opfsError("NoModificationAllowedError")
		}
		at := number("at")
		size := number("size")
		if at < 0 || size < 0 || at > 1<<30 || size > 1<<30 {
			return opfsError("QuotaExceededError")
		}
		switch op {
		case "size":
			return len(n.data)
		case "flush":
			return nil
		case "read":
			if at > len(n.data) {
				at = len(n.data)
			}
			end := at + size
			if end > len(n.data) {
				end = len(n.data)
			}
			data := make([]int, end-at)
			for i, b := range n.data[at:end] {
				data[i] = int(b)
			}
			return data
		case "write":
			data := byteSlice(p["data"])
			end := at + len(data)
			if end > 1<<30 {
				return opfsError("QuotaExceededError")
			}
			if end > len(n.data) {
				n.data = append(n.data, make([]byte, end-len(n.data))...)
			}
			copy(n.data[at:], data)
			n.modified = time.Now().UnixMilli()
			return len(data)
		case "truncate":
			if size < len(n.data) {
				n.data = n.data[:size]
			} else {
				n.data = append(n.data, make([]byte, size-len(n.data))...)
			}
			n.modified = time.Now().UnixMilli()
			return nil
		}
	}
	n := s.nodes[id]
	if n == nil || n.deleted {
		return opfsError("NotFoundError")
	}
	switch op {
	case "child":
		name, kind := str("name"), str("kind")
		if child := s.nodes[n.children[name]]; child != nil && !child.deleted {
			if child.kind != kind {
				return opfsError("TypeMismatchError")
			}
			return child.info()
		}
		if !flag("create") {
			return opfsError("NotFoundError")
		}
		s.next++
		child := &opfsNode{id: s.next, parent: id, name: name, kind: kind, store: n.store, modified: time.Now().UnixMilli()}
		if kind == "directory" {
			child.children = map[string]int{}
		}
		n.children[name] = child.id
		s.nodes[child.id] = child
		return child.info()
	case "entries":
		names := make([]string, 0, len(n.children))
		for name := range n.children {
			names = append(names, name)
		}
		sort.Strings(names)
		out := make([]any, 0, len(names))
		for _, name := range names {
			out = append(out, s.nodes[n.children[name]].info())
		}
		return out
	case "resolve":
		if str("store") != n.store {
			return nil
		}
		child := s.nodes[number("target")]
		path := []string{}
		for child != nil && !child.deleted {
			if child.id == id {
				return path
			}
			path = append([]string{child.name}, path...)
			child = s.nodes[child.parent]
		}
		return nil
	case "remove":
		child := s.nodes[n.children[str("name")]]
		if child == nil {
			return opfsError("NotFoundError")
		}
		if s.locked(child.id) {
			return opfsError("NoModificationAllowedError")
		}
		if len(child.children) > 0 && !flag("recursive") {
			return opfsError("InvalidModificationError")
		}
		s.remove(child.id)
		delete(n.children, child.name)
		return nil
	case "file":
		data := make([]int, len(n.data))
		for i, b := range n.data {
			data[i] = int(b)
		}
		return map[string]any{"data": data, "modified": n.modified}
	case "open", "openWritable":
		mode := str("mode")
		for _, a := range s.access {
			if a.node == id && (mode == "readwrite" || a.mode == "readwrite" || mode == "exclusive" || a.mode == "exclusive" || mode != a.mode) {
				return opfsError("NoModificationAllowedError")
			}
		}
		s.next++
		a := &opfsAccess{node: id, owner: owner, mode: mode, writable: op == "openWritable"}
		if a.writable && flag("keep") {
			a.data = append([]byte(nil), n.data...)
		}
		s.access[s.next] = a
		return s.next
	}
	return opfsError("NotSupportedError")
}
