(async()=>{
  const out={},sample=m=>({used:m.usedJSHeapSize,total:m.totalJSHeapSize,limit:m.jsHeapSizeLimit}),before=performance.memory,initial=sample(before);
  const allocations=Array.from({length:32},()=>Array.from({length:65536},(_,i)=>({value:i})));globalThis.__performanceMemoryWorkload=allocations;
  out.immediate={sameSnapshot:JSON.stringify(sample(before))===JSON.stringify(initial),freshStable:JSON.stringify(sample(performance.memory))===JSON.stringify(initial)};
  await new Promise(r=>setTimeout(r,1200));out.afterTask={heldUnchanged:JSON.stringify(sample(before))===JSON.stringify(initial),limitStable:performance.memory.jsHeapSizeLimit===initial.limit};
  out.descriptors=Reflect.ownKeys(Object.getPrototypeOf(before)).map(k=>{const d=Object.getOwnPropertyDescriptor(Object.getPrototypeOf(before),k);let brand;try{d.get?.call({});brand='ok'}catch(e){brand=e.name}return {key:String(k),enumerable:d.enumerable,configurable:d.configurable,get:!!d.get,set:!!d.set,brand}});
  if(typeof document!=='undefined'){const f=document.createElement('iframe');document.body.append(f);out.frame={newSnapshot:f.contentWindow.performance.memory!==performance.memory,limitSame:f.contentWindow.performance.memory.jsHeapSizeLimit===performance.memory.jsHeapSizeLimit};f.remove()}
  delete globalThis.__performanceMemoryWorkload;return out;
})()
