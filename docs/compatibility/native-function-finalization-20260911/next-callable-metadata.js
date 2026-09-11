(() => {
 const result={},shape=fn=>({name:fn.name,length:fn.length,keys:Reflect.ownKeys(fn).map(String),constructible:(()=>{try{Reflect.construct(function(){},[],fn);return true}catch{return false}})()});
 for(const name of ['EventTarget','AbortController','AbortSignal','TextDecoder','Headers','Request','Response','ReadableStream','WritableStream','TransformStream']) {
  const ctor=globalThis[name],entry={constructor:shape(ctor),members:{}};
  for(const key of Reflect.ownKeys(ctor.prototype)) {
   if(key==='constructor')continue;
   const d=Object.getOwnPropertyDescriptor(ctor.prototype,key);
   if(typeof d.value==='function')entry.members[String(key)]=shape(d.value);
   if(d.get)entry.members['get '+String(key)]=shape(d.get);
   if(d.set)entry.members['set '+String(key)]=shape(d.set);
  }
  result[name]=entry;
 }
 return result;
})()
