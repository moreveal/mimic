package browser

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/state"
	"github.com/moreveal/mimic/internal/trace"
	"github.com/moreveal/mimic/internal/webapi"
)

// The cache belongs to a browser Context, not to a process-wide V8 owner.
// It stores immutable seed data only; every consumer receives its own isolate,
// JS graph and native callbacks. A second use admits a background build so an
// ordinary single-realm Page does not pay snapshot serialization startup cost.
const bootstrapSnapshotEntries = 4
const bootstrapSnapshotBytes = 32 << 20

type bootstrapSource struct {
	source, exposureJSON, catalogJSON string
	key                               [32]byte
}

type bootstrapSnapshotEntry struct {
	key                 [32]byte
	seed                []string
	snapshot            engine.BootstrapSnapshot
	capturing, building bool
	done                chan struct{}
	err                 error
	touched             uint64
	buildDuration       time.Duration
}

type bootstrapSnapshotCache struct {
	mu        sync.Mutex
	entries   map[[32]byte]*bootstrapSnapshotEntry
	sequence  uint64
	closed    bool
	builds    sync.WaitGroup
	closeDone chan struct{}
	closeErr  error
}

func (r *Realm) bootstrapSource() *bootstrapSource {
	if r.bootstrapPlan != nil {
		return r.bootstrapPlan
	}
	var generated string
	var exposure *compatibility.RealmExposure
	var surface *compatibility.WebAPISurface
	var exposureName string
	security := r.securityState()
	if bundle := r.agent.Page().Compatibility(); bundle != nil {
		surface = bundle.Surface()
	}
	if surface != nil {
		generated = surface.GeneratedJavaScript
		exposureName = "window.insecure.non-isolated"
		if security.secureContext {
			exposureName = "window.secure.non-isolated"
			if security.crossOriginIsolated {
				exposureName = "window.secure.isolated"
			}
		}
		if selected, ok := surface.Exposures[exposureName]; ok {
			exposure = &selected
		} else {
			exposureName = ""
		}
	}
	plan := &bootstrapSource{}
	if surface != nil {
		plan.source, plan.exposureJSON, plan.catalogJSON = webapi.BootstrapFor(surface, exposureName)
	} else {
		plan.source = webapi.Surface(generated, exposure)
	}
	env := r.agent.Page().Environment()
	profile, _ := json.Marshal(struct {
		Features                                             map[string]bool
		Graphics                                             state.Graphics
		Secure, Isolated, Credentialless, OriginAgentCluster bool
	}{env.Features, env.Graphics, security.secureContext, security.crossOriginIsolated, security.credentialless, security.originAgentCluster})
	hash := sha256.New()
	for _, part := range []string{plan.source, plan.exposureJSON, plan.catalogJSON, string(profile)} {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	copy(plan.key[:], hash.Sum(nil))
	r.bootstrapPlan = plan
	return plan
}

func (r *Realm) newRuntime() (engine.Runtime, error) {
	p := r.agent.Page()
	c := p.ctx
	if err := c.lifetime.Err(); err != nil {
		return nil, err
	}
	factory, ok := c.browser.factory.(engine.BootstrapSnapshotFactory)
	if !ok || !factory.BootstrapSnapshotsEnabled() || os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		return c.browser.factory.New(), nil
	}
	plan := r.bootstrapSource()
	snapshot, capture, issue := c.bootstrapSnapshots.selectEntry(c.lifetime, factory, plan.key)
	r.bootstrapCapture = capture
	if issue != nil {
		p.trace.Add(trace.Error, "bootstrapSnapshotUnavailable", map[string]any{"error": issue.Error()})
	}
	if snapshot != nil {
		runtime, err := snapshot.NewRuntime()
		if err == nil {
			r.bootstrapRestored = true
			return runtime, nil
		}
		if c.lifetime.Err() != nil {
			return nil, c.lifetime.Err()
		}
		p.trace.Add(trace.Error, "bootstrapSnapshotRestoreFailed", map[string]any{"error": err.Error()})
	}
	return c.browser.factory.New(), nil
}

// selectEntry never holds the cache mutex while creating an isolate or running
// JavaScript. Eviction and Close can race creation: the engine artifact owns the
// blob/consumer lifetime boundary and reports a closed artifact safely.
func (c *bootstrapSnapshotCache) selectEntry(ctx context.Context, factory engine.BootstrapSnapshotFactory, key [32]byte) (engine.BootstrapSnapshot, *bootstrapSnapshotEntry, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil, ctx.Err()
	}
	if c.entries == nil {
		c.entries = make(map[[32]byte]*bootstrapSnapshotEntry)
	}
	c.sequence++
	entry := c.entries[key]
	if entry != nil {
		entry.touched = c.sequence
		snapshot, issue := entry.snapshot, entry.err
		if entry.seed != nil && !entry.building && issue == nil && snapshot == nil {
			c.startBuildLocked(ctx, factory, entry)
		}
		c.mu.Unlock()
		return snapshot, nil, issue
	}
	var evicted engine.BootstrapSnapshot
	if len(c.entries) >= bootstrapSnapshotEntries {
		var oldest *bootstrapSnapshotEntry
		for _, candidate := range c.entries {
			if !candidate.capturing && !candidate.building && (oldest == nil || candidate.touched < oldest.touched) {
				oldest = candidate
			}
		}
		if oldest == nil {
			c.mu.Unlock()
			return nil, nil, nil
		}
		delete(c.entries, oldest.key)
		evicted = oldest.snapshot
	}
	entry = &bootstrapSnapshotEntry{key: key, capturing: true, touched: c.sequence}
	c.entries[key] = entry
	c.mu.Unlock()
	if evicted != nil {
		_ = evicted.Close()
	}
	return nil, entry, nil
}

func (c *bootstrapSnapshotCache) startBuildLocked(ctx context.Context, factory engine.BootstrapSnapshotFactory, entry *bootstrapSnapshotEntry) {
	entry.building = true
	entry.done = make(chan struct{})
	source := entry.seed
	entry.seed = nil
	c.builds.Add(1)
	go func() {
		defer c.builds.Done()
		start := time.Now()
		snapshot, err := factory.BuildBootstrapSnapshot(ctx, source...)
		duration := time.Since(start)
		if err == nil && snapshot.SizeBytes() > bootstrapSnapshotBytes {
			err = fmt.Errorf("bootstrap snapshot exceeds per-profile cache budget: %d bytes", snapshot.SizeBytes())
		}
		c.mu.Lock()
		discard := c.closed || c.entries[entry.key] != entry || err != nil
		var evicted []engine.BootstrapSnapshot
		if !discard {
			evicted, err = c.makeRoomLocked(entry, snapshot.SizeBytes())
			discard = err != nil
		}
		if !discard {
			entry.snapshot = snapshot
		}
		entry.err = err
		entry.buildDuration = duration
		c.mu.Unlock()
		for _, old := range evicted {
			_ = old.Close()
		}
		if discard && snapshot != nil {
			_ = snapshot.Close()
		}
		c.mu.Lock()
		entry.building = false
		close(entry.done)
		c.mu.Unlock()
	}()
}

// makeRoomLocked enforces a total retained byte budget, including seeds
// awaiting admission. In-flight builders own transient compilation memory.
func (c *bootstrapSnapshotCache) makeRoomLocked(current *bootstrapSnapshotEntry, size int) ([]engine.BootstrapSnapshot, error) {
	var evicted []engine.BootstrapSnapshot
	if size > bootstrapSnapshotBytes {
		return nil, fmt.Errorf("bootstrap snapshot exceeds cache byte budget")
	}
	for {
		used := size
		var oldest *bootstrapSnapshotEntry
		for _, entry := range c.entries {
			if entry == current {
				continue
			}
			used += bootstrapSeedBytes(entry.seed)
			if entry.snapshot != nil {
				used += entry.snapshot.SizeBytes()
			}
			if !entry.building && !entry.capturing && (oldest == nil || entry.touched < oldest.touched) {
				oldest = entry
			}
		}
		if used <= bootstrapSnapshotBytes {
			return evicted, nil
		}
		if oldest == nil {
			return evicted, fmt.Errorf("bootstrap snapshot cache byte budget exhausted")
		}
		delete(c.entries, oldest.key)
		if oldest.snapshot != nil {
			evicted = append(evicted, oldest.snapshot)
		}
	}
}

func (c *bootstrapSnapshotCache) captured(entry *bootstrapSnapshotEntry, seed []string, err error) {
	if entry == nil {
		return
	}
	c.mu.Lock()
	if c.closed || c.entries[entry.key] != entry {
		c.mu.Unlock()
		return
	}
	entry.capturing = false
	var evicted []engine.BootstrapSnapshot
	if err == nil {
		evicted, err = c.makeRoomLocked(entry, bootstrapSeedBytes(seed))
	}
	entry.err = err
	if err == nil {
		entry.seed = seed
	}
	c.mu.Unlock()
	for _, old := range evicted {
		_ = old.Close()
	}
}

func (c *bootstrapSnapshotCache) abandon(entry *bootstrapSnapshotEntry) {
	if entry == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[entry.key] == entry && entry.capturing {
		delete(c.entries, entry.key)
	}
}

func (c *bootstrapSnapshotCache) invalidate(key [32]byte, reason error) {
	c.mu.Lock()
	entry := c.entries[key]
	var snapshot engine.BootstrapSnapshot
	if entry != nil {
		snapshot = entry.snapshot
		entry.snapshot = nil
		entry.seed = nil
		entry.err = reason
	}
	c.mu.Unlock()
	if snapshot != nil {
		_ = snapshot.Close()
	}
}

// A failed restore has not admitted any document/user script. Dispose all of
// its native values before retrying ordinary initialization on the same DOM.
func (r *Realm) retryBootstrap(err error) error {
	r.agent.Page().trace.Add(trace.Error, "bootstrapSnapshotBindingFailed", map[string]any{"error": err.Error()})
	r.agent.Page().ctx.bootstrapSnapshots.invalidate(r.bootstrapSource().key, err)
	if r.cookieUnsubscribe != nil {
		r.cookieUnsubscribe()
		r.cookieUnsubscribe = nil
	}
	r.cookieNotifier = nil
	r.launchNotifier = nil
	if closeErr := r.runtime.Close(); closeErr != nil {
		return fmt.Errorf("snapshot binding: %v; close failed runtime: %w", err, closeErr)
	}
	r.bootstrapRestored = false
	r.frameReflection = nil
	r.viewportNotifier = nil
	r.frameViewportRead = nil
	r.eventListenerInvoker = nil
	r.frameReferenceImport = nil
	r.frameReferenceDescribe = nil
	r.frameGlobalRead = nil
	r.frameValueEncoder = nil
	r.frameValueRetain = nil
	r.frameValueEncoderJSON = false
	r.frameNodeDescribe = nil
	r.inputDispatcher = nil
	r.permissionNotifier = nil
	r.documentStreamReset = nil
	r.documentStreamEvent = nil
	r.messageReceiver = nil
	r.messagePortReceiver = nil
	r.frameLoadDispatcher = nil
	r.resourceEventDispatcher = nil
	r.performanceNotifier = nil
	r.domQueryCallback = nil
	r.shadowSnapshotCallback = nil
	r.formSnapshotCallback = nil
	r.crossValues = map[int64]engine.Value{}
	r.crossValueSeq = 0
	r.apiTracking = false
	r.apiSeen = map[string]bool{}
	r.runtime = r.agent.Page().ctx.browser.factory.New()
	r.runtime.SetTimeSource(r.scheduler.Now)
	r.runtime.SetGlobalAccessObserver(func(name string, supported bool) {
		if r.apiTracking {
			r.recordAPIAccess("Window."+name, supported)
		}
	})
	return r.installBindings()
}

func (c *bootstrapSnapshotCache) wait(ctx context.Context) error {
	c.mu.Lock()
	var pending []<-chan struct{}
	for _, entry := range c.entries {
		if entry.building {
			pending = append(pending, entry.done)
		}
	}
	c.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.entries {
		if entry.err != nil {
			return entry.err
		}
	}
	return nil
}

func (c *bootstrapSnapshotCache) close() error {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		<-done
		c.mu.Lock()
		err := c.closeErr
		c.mu.Unlock()
		return err
	}
	c.closed = true
	c.closeDone = make(chan struct{})
	var snapshots []engine.BootstrapSnapshot
	for _, entry := range c.entries {
		if entry.snapshot != nil {
			snapshots = append(snapshots, entry.snapshot)
		}
	}
	c.entries = nil
	c.mu.Unlock()
	c.builds.Wait()
	var first error
	for _, snapshot := range snapshots {
		if err := snapshot.Close(); err != nil && first == nil {
			first = err
		}
	}
	c.mu.Lock()
	c.closeErr = first
	close(c.closeDone)
	c.mu.Unlock()
	return first
}

// Recording wraps only the first ordinary initialization for a profile. It
// retains JSON replies, never real host callbacks, Page state or user objects.
// finish turns recording off even for closures which captured the proxy.
// Recording an optional seed must not turn a successful host call into a
// script exception. Defer serialization failures until capture finalization,
// where the cache rejects the seed and the live realm keeps its host bindings.
const bootstrapCaptureSource = `(function(original){
 const stringify=JSON.stringify,apply=Reflect.apply,get=Reflect.get;
 let calls=[],captureFailed=false;const engineKeys=Reflect.ownKeys(globalThis).filter(key=>typeof key==='string');
 const shape=Object.fromEntries(Object.getOwnPropertyNames(original).map(name=>[name,typeof original[name]]));
 globalThis.__mimic=new Proxy(original,{get(target,name,receiver){const value=get(target,name,receiver);if(calls===null||typeof value!=='function')return value;return function(...args){const result=apply(value,target,args);if(calls!==null&&!captureFailed){try{calls.push({name,args:stringify(args),result:stringify(result)})}catch{captureFailed=true}}return result}}});
 return function(){globalThis.__mimic=original;if(captureFailed){calls=null;throw new Error('bootstrap capture serialization failed')}const globalKeys=Reflect.ownKeys(globalThis).filter(key=>typeof key==='string');const result=stringify({shape,calls,engineKeys,globalKeys});calls=null;return result};
})(__mimic)`

func (r *Realm) beginBootstrapCapture() (engine.Value, error) {
	if r.bootstrapCapture == nil {
		return nil, nil
	}
	return r.runtime.Eval(context.Background(), bootstrapCaptureSource, "mimic:bootstrap-capture")
}

func (r *Realm) finishBootstrapCapture(finish engine.Value, source string, bootstrapErr error) {
	entry := r.bootstrapCapture
	r.bootstrapCapture = nil
	if entry == nil {
		return
	}
	if finish == nil {
		r.agent.Page().ctx.bootstrapSnapshots.captured(entry, nil, bootstrapErr)
		return
	}
	value, err := r.runtime.Call(context.Background(), finish, nil)
	if bootstrapErr != nil {
		err = bootstrapErr
	}
	var seed []string
	if err == nil {
		raw := value.String()
		if !json.Valid([]byte(raw)) {
			err = fmt.Errorf("bootstrap capture is not valid JSON")
		} else {
			seed = bootstrapSeedSources(source, raw)
		}
	}
	r.agent.Page().ctx.bootstrapSnapshots.captured(entry, seed, err)
}

// The forwarding proxy is also captured by generated bindings. Switching its
// private target is necessary even when the handwritten restore hook replaces
// its own host parameter. No Go callback is present during serialization.
//
// V8 excludes conditional intrinsics from its serializing context and installs
// them on restore. Retaining generated global placeholders would shadow those
// natives. Instead, keep the static API graph in private descriptors and publish
// it after V8's native globals, in the order captured from ordinary bootstrap.
func bootstrapSeedBytes(parts []string) int {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	return total
}

func bootstrapSeedSources(source, capture string) []string {
	encoded, _ := json.Marshal(capture)
	// Compile data separately from every surviving function. Putting the reply
	// literal and surface into one enclosing function retains its source and
	// literal boilerplates in each deserialized isolate even after replies=null.
	return []string{
		`globalThis.__mimicSnapshotData=JSON.parse(` + string(encoded) + `);`,
		`(function(){
 const stringify=JSON.stringify,apply=Reflect.apply,get=Reflect.get;
 let data=globalThis.__mimicSnapshotData,replies=data.calls,index=0,live=null;const shape=data.shape,engineKeys=data.engineKeys,globalKeys=data.globalKeys;data=null;
 const seedEngineKeys=new Set(Reflect.ownKeys(globalThis));
 const lateEngineKeys=new Set(engineKeys.filter(key=>!seedEngineKeys.has(key)));
 const publication={engineKeys,lateEngineKeys,descriptors:new Map()};
 globalThis.__mimicSnapshotPublication=publication;
 delete globalThis.__mimicSnapshotData;
 const host=new Proxy(Object.create(null),{get(target,name){
  if(live!==null)return get(live,name);
  if(shape[name]!=='function')return undefined;
  return function(...args){
   if(live!==null)return apply(live[name],live,args);
   const next=replies[index++];
   if(!next||next.name!==name||next.args!==stringify(args))throw new Error('bootstrap snapshot replay mismatch: '+String(name)+' at '+(index-1));
   return next.result===undefined?undefined:JSON.parse(next.result);
  };
 }});
 globalThis.__mimic=host;
 globalThis.__mimicSnapshotFinish=function(){
  if(index!==replies.length)throw new Error('bootstrap snapshot replay is incomplete');
  replies=null;
  const restore=globalThis.__mimicRestoreBootstrap;
  if(typeof restore!=='function')throw new Error('bootstrap snapshot restore hook is missing');
  const define=Object.defineProperty,ownDescriptor=Object.getOwnPropertyDescriptor;
  const retainedEngineKeys=new Set();
  for(const key of globalKeys){if(!engineKeys.includes(key))break;retainedEngineKeys.add(key)}
  const published=[];
  for(const key of globalKeys){
   if(retainedEngineKeys.has(key))continue;
   if(lateEngineKeys.has(key)){published.push([key,null]);continue}
   const descriptor=publication.descriptors.get(key)||ownDescriptor(globalThis,key);
   if(descriptor)published.push([key,descriptor]);
  }
  for(const key of Reflect.ownKeys(globalThis)){
   if(typeof key!=='string'||retainedEngineKeys.has(key))continue;
   const descriptor=ownDescriptor(globalThis,key);
   if(descriptor&&!descriptor.configurable)throw new Error('non-configurable snapshot publication: '+key);
   delete globalThis[key];
  }
  globalThis.__mimicRestoreBootstrap=function(next){
   live=next;
   // V8 installs conditional intrinsics only when deserializing a context.
   // Apply the profile's native-global exposure before publishing Web APIs.
   for(const key of lateEngineKeys)if(!globalKeys.includes(key))delete globalThis[key];
   const descriptors=published.map(([key,descriptor])=>[key,descriptor||ownDescriptor(globalThis,key)]);
   for(const [key]of descriptors)delete globalThis[key];
   for(const [key,descriptor]of descriptors)if(descriptor)define(globalThis,key,descriptor);
   return restore(host);
  };
  delete globalThis.__mimic;delete globalThis.__mimicSnapshotFinish;
 };
})();`,
		// Match BootstrapRuntime.EvalBootstrap's origin without shifting lines.
		source + "\n//# sourceURL=mimic:webapi-surface",
		`__mimicSnapshotFinish();`,
	}
}
