package browser

import (
	"context"
	"fmt"
	"net/url"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
)

// Compilation and graph bookkeeping belong to the Page. Workers only wait for
// immutable fetch results; linking never starts until every static edge is ready.
type moduleGraph struct {
	visited   map[string]bool
	pending   int
	done      bool
	err       error
	callbacks []func(context.Context, error) error
}

func moduleDependency(specifier, referrer string) (*url.URL, *url.URL, error) {
	base, err := url.Parse(referrer)
	if err != nil {
		return nil, nil, err
	}
	target, err := base.Parse(specifier)
	return target, base, err
}

// Linking's resolver only reads completed graph fetches. A missing edge is a
// useful diagnostic, never an implicit blocking network request inside V8.
func (r *Realm) loadedModule(specifier, referrer string) (string, string, error) {
	target, _, err := moduleDependency(specifier, referrer)
	if err != nil {
		return "", "", err
	}
	if r.preparedModules[target.String()] {
		// Native identity owns the already compiled source, including an inline
		// entry reached again through a cyclic import. No duplicate source needed.
		return "", target.String(), nil
	}
	pending := r.moduleFetches[target.String()]
	if pending == nil {
		return "", "", fmt.Errorf("module graph edge was not prepared: %s", target)
	}
	select {
	case <-pending.done:
		return string(pending.response.Body), target.String(), pending.err
	default:
		return "", "", fmt.Errorf("module graph edge is still fetching: %s", target)
	}
}

func (r *Realm) prepareModuleGraph(ctx context.Context, source, name string) *moduleGraph {
	if r.moduleGraphs == nil {
		r.moduleGraphs = make(map[string]*moduleGraph)
	}
	if graph := r.moduleGraphs[name]; graph != nil {
		return graph
	}
	graph := &moduleGraph{visited: make(map[string]bool)}
	r.moduleGraphs[name] = graph
	var visit func(context.Context, string, string) error
	finish := func(ctx context.Context) error {
		if graph.done || graph.pending != 0 && graph.err == nil {
			return nil
		}
		graph.done = true
		callbacks := graph.callbacks
		graph.callbacks = nil
		for _, callback := range callbacks {
			if err := callback(ctx, graph.err); err != nil {
				return err
			}
		}
		return nil
	}
	visit = func(ctx context.Context, code, resourceName string) error {
		if graph.visited[resourceName] {
			return nil
		}
		graph.visited[resourceName] = true
		runtime, ok := r.runtime.(engine.PreparedModuleRuntime)
		if !ok {
			return fmt.Errorf("engine does not support prepared module graphs")
		}
		imports, err := runtime.PrepareModule(ctx, code, resourceName)
		if err != nil {
			return err
		}
		if r.preparedModules == nil {
			r.preparedModules = make(map[string]bool)
		}
		r.preparedModules[resourceName] = true
		for _, specifier := range imports {
			target, base, err := moduleDependency(specifier, resourceName)
			if err != nil {
				return err
			}
			if graph.visited[target.String()] {
				continue
			}
			pending := r.fetchModule(r.moduleRequest(target, base))
			graph.pending++
			eventLoop := r.scheduler
			r.resourceWG.Add(1)
			go func() {
				defer r.resourceWG.Done()
				response, err := pending.wait(r.resourceContext)
				if r.resourceContext.Err() != nil {
					return
				}
				eventLoop.Post(scheduler.Network, 0, func(ctx context.Context) error {
					if r.inactive || r.resourceContext.Err() != nil || graph.done {
						return nil
					}
					graph.pending--
					if err == nil {
						err = visit(ctx, string(response.Body), target.String())
					}
					if err != nil {
						graph.err = err
					}
					return finish(ctx)
				})
			}()
		}
		return nil
	}
	graph.err = visit(ctx, source, name)
	_ = finish(ctx) // no subscribers exist until the initial preparation returns
	return graph
}

func (r *Realm) deferNavigationModuleGraph(s *documentStream, source, name string, resume func() error) (bool, error) {
	r.enableAsyncModules()
	graph := r.prepareModuleGraph(s.executionContext(), source, name)
	if graph.done {
		return false, graph.err
	}
	graph.callbacks = append(graph.callbacks, func(taskContext context.Context, _ error) error {
		if s.ctx.Err() != nil || r.documentStream != s || r.inactive {
			return nil
		}
		previous := s.inNavigationTask
		s.inNavigationTask = true
		restoreTaskContext := s.useTaskContext(taskContext)
		defer restoreTaskContext()
		defer func() { s.inNavigationTask = previous }()
		return resume()
	})
	return true, nil
}

func (r *Realm) enableAsyncModules() {
	runtime, ok := r.runtime.(engine.PreparedModuleRuntime)
	if !ok {
		return
	}
	runtime.SetDynamicModuleHandler(func(specifier, referrer string, complete engine.DynamicModuleCompletion) {
		// Use this realm's queue within the Page event loop. Its checkpoint must
		// run the importing realm's promise reactions, including child frames.
		eventLoop := r.scheduler
		// This callback runs inside V8. Merely enqueue browser work; never fetch,
		// compile another graph, or recursively enter the Page scheduler here.
		eventLoop.Post(scheduler.Network, 0, func(ctx context.Context) error {
			if r.inactive || r.resourceContext.Err() != nil {
				return nil
			}
			if base, err := url.Parse(referrer); err == nil && !base.IsAbs() {
				referrer = r.documentURL().String()
			}
			target, base, err := moduleDependency(specifier, referrer)
			if err != nil {
				return complete(ctx, "", "", nil, err)
			}
			pending := r.fetchModule(r.moduleRequest(target, base))
			r.resourceWG.Add(1)
			go func() {
				defer r.resourceWG.Done()
				response, err := pending.wait(r.resourceContext)
				if r.resourceContext.Err() != nil {
					return
				}
				eventLoop.Post(scheduler.Network, 0, func(ctx context.Context) error {
					if r.inactive || r.resourceContext.Err() != nil {
						return nil
					}
					if err != nil {
						return complete(ctx, "", "", nil, err)
					}
					source, name := string(response.Body), target.String()
					graph := r.prepareModuleGraph(ctx, source, name)
					if graph.done {
						return complete(ctx, source, name, r.loadedModule, graph.err)
					}
					graph.callbacks = append(graph.callbacks, func(ctx context.Context, err error) error {
						return complete(ctx, source, name, r.loadedModule, err)
					})
					return nil
				})
			}()
			return nil
		})
	})
}
