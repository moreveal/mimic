package browser

import (
	"github.com/moreveal/mimic/internal/profile"
	"github.com/moreveal/mimic/internal/state"
)

func (b *Browser) ValidateProfile(raw []byte) (profile.Document, error) {
	return profile.Normalize(raw, b.env, nil)
}
func (b *Browser) NewContextWithProfile(raw []byte) (*Context, error) {
	d, err := b.ValidateProfile(raw)
	if err != nil {
		return nil, err
	}
	if d.Network.Proxy.Server != "" && b.compat.Environment().NewProxyTransport == nil {
		return nil, &profile.Error{Path: "network.proxy", Reason: "unsupported", Message: "bundle has no proxy transport"}
	}
	return b.newContext(&d), nil
}
func (c *Context) Environment() state.Environment  { return c.env.Clone() }
func (p *Page) BaseEnvironment() state.Environment { return p.ctx.Environment() }

// Internal projections only read this snapshot. Mutable subobjects are replaced,
// never edited in place; public consumers use Environment's defensive copy.
func (p *Page) environmentView() state.Environment { p.mu.RLock(); defer p.mu.RUnlock(); return p.env }
func (c *Context) Profile() profile.Document {
	return profile.FromEnvironment(c.env.Clone(), c.browser.env.ProfileID, c.proxy)
}
func (p *Page) Profile() profile.Document {
	return profile.FromEnvironment(p.Environment(), p.ctx.browser.env.ProfileID, p.ctx.proxy)
}

// Caller owns the Page command boundary, shared with all CDP sessions.
func (p *Page) UpdateProfile(raw []byte) (profile.Document, error) {
	current := p.Profile()
	d, err := profile.Normalize(raw, p.ctx.browser.env, &current)
	if err != nil {
		return current, err
	}
	p.applyProfileEnvironment(d.Apply(p.ctx.env))
	return p.Profile(), nil
}
func (p *Page) ResetProfileOverrides() profile.Document {
	p.applyProfileEnvironment(p.ctx.env.Clone())
	return p.Profile()
}
func (p *Page) applyProfileEnvironment(e state.Environment) {
	p.viewportObservationChange(true)
	p.mu.Lock()
	p.env = e
	p.mu.Unlock()
	p.viewportObservationChange(false)
}
