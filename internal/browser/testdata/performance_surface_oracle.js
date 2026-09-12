(() => {
  const out = {}, attempt = fn => { try { fn(); return 'ok'; } catch(e) { return e.name; } };
  const names = ['Performance','PerformanceEntry','PerformanceMark','PerformanceMeasure','PerformanceResourceTiming','PerformanceNavigationTiming','PerformanceServerTiming','PerformanceObserver','PerformanceObserverEntryList','PerformanceTiming','PerformanceNavigation','MemoryInfo','PerformanceEventTiming','PerformanceLongTaskTiming','TaskAttributionTiming','PerformancePaintTiming','PerformanceElementTiming','LargestContentfulPaint','LayoutShift','LayoutShiftAttribution','PerformanceLongAnimationFrameTiming','PerformanceScriptTiming','VisibilityStateEntry','NotRestoredReasons','NotRestoredReasonDetails'];
  names.push('PerformanceTimingConfidence','PerformanceSoftNavigation','InteractionContentfulPaint','EventCounts');
  for (const name of names) {
    const C = globalThis[name];
    if (typeof C !== 'function') { out[name] = typeof C; continue; }
    out[name] = { length:C.length, parent:Object.getPrototypeOf(C.prototype)?.constructor?.name,
      construct:attempt(()=>new C()), call:attempt(()=>C()), tag:Object.prototype.toString.call(C.prototype),
      members:Reflect.ownKeys(C.prototype).map(k=>{
        const d=Object.getOwnPropertyDescriptor(C.prototype,k), r={key:String(k),enumerable:d.enumerable,configurable:d.configurable};
        if ('value' in d) {r.writable=d.writable;r.kind=typeof d.value;if(typeof d.value==='function')r.length=d.value.length;}
        else {r.get=!!d.get;r.set=!!d.set;if(d.get)r.brand=attempt(()=>d.get.call(Object.create(C.prototype)));}
        return r;
      }) };
  }
  out.performance = {same:performance===performance, tag:Object.prototype.toString.call(performance), ownKeys:Object.keys(performance),eventTarget:performance instanceof EventTarget,
    jsonKeys:Object.keys(performance.toJSON()),jsonOrigin:performance.toJSON().timeOrigin===performance.timeOrigin,
    allSorted:performance.getEntries().every((e,i,a)=>i===0||a[i-1].startTime<=e.startTime),supported:PerformanceObserver.supportedEntryTypes,
    supportedFrozen:Object.isFrozen(PerformanceObserver.supportedEntryTypes),supportedSame:PerformanceObserver.supportedEntryTypes===PerformanceObserver.supportedEntryTypes};
  for (const name of ['now','toJSON','getEntries','getEntriesByType','getEntriesByName','mark','measure','clearMarks','clearMeasures','clearResourceTimings','setResourceTimingBufferSize']) {
    const fn=Performance.prototype[name]; let conversions=0; const a={toString(){conversions++;return 'x'}};
    out['receiver:'+name]={forged:attempt(()=>fn.call(Object.create(Performance.prototype),a)),conversions,proxy:attempt(()=>fn.call(new Proxy(performance,{}),a)),missing:attempt(()=>fn.call(performance)),symbol:attempt(()=>fn.call(performance,Symbol('x')))};
  }
  if ('memory' in performance) {
    const m=performance.memory, next=performance.memory;
    out.memory={tag:Object.prototype.toString.call(m),same:m===next,keys:Object.keys(m),prototypeKeys:Object.getOwnPropertyNames(Object.getPrototypeOf(m)),finite:[m.usedJSHeapSize,m.totalJSHeapSize,m.jsHeapSizeLimit].every(Number.isFinite),positive:m.usedJSHeapSize>0,ordered:m.usedJSHeapSize<=m.totalJSHeapSize&&m.totalJSHeapSize<=m.jsHeapSizeLimit,stable:[m.usedJSHeapSize===next.usedJSHeapSize,m.totalJSHeapSize===next.totalJSHeapSize,m.jsHeapSizeLimit===next.jsHeapSizeLimit]};
  }
  if ('timing' in performance) {
    out.legacy={timingSame:performance.timing===performance.timing,navigationSame:performance.navigation===performance.navigation,timingOwn:Object.keys(performance.timing),navigationOwn:Object.keys(performance.navigation),timingJSONKeys:Object.keys(performance.timing.toJSON()),navigationJSON:performance.navigation.toJSON()};
  }
  performance.clearMarks();performance.clearMeasures();return out;
})()
