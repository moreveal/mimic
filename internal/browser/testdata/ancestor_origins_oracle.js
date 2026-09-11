(async () => {
  const out={},f=document.createElement('iframe');document.body.appendChild(f);
  try {
    const c=f.contentWindow,list=c.location.ancestorOrigins;
    const attempt=fn=>{try{const value=fn();return{ok:true,value:value===undefined?'undefined':value}}catch(e){return{ok:false,name:e.name,local:e instanceof TypeError}}};
    const originDescriptor=Object.getOwnPropertyDescriptor(window,'origin');
    out.windowOrigin={type:typeof origin,matchesURL:origin===location.origin,childMatches:c.origin===origin,get:typeof originDescriptor.get,set:typeof originDescriptor.set,enumerable:originDescriptor.enumerable,configurable:originDescriptor.configurable,forged:attempt(()=>originDescriptor.get.call({})),borrowed:originDescriptor.get.call(c)===origin};
    window.origin=17;out.windowOrigin.replaceable=origin===17;Object.defineProperty(window,'origin',originDescriptor);
    out.shape={keys:Reflect.ownKeys(DOMStringList.prototype).map(String),iterator:typeof list[Symbol.iterator],constructor:attempt(()=>new DOMStringList()),own:Reflect.ownKeys(list).map(String),indexDescriptor:Object.getOwnPropertyDescriptor(list,'0')};
    out.values={length:list.length,first:list[0]===origin,missing:list[1]===undefined,item:list.item(0)===origin,itemMissing:list.item(99),contains:list.contains(origin),notContained:list.contains('https://missing.invalid')};
    out.shape.indexDescriptor.value='origin';
    out.bindings={forged:attempt(()=>DOMStringList.prototype.item.call({},0)),missing:attempt(()=>DOMStringList.prototype.item.call(list)),containsMissing:attempt(()=>DOMStringList.prototype.contains.call(list)),symbol:attempt(()=>DOMStringList.prototype.contains.call(list,Symbol())),borrowed:DOMStringList.prototype.item.call(list,0)===origin};
    out.mutations={setIndex:Reflect.set(list,'0','other'),setMissing:Reflect.set(list,'99','other'),defineIndex:Reflect.defineProperty(list,'0',{value:'other'}),defineMissing:Reflect.defineProperty(list,'99',{value:'other'}),deleteIndex:Reflect.deleteProperty(list,'0'),deleteMissing:Reflect.deleteProperty(list,'99'),preventExtensions:Reflect.preventExtensions(list),unchanged:list[0]===origin};
    const nested=c.document.createElement('iframe');c.document.body.appendChild(nested);
    out.nested={length:nested.contentWindow.location.ancestorOrigins.length,values:Array.from(nested.contentWindow.location.ancestorOrigins).every(x=>x===origin)};
    const old=list;
    await new Promise(resolve=>{f.onload=resolve;f.srcdoc='<title>replacement</title>'});
    out.after={newList:old!==f.contentWindow.location.ancestorOrigins,oldLength:old.length,oldFirst:old[0]===origin};
    f.remove();out.detached={length:old.length,first:old[0]===origin};return out;
  }finally{f.remove()}
})()
