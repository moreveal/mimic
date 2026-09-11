package browser

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
)

// The bounded annotation buffer belongs to the document, not its JS wrapper.
// Diagnostics can read it while the runtime is blocked, without entering V8.
type crashReportState struct {
	mu   sync.RWMutex
	data string
}

func (p *Page) CrashReports() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := map[string]string{}
	for id, frame := range p.frames {
		if frame.Realm == nil {
			continue
		}
		s := &frame.Realm.crashReport
		s.mu.RLock()
		if s.data != "" {
			result[id] = s.data
		}
		s.mu.RUnlock()
	}
	return result
}

func addWindowServiceHosts(r *Realm, h map[string]any) {
	addIndexedDBHosts(r, h)
	addCacheHosts(r, h)
	addCookieStoreHosts(r, h)
	addLaunchHosts(r, h)
	h["enqueueWebTask"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		fn := a[0]
		id := r.scheduler.Post(scheduler.WebTask, time.Duration(numarg(a, 2)*float64(time.Millisecond)), func(ctx context.Context) error {
			p := r.agent.Page()
			p.userScriptDepth++
			defer func() { p.userScriptDepth-- }()
			_, err := r.runtime.Call(ctx, fn, nil)
			return err
		})
		continuation, _ := arg(a, 3).(bool)
		r.scheduler.SetWebTaskPriority(id, int(numarg(a, 1)), continuation)
		return r.val(id), nil
	})
	h["changeWebTask"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		id := uint64(numarg(a, 0))
		if numarg(a, 1) < 0 {
			r.scheduler.Cancel(id)
		} else {
			continuation, _ := arg(a, 2).(bool)
			r.scheduler.SetWebTaskPriority(id, int(numarg(a, 1)), continuation)
		}
		return nil, nil
	})
	h["currentWebTask"] = r.fn(func(_ engine.Value, _ []engine.Value) (engine.Value, error) {
		return r.val(r.scheduler.ExecutionStatus().TaskID), nil
	})
	h["readCrashReport"] = r.fn(func(_ engine.Value, _ []engine.Value) (engine.Value, error) {
		r.crashReport.mu.RLock()
		defer r.crashReport.mu.RUnlock()
		return r.val(r.crashReport.data), nil
	})
	h["writeCrashReport"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		r.crashReport.mu.Lock()
		r.crashReport.data = strarg(a, 0)
		r.crashReport.mu.Unlock()
		return nil, nil
	})
}

// QueueLaunch is the embedder entry point for an OS/app URL launch. Launches
// wait for a consumer in the active top document and survive its navigation.
func (p *Page) QueueLaunch(targetURL string) error {
	u, err := url.Parse(targetURL)
	if err != nil || !u.IsAbs() {
		return fmt.Errorf("launch URL must be absolute")
	}
	p.mu.Lock()
	if p.Top == nil || p.Top.Realm == nil {
		p.mu.Unlock()
		return fmt.Errorf("page is closed")
	}
	p.launches = append(p.launches, u.String())
	r := p.Top.Realm
	if r != nil && r.launchNotifier != nil {
		callback := r.launchNotifier
		r.scheduler.Post(scheduler.UserInteraction, 0, func(ctx context.Context) error { _, err := r.runtime.Call(ctx, callback, nil); return err })
	}
	p.mu.Unlock()
	return nil
}
func addLaunchHosts(r *Realm, h map[string]any) {
	h["installLaunchNotifier"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		p.mu.Lock()
		r.launchNotifier = a[0]
		p.mu.Unlock()
		return nil, nil
	})
	h["nextLaunch"] = r.fn(func(_ engine.Value, _ []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.Top.Realm != r || len(p.launches) == 0 {
			return nil, nil
		}
		value := p.launches[0]
		p.launches = p.launches[1:]
		return r.val(value), nil
	})
}
