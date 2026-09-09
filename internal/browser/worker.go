package browser

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
	"github.com/moreveal/mimic/internal/webapi"
)

// DedicatedWorker owns an independent ECMAScript runtime and browser scheduler.
// Cross-agent callbacks are always posted as tasks; neither runtime calls into the
// other directly while executing observable JavaScript.
type DedicatedWorker struct {
	mu                sync.Mutex
	id                int64
	parent            *Realm
	runtime           engine.Runtime
	scheduler         *scheduler.Scheduler
	deliverCallback   engine.Value
	errorCallback     engine.Value
	messageReceiver   engine.Value
	url               *url.URL
	securityURL       *url.URL // inherited creator URL for blob workers; independent of URL base
	closed            bool
	started           bool
	pending           []any
	cancel            context.CancelFunc
	done              chan struct{}
	wake              chan struct{}
	performanceOrigin time.Time
	fetchCancels      map[string]context.CancelFunc // worker task/host callbacks only
	fetchWG           sync.WaitGroup
}

func (r *Realm) hostCreateWorker(_ engine.Value, args []engine.Value) (engine.Value, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("worker callbacks and URL are required")
	}
	r.workerSeq++
	id := r.workerSeq
	workerURL, err := r.resolveDocument(strarg(args, 2))
	if err != nil {
		return nil, err
	}
	w := &DedicatedWorker{id: id, parent: r, deliverCallback: args[0], errorCallback: args[1], url: workerURL, securityURL: r.documentURL(), done: make(chan struct{}), wake: make(chan struct{}, 1), performanceOrigin: r.scheduler.Now()}
	r.workers[id] = w
	source := strarg(args, 3)
	r.agent.Page().trace.Add(trace.Lifecycle, "workerCreated", map[string]any{"url": workerURL.String(), "worker": id, "realm": r.ID})
	w.Start(source)
	return r.val(id), nil
}

// Start launches the worker agent. JavaScript is executed only by the worker's
// own scheduler runner; the creating Window agent never calls into this runtime.
func (w *DedicatedWorker) Start(source string) {
	ctx, cancel := context.WithCancel(w.parent.resourceContext)
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		cancel()
		return
	}
	w.cancel = cancel
	w.mu.Unlock()
	// Worker startup is a browser task, not synchronous constructor work. This
	// deterministic boundary lets terminate() in the creating turn cancel the
	// fetch/realm/script before any of them become observable.
	w.parent.scheduler.Post(scheduler.DOM, 0, func(context.Context) error {
		w.mu.Lock()
		if ctx.Err() != nil || w.closed {
			w.mu.Unlock()
			return nil
		}
		w.started = true
		w.mu.Unlock()
		go w.run(ctx, source)
		return nil
	})
}

func (w *DedicatedWorker) run(ctx context.Context, source string) {
	defer close(w.done)
	if ctx.Err() != nil || w.isClosed() {
		return
	}
	p := w.parent.agent.Page()
	runtime := p.ctx.browser.factory.New()
	workerScheduler := scheduler.New(w.performanceOrigin, func(ctx context.Context) error {
		if checkpoint, ok := runtime.(interface{ MicrotaskCheckpointContext(context.Context) error }); ok {
			return checkpoint.MicrotaskCheckpointContext(ctx)
		}
		return runtime.MicrotaskCheckpoint()
	})
	workerScheduler.SetExecutionScale(p.Environment().Time.ExecutionScale)
	w.mu.Lock()
	w.runtime = runtime
	w.scheduler = workerScheduler
	w.mu.Unlock()
	defer func() {
		w.stopFetches()
		_ = runtime.Close()
		// A terminated Worker object can remain reachable from its creating
		// document. Do not retain its abandoned task queue or response bodies.
		w.mu.Lock()
		w.closed = true
		w.runtime, w.scheduler, w.messageReceiver = nil, nil, nil
		w.pending = nil
		w.mu.Unlock()
	}()
	workerScheduler.SetObserver(func(t scheduler.Transition) {
		p.trace.Add(trace.Scheduler, t.Name, map[string]any{"taskId": t.TaskID, "source": t.Source, "due": t.Due, "worker": w.id})
	})
	runtime.SetTimeSource(workerScheduler.Now)
	runtime.SetGlobalAccessObserver(func(name string, supported bool) {
		p.trace.Add(trace.API, "propertyAccess", map[string]any{"property": "DedicatedWorkerGlobalScope." + name, "supported": supported, "worker": w.id})
		if !supported {
			p.trace.Add(trace.Unsupported, "DedicatedWorkerGlobalScope."+name, map[string]any{"worker": w.id, "access": "property"})
		}
	})
	host := map[string]any{}
	installURLHost(host, runtime, func() *url.URL { return w.url })
	host["createObjectURL"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		source := w.url
		if source.Scheme == "blob" {
			source = w.securityURL
		}
		raw := "blob:" + originOf(source.String()) + "/" + uuid.NewString()
		p.ctx.network.PutBlob(raw, byteSlice(arg(args, 0)), strarg(args, 1))
		return runtime.Value(raw), nil
	})
	host["revokeObjectURL"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		p.ctx.network.RevokeBlob(strarg(args, 0))
		return nil, nil
	})
	w.installFetch(host, ctx)
	workerToken := uuid.NewString()
	host["token"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		return runtime.Value(workerToken), nil
	})
	host["postMessage"] = w.runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		w.deliverToParent(arg(args, 0))
		return nil, nil
	})
	host["setTimer"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("timer callback required")
		}
		fn := args[0]
		delay := time.Duration(numarg(args, 1)) * time.Millisecond
		var callback scheduler.Callback
		var taskID uint64
		repeat, _ := arg(args, 2).(bool)
		callback = func(ctx context.Context) error {
			_, callErr := runtime.Call(ctx, fn, runtime.Get("self"))
			if callErr == nil && repeat && !w.isClosed() {
				taskID = workerScheduler.Post(scheduler.Timer, delay, callback)
			}
			return callErr
		}
		taskID = workerScheduler.Post(scheduler.Timer, delay, callback)
		w.signal()
		return runtime.Value(taskID), nil
	})
	host["clearTimer"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		workerScheduler.Cancel(uint64(numarg(args, 0)))
		return nil, nil
	})
	host["close"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		w.markClosed()
		return nil, nil
	})
	host["semanticMissing"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		name := strarg(args, 0)
		p.trace.Add(trace.SemanticMissing, name, map[string]any{"worker": w.id})
		return nil, nil
	})
	host["navigator"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		environment := p.Environment()
		n := environment.Navigator()
		return runtime.Value(map[string]any{
			"userAgent": n.UserAgent, "appVersion": strings.TrimPrefix(n.UserAgent, "Mozilla/"), "platform": n.Platform,
			"languages": n.Languages, "language": n.Languages[0], "hardwareConcurrency": n.HardwareConcurrency,
			"deviceMemory": n.DeviceMemory, "onLine": n.Online,
		}), nil
	})
	host["location"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		href := w.url.String()
		pathname := w.url.EscapedPath()
		origin := "null"
		if w.url.Scheme == "blob" {
			inner := w.url.Opaque
			if inner == "" {
				inner = strings.TrimPrefix(href, "blob:")
			}
			pathname = inner
			if innerURL, err := url.Parse(inner); err == nil && (innerURL.Scheme == "http" || innerURL.Scheme == "https") {
				origin = innerURL.Scheme + "://" + innerURL.Host
			}
		} else if w.url.Scheme == "http" || w.url.Scheme == "https" {
			origin = w.url.Scheme + "://" + w.url.Host
		}
		return runtime.Value(map[string]any{"href": href, "origin": origin, "protocol": w.url.Scheme + ":", "host": w.url.Host, "hostname": w.url.Hostname(), "port": w.url.Port(), "pathname": pathname, "search": prefixed(w.url.RawQuery, "?"), "hash": prefixed(w.url.Fragment, "#")}), nil
	})
	host["isSecureContext"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		return runtime.Value(w.isSecureContext()), nil
	})
	host["randomBytes"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		n := int(numarg(args, 0))
		if n < 0 || n > 65536 {
			return nil, fmt.Errorf("Crypto.getRandomValues length must be between 0 and 65536 bytes")
		}
		buffer := make([]byte, n)
		if _, err := cryptorand.Read(buffer); err != nil {
			return nil, err
		}
		out := make([]int, n)
		for index, value := range buffer {
			out[index] = int(value)
		}
		p.trace.Add(trace.API, "Crypto.getRandomValues", map[string]any{"bytes": n, "worker": w.id})
		return runtime.Value(out), nil
	})
	host["randomUUID"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		p.trace.Add(trace.API, "Crypto.randomUUID", map[string]any{"worker": w.id})
		return runtime.Value(uuid.NewString()), nil
	})
	host["cryptoDigest"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		name := strings.ToUpper(strings.ReplaceAll(strarg(args, 0), "_", "-"))
		data := byteSlice(arg(args, 1))
		var digest []byte
		switch name {
		case "SHA-1", "SHA1":
			value := sha1.Sum(data)
			digest = value[:]
		case "SHA-256", "SHA256":
			value := sha256.Sum256(data)
			digest = value[:]
		case "SHA-384", "SHA384":
			value := sha512.Sum384(data)
			digest = value[:]
		case "SHA-512", "SHA512":
			value := sha512.Sum512(data)
			digest = value[:]
		default:
			return nil, fmt.Errorf("NotSupportedError: unsupported digest algorithm %s", name)
		}
		out := make([]int, len(digest))
		for index, value := range digest {
			out[index] = int(value)
		}
		p.trace.Add(trace.API, "SubtleCrypto.digest", map[string]any{"algorithm": name, "bytes": len(data), "worker": w.id})
		return runtime.Value(out), nil
	})
	host["performanceNow"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		return runtime.Value(float64(workerScheduler.Now().Sub(w.performanceOrigin)) / float64(time.Millisecond)), nil
	})
	host["performanceTimeOrigin"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		return runtime.Value(float64(w.performanceOrigin.UnixNano()) / float64(time.Millisecond)), nil
	})
	host["catalogJSON"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		catalog := ""
		if bundle := p.Compatibility(); bundle != nil && bundle.Surface() != nil {
			catalog = bundle.Surface().GeneratedCatalogJSON
		}
		return runtime.Value(catalog), nil
	})
	if err := runtime.Set("__workerHost", host); err != nil {
		_ = w.reportError(err)
		return
	}
	if err := runtime.Set("__mimic", host); err != nil {
		_ = w.reportError(err)
		return
	}
	if err := runtime.Set("__mimicIDLExposure", "DedicatedWorker"); err != nil {
		_ = w.reportError(err)
		return
	}
	generated := ""
	var exposure *compatibility.RealmExposure
	if bundle := p.Compatibility(); bundle != nil && bundle.Surface() != nil {
		generated = bundle.Surface().GeneratedJavaScript
		key := "worker.insecure.non-isolated"
		if w.isSecureContext() {
			key = "worker.secure.non-isolated"
		}
		if selected, ok := bundle.Surface().Exposures[key]; ok {
			exposure = &selected
		}
	}
	bootstrap := webapi.WorkerSurface(generated, exposure)
	// Native Worker globals already exist when the initial script job is
	// queued. Our generated bindings are the implementation of those globals,
	// not page work, so install them while establishing the agent. Otherwise a
	// fast parent-side terminate() can cancel hundreds of milliseconds of host
	// binding setup and prevent even tiny Blob worker scripts from starting.
	p.trace.Add(trace.JS, "workerBootstrapStart", map[string]any{"url": w.url.String(), "worker": w.id})
	if _, err := runtime.Eval(ctx, bootstrap, "mimic:worker-surface"); err != nil {
		p.trace.Add(trace.Error, "workerBootstrap", map[string]any{"url": w.url.String(), "worker": w.id, "error": err.Error()})
		_ = w.reportError(err)
		return
	}
	if err := installEvalSourceResolver(runtime); err != nil {
		_ = w.reportError(err)
		return
	}
	w.mu.Lock()
	w.messageReceiver = runtime.Get("__deliver")
	w.mu.Unlock()
	if _, err := runtime.Eval(ctx, `delete globalThis.__deliver;delete globalThis.__workerHost;delete globalThis.__mimicEvalSourceResolver`, "mimic:hide-worker-internals"); err != nil {
		_ = w.reportError(err)
		return
	}
	p.trace.Add(trace.JS, "workerBootstrapEnd", map[string]any{"url": w.url.String(), "worker": w.id})
	workerScheduler.Post(scheduler.DOM, 0, func(taskContext context.Context) error {
		// Worker lifecycle is not a Page lifecycle event in CDP. Keep it in the
		// structured JS trace so protocol subscribers do not misroute it.
		p.trace.Add(trace.JS, "workerStart", map[string]any{"url": w.url.String(), "worker": w.id})
		if source == "" {
			res, loadErr := p.loader.Load(taskContext, network.Request{ContextID: w.parent.agent.ContextID(), URL: w.url, Referrer: w.parent.documentURL(), Initiator: network.Worker})
			if loadErr != nil {
				return loadErr
			}
			source = string(res.Body)
			if res.URL != nil {
				w.url = res.URL
			}
		}
		p.trace.Add(trace.JS, "scriptStart", map[string]any{"url": w.url.String(), "worker": w.id})
		_, err := runtime.Eval(taskContext, source, w.url.String())
		p.trace.Add(trace.JS, "scriptEnd", map[string]any{"url": w.url.String(), "worker": w.id, "error": errorString(err)})
		if err != nil {
			return err
		}
		w.mu.Lock()
		pending := append([]any(nil), w.pending...)
		w.pending = nil
		w.mu.Unlock()
		for _, data := range pending {
			w.postMessageReady(data)
		}
		return nil
	})
	last := time.Now()
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		now := time.Now()
		workerScheduler.AdvanceBy(now.Sub(last))
		last = now
		if err := workerScheduler.RunReady(ctx, 10000); err != nil && ctx.Err() == nil {
			_ = w.reportError(err)
		}
		if w.isClosed() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

func (w *DedicatedWorker) PostMessage(data any) {
	w.parent.agent.Page().trace.Add(trace.Scheduler, "workerMessageQueued", map[string]any{"direction": "parent-to-worker", "worker": w.id})
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	if w.runtime == nil {
		w.pending = append(w.pending, data)
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()
	w.postMessageReady(data)
}

func (w *DedicatedWorker) postMessageReady(data any) {
	w.mu.Lock()
	runtime, workerScheduler := w.runtime, w.scheduler
	w.mu.Unlock()
	if runtime == nil || workerScheduler == nil {
		return
	}
	workerScheduler.Post(scheduler.DOM, 0, func(ctx context.Context) error {
		w.parent.agent.Page().trace.Add(trace.Scheduler, "workerMessageDispatch", map[string]any{"direction": "parent-to-worker", "worker": w.id})
		w.mu.Lock()
		fn := w.messageReceiver
		w.mu.Unlock()
		_, err := runtime.Call(ctx, fn, runtime.Get("self"), runtime.Value(data))
		if err != nil {
			w.parent.agent.Page().trace.Add(trace.Error, "workerMessageDispatch", map[string]any{"direction": "parent-to-worker", "worker": w.id, "error": err.Error()})
		}
		return err
	})
	w.signal()
}

func (w *DedicatedWorker) deliverToParent(data any) {
	if w.isClosed() {
		return
	}
	w.parent.agent.Page().trace.Add(trace.Scheduler, "workerMessageQueued", map[string]any{"direction": "worker-to-parent", "worker": w.id})
	w.parent.scheduler.Post(scheduler.DOM, 0, func(ctx context.Context) error {
		w.parent.agent.Page().trace.Add(trace.Scheduler, "workerMessageDispatch", map[string]any{"direction": "worker-to-parent", "worker": w.id})
		if w.isClosed() || w.parent.resourceContext.Err() != nil {
			return nil
		}
		_, err := w.parent.runtime.Call(ctx, w.deliverCallback, nil, w.parent.runtime.Value(data))
		return err
	})
}

func (w *DedicatedWorker) reportError(cause error) error {
	if w.isClosed() {
		return nil
	}
	w.parent.agent.Page().trace.Add(trace.Error, "worker", map[string]any{"url": w.url.String(), "worker": w.id, "error": cause.Error()})
	w.parent.scheduler.Post(scheduler.DOM, 0, func(ctx context.Context) error {
		if w.isClosed() || w.parent.resourceContext.Err() != nil {
			return nil
		}
		_, _ = w.parent.runtime.Call(ctx, w.errorCallback, nil, w.parent.runtime.Value(cause.Error()))
		return nil
	})
	return nil
}

func (w *DedicatedWorker) Close() error {
	w.mu.Lock()
	w.closed = true
	cancel, started := w.cancel, w.started
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if started {
		<-w.done
	}
	return nil
}

func (w *DedicatedWorker) signal() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *DedicatedWorker) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

func (w *DedicatedWorker) markClosed() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func prefixed(value, prefix string) string {
	if value == "" {
		return ""
	}
	return prefix + value
}

func (w *DedicatedWorker) isSecureContext() bool {
	scheme := w.url.Scheme
	if scheme == "blob" {
		inner := w.url.Opaque
		if inner == "" {
			inner = strings.TrimPrefix(w.url.String(), "blob:")
		}
		if innerURL, err := url.Parse(inner); err == nil {
			scheme = innerURL.Scheme
		}
	}
	return scheme == "https"
}
