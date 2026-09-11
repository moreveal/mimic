(() => {
  const out={},desc=(object,key)=>{const d=Object.getOwnPropertyDescriptor(object,key);if(!d)return null;return {enumerable:d.enumerable,configurable:d.configurable,writable:d.writable,get:typeof d.get,set:typeof d.set,type:typeof d.value,name:d.value?.name,length:d.value?.length}};
  out.prototype={keys:Reflect.ownKeys(Location.prototype).map(String),parent:Object.getPrototypeOf(Location.prototype)===Object.prototype};
  out.ownKeys=Reflect.ownKeys(location).map(String);
  out.descriptors=Object.fromEntries(Reflect.ownKeys(location).map(key=>[String(key),desc(location,key)]));
  out.methods={};
  for(const name of ['assign','replace','reload','toString']){
    const fn=location[name];let constructible=true;try{Reflect.construct(function(){},[],fn)}catch{constructible=false}
    out.methods[name]={name:fn.name,length:fn.length,keys:Reflect.ownKeys(fn).map(String),constructible,native:Function.prototype.toString.call(fn).includes('[native code]')};
  }
  const origins=location.ancestorOrigins;
  out.ancestors={same:origins===location.ancestorOrigins,tag:Object.prototype.toString.call(origins),length:origins?.length,keys:Reflect.ownKeys(origins||{}).map(String)};
  return out;
})()
