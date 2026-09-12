package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func (b *Browser) DevPreviewEnabled() bool { return b.devPreview }

// PreviewSubscription has a single latest-state mailbox: slow viewers never
// hold the Page command lock waiting for network I/O. All methods except reading
// Updates require the Page command lock, just like other Page observations.
type PreviewSubscription struct {
	Updates chan []byte
	version string
}

func (p *Page) SubscribePreview() (*PreviewSubscription, error) {
	if !p.ctx.browser.devPreview {
		return nil, fmt.Errorf("dev preview is disabled")
	}
	s := &PreviewSubscription{Updates: make(chan []byte, 1)}
	if p.previewObservers == nil {
		p.previewObservers = make(map[*PreviewSubscription]struct{})
	}
	p.previewObservers[s] = struct{}{}
	return s, nil
}

func (p *Page) UnsubscribePreview(s *PreviewSubscription) {
	delete(p.previewObservers, s)
	if len(p.previewObservers) == 0 {
		p.previewObservers = nil
	}
}

func (r *Realm) readPreview(kind string) (any, error) {
	if deferred, ok := r.runtime.(*deferredRuntime); ok {
		if _, err := deferred.ready(); err != nil {
			return nil, err
		}
	}
	if r.previewRead == nil {
		return nil, fmt.Errorf("preview binding is unavailable")
	}
	var result any
	err := r.runOnOwner(context.Background(), func(ctx context.Context) error {
		v, err := r.runtime.Call(ctx, r.previewRead, nil, r.val(kind))
		if err == nil {
			result = v.Export()
		}
		return err
	})
	return result, err
}

func (p *Page) publishPreview() {
	frames := make([]*Frame, 0, len(p.frames))
	for _, frame := range p.frames {
		if frame.Realm != nil && !frame.Realm.closed && !frame.Realm.inactive {
			frames = append(frames, frame)
		}
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i].ID < frames[j].ID })
	var version strings.Builder
	version.WriteString(p.URL())
	var issue error
	for _, frame := range frames {
		v, err := frame.Realm.readPreview("version")
		if err != nil {
			issue = err
			break
		}
		fmt.Fprintf(&version, "|%s:%s:%v", frame.ID, frame.Realm.ID, v)
	}
	key := version.String()
	dirty := issue != nil
	for sub := range p.previewObservers {
		if sub.version != key {
			dirty = true
		}
	}
	if !dirty {
		return
	}
	packet := map[string]any{"target": p.ID, "url": p.URL(), "closed": len(frames) == 0}
	if issue == nil && len(frames) > 0 && p.Top.Realm != nil {
		packet["html"], issue = p.previewDocument(p.Top)
		packet["width"] = p.env.Window.ViewportWidth
		packet["height"] = p.env.Window.ViewportHeight
	}
	if issue != nil {
		packet["error"] = issue.Error()
	}
	wire, err := json.Marshal(packet)
	if err != nil {
		wire, _ = json.Marshal(map[string]any{"error": err.Error()})
	}
	for sub := range p.previewObservers {
		if sub.version == key && issue == nil {
			continue
		}
		sub.version = key
		select {
		case <-sub.Updates:
		default:
		}
		sub.Updates <- wire
	}
}
