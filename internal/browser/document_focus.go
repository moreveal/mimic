package browser

import "github.com/moreveal/mimic/internal/engine"

func (r *Realm) installDocumentFocus(host map[string]any) {
	p := r.agent.Page()
	host["focusDocument"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		if frame, ok := r.agent.(*Frame); ok {
			p.mu.Lock()
			p.focusedFrameID = frame.ID
			for _, child := range p.frames {
				if child.parent == frame && child.elementID == int64(numarg(args, 0)) {
					p.focusedFrameID = child.ID
					break
				}
			}
			p.mu.Unlock()
		}
		return nil, nil
	})
	host["focusedChildElement"] = r.fn(func(engine.Value, []engine.Value) (engine.Value, error) {
		p.mu.RLock()
		defer p.mu.RUnlock()
		owner, _ := r.agent.(*Frame)
		for frame := p.frames[p.focusedFrameID]; frame != nil; frame = frame.parent {
			if frame.parent == owner {
				return r.val(frame.elementID), nil
			}
		}
		return r.val(0), nil
	})
	host["documentHasFocus"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		id := int64(numarg(args, 0))
		p.mu.RLock()
		defer p.mu.RUnlock()
		if !p.pageFocused && !p.focusEmulated {
			return r.val(false), nil
		}
		frame := p.frames[p.focusedFrameID]
		if frame == nil {
			frame = p.Top
		}
		for ; frame != nil; frame = frame.parent {
			if frame.Realm != nil && frame.Realm.document.Root().ID == id {
				return r.val(true), nil
			}
		}
		return r.val(false), nil
	})
	host["setPageFocus"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		p.mu.Lock()
		previous := p.pageFocused || p.focusEmulated
		p.pageFocused = numarg(args, 0) != 0
		current := p.pageFocused || p.focusEmulated
		p.mu.Unlock()
		return r.val(previous != current), nil
	})
}

func (p *Page) SetFocusEmulationEnabled(enabled bool) {
	p.mu.Lock()
	p.focusEmulated = enabled
	p.mu.Unlock()
}
