// Read-only WebIDL receiver and method-shape census. No API method is invoked:
// the newTarget test invokes only our empty constructor, never the tested API.
(() => {
  const interfaces = ['Document','Node','Element','HTMLElement','HTMLInputElement',
    'HTMLCanvasElement','HTMLImageElement','HTMLMediaElement','HTMLCollection',
    'HTMLAllCollection','NodeList','DOMTokenList','CSSStyleDeclaration','Event',
    'MouseEvent','DOMRectReadOnly','Range','NodeIterator','TreeWalker','Attr',
    'DOMException','URL','URLSearchParams','Blob','File','Headers','Request',
    'Response','AbortSignal','PerformanceEntry','PerformanceMark',
    'PerformanceResourceTiming','TextEncoder','TextDecoder','Navigator','Screen',
    'History','Storage'];
  const out = {getters:{}, methods:{}, interfaces:{}};
  const result = fn => {try { const v=fn(); return {result:typeof v}; }
    catch(e) {return {exception:e.name};}};
  for (const name of interfaces) {
    const ctor=globalThis[name];out.interfaces[name]=typeof ctor;
    if (!ctor || !ctor.prototype) continue;
    for (const key of Reflect.ownKeys(ctor.prototype)) {
      if (key==='constructor') continue;
      const id=name+'.'+String(key),d=Object.getOwnPropertyDescriptor(ctor.prototype,key);
      if (d.get) out.getters[id]=result(()=>Reflect.apply(d.get,{},[]));
      if (typeof d.value==='function') out.methods[id]={
        name:d.value.name,length:d.value.length,
        newTarget:result(()=>Reflect.construct(function(){},[],d.value)),
        ownKeys:Reflect.ownKeys(d.value).map(String)
      };
    }
  }
  return out;
})()
