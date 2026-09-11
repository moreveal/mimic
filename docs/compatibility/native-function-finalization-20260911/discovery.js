(() => {
 const source=fn=>Function.prototype.toString.call(fn);
 const result={};
 for(const name of ['AbortController','AbortSignal','TextDecoder','TextEncoder','Headers','Request','Response','ReadableStream','WritableStream','TransformStream','DOMStringList']) {
  const ctor=globalThis[name];
  if(typeof ctor!=='function'){result[name]={missing:true};continue}
  const methods={};
  for(const key of Reflect.ownKeys(ctor.prototype)) {
   if(key==='constructor')continue;
   const d=Object.getOwnPropertyDescriptor(ctor.prototype,key);
   if(typeof d.value==='function')methods[String(key)]=source(d.value);
   if(d.get)methods['get '+String(key)]=source(d.get);
   if(d.set)methods['set '+String(key)]=source(d.set);
  }
  const statics={};
  for(const key of Reflect.ownKeys(ctor)) {const d=Object.getOwnPropertyDescriptor(ctor,key);if(typeof d.value==='function'&&key!=='prototype')statics[String(key)]=source(d.value)}
  result[name]={source:source(ctor),methods,statics};
 }
 result.intrinsics={arrayValues:source(Array.prototype.values),arrayIterator:source(Array.prototype[Symbol.iterator]),mapEntries:source(Map.prototype.entries)};
 const user=function visibleUserFunction(){return 42};result.userSource=source(user);
 return result;
})()
