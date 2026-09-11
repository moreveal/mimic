// WindowProperties is the named-properties object in the prototype chain,
// never a replacement for the native global proxy. Lookups use the owner's
// current canonical document; only live collection identities are cached.
if(typeof Window==='function'&&Window.prototype){
 const target=Object.create(EventTarget.prototype),collections=new Map();
 Object.defineProperty(target,Symbol.toStringTag,{value:'WindowProperties',configurable:true});
 const lookup=key=>{
  if(typeof key!=='string'||key==='')return undefined;
  const ids=host.windowNamedElements(key);
  for(const id of ids){const data=host.nodeData(id);if(data.tagName==='IFRAME'&&data.attributes?.name===key){const value=wrap(id).contentWindow;if(value)return value}}
  if(!ids.length)return undefined;
  if(ids.length===1)return wrap(ids[0]);
  let collection=collections.get(key);if(!collection){collection=htmlCollection(()=>host.windowNamedElements(key));collections.set(key,collection)}return collection;
 };
 const layer=new Proxy(target,{
  get(t,key,receiver){const value=lookup(key);return value===undefined?Reflect.get(t,key,receiver):value},
  has(t,key){return lookup(key)!==undefined||Reflect.has(t,key)},
  getOwnPropertyDescriptor(t,key){const value=lookup(key);return value===undefined?Reflect.getOwnPropertyDescriptor(t,key):{value,writable:true,enumerable:false,configurable:true}},
  set(t,key,value,receiver){return receiver===layer?true:Reflect.set(t,key,value,receiver)},
  defineProperty(){return true},
  deleteProperty(t,key){return lookup(key)===undefined},
  setPrototypeOf(t,prototype){return prototype===Reflect.getPrototypeOf(t)},
  preventExtensions(){return false}
 });
 Object.setPrototypeOf(Window.prototype,layer);
}
