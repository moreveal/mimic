package browser

// Contexts returns a registry snapshot; constructing or executing a Page never
// holds the browser registry lock. Each Context retains its own cookie/storage
// and transport ownership.
func (b *Browser) Contexts() []*Context {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]*Context, 0, len(b.contexts))
	for _, c := range b.contexts {
		out = append(out, c)
	}
	return out
}

func (b *Browser) Context(id string) (*Context, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c, ok := b.contexts[id]
	return c, ok
}

func (b *Browser) CloseContext(id string) error {
	b.mu.Lock()
	c := b.contexts[id]
	delete(b.contexts, id)
	b.mu.Unlock()
	if c == nil {
		return nil
	}
	return c.Close()
}

func (p *Page) ContextID() string { return p.ctx.ID }
