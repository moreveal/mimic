package browser

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/state"
)

type Browser struct {
	mu       sync.RWMutex
	factory  engine.Factory
	env      state.Environment
	compat   compatibility.Bundle
	contexts map[string]*Context
}

func New(factory engine.Factory, bundle compatibility.Bundle) (*Browser, error) {
	if bundle == nil || bundle.Environment() == nil {
		return nil, fmt.Errorf("compatibility bundle is required")
	}
	env := bundle.Environment().State
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return &Browser{factory: factory, env: env, compat: bundle, contexts: map[string]*Context{}}, nil
}
func (b *Browser) NewContext() *Context {
	b.mu.Lock()
	defer b.mu.Unlock()
	lifetime, cancel := context.WithCancel(context.Background())
	c := &Context{lifetime: lifetime, cancel: cancel, ID: uuid.NewString(), browser: b, cookies: network.NewCookieStore(), network: network.NewSessionState(), storage: map[string]map[string]string{}, pages: map[string]*Page{}}
	b.contexts[c.ID] = c
	return c
}
func (b *Browser) Environment() state.Environment      { b.mu.RLock(); defer b.mu.RUnlock(); return b.env }
func (b *Browser) Compatibility() compatibility.Bundle { return b.compat }

type Context struct {
	indexedDatabases   map[string]map[string]*indexedDatabase
	indexedSequence    uint64
	cacheNames         map[string]map[string]*cacheBucket
	cacheSequence      uint64
	bootstrapSnapshots bootstrapSnapshotCache
	storageMu          sync.Mutex
	permissionRealms   map[*Realm]struct{}
	lifetime           context.Context
	cancel             context.CancelFunc
	mu                 sync.RWMutex
	ID                 string
	browser            *Browser
	cookies            *network.CookieStore
	network            *network.SessionState
	transport          network.Transport
	storage            map[string]map[string]string
	capabilities       map[string]*originCapabilities
	pages              map[string]*Page
}

func (c *Context) NewPage() (*Page, error) {
	c.mu.Lock()
	if err := c.lifetime.Err(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	if c.transport == nil {
		profile := c.browser.Compatibility().Environment()
		if profile.NewTransport != nil {
			transport, err := profile.NewTransport()
			if err != nil {
				c.mu.Unlock()
				return nil, fmt.Errorf("create %s network transport: %w", c.browser.Environment().Network.WireProfile, err)
			}
			c.transport = transport
		}
	}
	p, err := newPage(c)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := p.initBlank(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if err := c.lifetime.Err(); err != nil {
		c.mu.Unlock()
		_ = p.Close()
		return nil, err
	}
	c.pages[p.ID] = p
	c.mu.Unlock()
	return p, nil
}
func (c *Context) Pages() []*Page {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*Page, 0, len(c.pages))
	for _, p := range c.pages {
		out = append(out, p)
	}
	return out
}
func (c *Context) Page(id string) (*Page, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.pages[id]
	return p, ok
}
func (c *Context) ClosePage(id string) bool {
	c.mu.Lock()
	p, ok := c.pages[id]
	if ok {
		delete(c.pages, id)
	}
	c.mu.Unlock()
	if ok {
		_ = p.Close()
	}
	return ok
}
func (c *Context) Cookies() *network.CookieStore { return c.cookies }
func (c *Context) store(origin string) map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.storage[origin] == nil {
		c.storage[origin] = map[string]string{}
	}
	return c.storage[origin]
}

type Frame struct {
	windowName         string
	ID                 string
	page               *Page
	parent             *Frame
	elementID          int64
	children           map[string]*Frame
	Realm              *Realm
	navigationStarted  bool
	navigationPending  bool
	navigationCancel   context.CancelFunc
	navigationSequence uint64
	loaderID           string
	pendingMessages    []frameMessage
	loadBlockers       map[uint64]string
}

// ExecutionAgent is the browser-observable owner of a realm and event loop.
// Frame is the first implementation; dedicated/shared workers can implement
// the same boundary without pretending to be frames or sharing globals.
type ExecutionAgent interface {
	Page() *Page
	ContextID() string
	AgentType() string
}

func (f *Frame) Page() *Page       { return f.page }
func (f *Frame) ContextID() string { return f.ID }
func (f *Frame) AgentType() string { return "frame" }
func (f *Frame) Parent() *Frame    { return f.parent }
func (f *Frame) URL() string {
	if f == nil || f.Realm == nil || f.Realm.documentURL() == nil {
		return ""
	}
	return f.Realm.documentURL().String()
}
func (f *Frame) LoaderID() string {
	if f == nil {
		return ""
	}
	return f.loaderID
}
func (f *Frame) RealmID() string {
	if f == nil || f.Realm == nil {
		return ""
	}
	return f.Realm.ID
}
func (f *Frame) ReadyState() string {
	if f == nil || f.Realm == nil {
		return ""
	}
	return f.Realm.readyState
}
func (f *Frame) Children() []*Frame {
	if f == nil {
		return nil
	}
	children := make([]*Frame, 0, len(f.children))
	for _, child := range f.children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool { return children[i].elementID < children[j].elementID })
	return children
}
func (f *Frame) Top() *Frame {
	for f.parent != nil {
		f = f.parent
	}
	return f
}

type WindowProxy struct{ frame *Frame }

func (w *WindowProxy) Realm() *Realm { return w.frame.Realm }

func (b *Browser) String() string { return fmt.Sprintf("Mimic/%s", b.env.Product.FullVersion) }

// Close releases all pages before their shared transport pool.
func (c *Context) Close() error {
	c.Cancel()
	for _, p := range c.Pages() {
		c.ClosePage(p.ID)
	}
	snapshotErr := c.bootstrapSnapshots.close()
	c.mu.Lock()
	defer c.mu.Unlock()
	if closer, ok := c.transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
	return snapshotErr
}

// Cancel stops external work without touching realm-owned JavaScript state.
// Shutdown can call it before joining active browser command runners.
func (c *Context) Cancel() { c.cancel() }
