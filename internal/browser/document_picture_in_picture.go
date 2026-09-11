package browser

import (
	"context"
	"net/url"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

func addPictureInPictureHosts(r *Realm, h map[string]any) {
	h["auxiliaryOpener"] = r.fn(func(engine.Value, []engine.Value) (engine.Value, error) {
		if f, ok := r.agent.(*Frame); ok && f.auxiliaryOpener != nil {
			return r.val(f.auxiliaryOpener.ID), nil
		}
		return r.val(nil), nil
	})
	h["pictureInPictureState"] = r.fn(func(engine.Value, []engine.Value) (engine.Value, error) {
		f, ok := r.agent.(*Frame)
		id := ""
		if r.pictureInPicture != nil {
			id = r.pictureInPicture.ID
		}
		return r.val(map[string]any{"window": id, "top": ok && f.parent == nil, "auxiliary": ok && f.auxiliaryOpener != nil, "active": !r.inactive, "activation": r.navigationActivated()}), nil
	})
	h["installPictureInPictureLifecycle"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		r.pipLifecycle = a[0]
		return nil, nil
	})
	h["openPictureInPicture"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		if r.pictureInPicture != nil {
			r.closePictureInPictureWindow(r.pictureInPicture, false)
		}
		p := r.agent.Page()
		opener := r.agent.(*Frame)
		width, height := numarg(a, 0), numarg(a, 1)
		screen := p.Environment().Screen()
		if width > float64(screen.AvailWidth) {
			width = float64(screen.AvailWidth)
		}
		if height > float64(screen.AvailHeight) {
			height = float64(screen.AvailHeight)
		}
		f := &Frame{ID: uuid.NewString(), page: p, auxiliaryOpener: opener, auxiliaryOwner: r, auxiliaryBase: r.documentBaseURL(), auxiliaryWidth: int(width), auxiliaryHeight: int(height), children: map[string]*Frame{}}
		// Dimensions belong to this auxiliary context, never to the opener's profile.
		if f.auxiliaryWidth == 0 {
			f.auxiliaryWidth = p.Environment().Window.ViewportWidth
			f.auxiliaryHeight = p.Environment().Window.ViewportHeight
		}
		d, err := dom.Parse("<html><head></head><body></body></html>")
		if err != nil {
			return nil, err
		}
		d.ShareNodeArena(r.document)
		u, _ := url.Parse("about:blank")
		child, err := newRealmState(p, f, d, u, true)
		if err != nil {
			return nil, err
		}
		child.origin = r.origin
		child.referrerPolicy = r.referrerPolicy
		child.documentReferrer = r.documentURL().String()
		child.readyState = "complete"
		f.Realm = child
		p.mu.Lock()
		p.frames[f.ID] = f
		p.mu.Unlock()
		r.pictureInPicture = f
		r.activationConsumed = true
		return r.val(f.ID), nil
	})
	h["closeAuxiliaryWindow"] = r.fn(func(engine.Value, []engine.Value) (engine.Value, error) {
		if f, ok := r.agent.(*Frame); ok && f.auxiliaryOpener != nil {
			r.closePictureInPictureWindow(f, true)
		}
		return nil, nil
	})
}

// Close marks the browsing context synchronously, then performs document
// teardown in a browser task. Retained Window/document references continue to
// resolve through the normal Page-owned realm graph after removal.
func (r *Realm) closePictureInPictureWindow(f *Frame, queued bool) {
	if f == nil || f.Realm == nil {
		return
	}
	p := r.agent.Page()
	if queued {
		if f.windowClosing {
			return
		}
		f.windowClosing = true
		owner := f.auxiliaryOwner
		if owner != nil && !owner.inactive {
			owner.scheduler.Post(scheduler.UserInteraction, 0, func(ctx context.Context) error { r.closePictureInPictureWindow(f, false); return nil })
			return
		}
	}
	f.windowClosing = true
	p.mu.Lock()
	p.removeDescendantFramesLocked(f)
	delete(p.frames, f.ID)
	p.mu.Unlock()
	child := f.Realm
	if !child.inactive && child.pipLifecycle != nil {
		if err := child.runOnOwner(context.Background(), func(ctx context.Context) error {
			_, err := child.runtime.Call(ctx, child.pipLifecycle, nil)
			return err
		}); err != nil {
			p.trace.Add(trace.Error, "documentPictureInPicture.close", map[string]any{"error": err.Error(), "realm": child.ID})
		}
	}
	if owner := f.auxiliaryOwner; owner != nil && owner.pictureInPicture == f {
		owner.pictureInPicture = nil
	}
	p.retireRealm(child)
}
