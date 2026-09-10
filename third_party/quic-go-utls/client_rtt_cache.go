package quic

import (
	"container/list"
	"sync"
	"time"
)

// ClientRTTCache remembers measured RTTs across connections within one client.
// Config clones share the cache; independent clients should create their own.
type ClientRTTCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*list.Element
	order    list.List
}

type clientRTTEntry struct {
	key string
	rtt time.Duration
}

func NewClientRTTCache(capacity int) *ClientRTTCache {
	return &ClientRTTCache{capacity: max(1, capacity), entries: make(map[string]*list.Element)}
}

func (c *ClientRTTCache) get(key string) time.Duration {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.order.MoveToFront(e)
		return e.Value.(clientRTTEntry).rtt
	}
	return 0
}

func (c *ClientRTTCache) put(key string, rtt time.Duration) {
	if c == nil || rtt < time.Microsecond {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		e.Value = clientRTTEntry{key, rtt}
		c.order.MoveToFront(e)
		return
	}
	if c.order.Len() >= c.capacity {
		e := c.order.Back()
		delete(c.entries, e.Value.(clientRTTEntry).key)
		c.order.Remove(e)
	}
	c.entries[key] = c.order.PushFront(clientRTTEntry{key, rtt})
}
