package browser

import (
	"context"
	"fmt"
	"strconv"
)

// Debugger navigation commands project the existing joint history. Entry IDs
// are Page-owned, stable across sessions and never reused after forward history
// is truncated. They do not create a second CDP history list.
func (p *Page) NavigationHistory() (int, []map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]any, 0, len(p.history))
	for i, entry := range p.history {
		if entry.id == 0 {
			p.historySequence++
			entry.id = p.historySequence
		}
		if i == p.historyIndex && p.Top.Realm != nil {
			entry.title = p.Top.Realm.document.Title()
		}
		out = append(out, map[string]any{"id": entry.id, "url": entry.URL.String(), "userTypedURL": entry.URL.String(), "title": entry.title, "transitionType": "typed"})
	}
	return p.historyIndex, out
}
func (p *Page) NavigateToHistoryEntry(id int) error {
	p.mu.RLock()
	index := -1
	for i, entry := range p.history {
		if entry.id == id {
			index = i
			break
		}
	}
	current := p.historyIndex
	r := p.Top.Realm
	p.mu.RUnlock()
	if index < 0 || r == nil {
		return fmt.Errorf("No entry with passed id")
	}
	if index != current {
		r.historyGo(index - current)
	}
	return nil
}
func (p *Page) Reload() error {
	if p.Top.Realm == nil {
		return fmt.Errorf("No document")
	}
	return p.Top.Realm.postNavigate(p.URL(), true, true)
}
func (p *Page) RemoveInitScript(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, script := range p.initScripts {
		if script.ID == id {
			p.initScripts = append(p.initScripts[:i], p.initScripts[i+1:]...)
			return true
		}
	}
	return false
}
func (p *Page) SetDocumentContent(ctx context.Context, frameID, source string) error {
	_, err := p.EvaluateCommand(ctx, frameID, "document.open();document.write("+strconv.Quote(source)+");document.close();undefined")
	return err
}
func (p *Page) FrameForDOMNode(id int64) (*Frame, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, frame := range p.frames {
		if frame.Realm == nil {
			continue
		}
		d := frame.Realm.document
		n, ok := d.Get(id)
		if ok && (n.ID == d.Root().ID || n.OwnerDocument == d.Root().ID) {
			return frame, true
		}
	}
	return nil, false
}
func (f *Frame) ElementNodeID() int64 { return f.elementID }

func (f *Frame) Name() string { return f.windowName }
