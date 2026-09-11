(() => {
 const result={},shape=fn=>({name:fn.name,length:fn.length,keys:Reflect.ownKeys(fn).map(String),constructible:(()=>{try{Reflect.construct(function(){},[],fn);return true}catch{return false}})()});
 const interfaces={EventTarget:['addEventListener','removeEventListener','dispatchEvent'],AbortController:['signal','abort'],AbortSignal:['aborted','reason','onabort','throwIfAborted'],TextDecoder:['decode','encoding','fatal','ignoreBOM'],Headers:['append','delete','set'],Request:['arrayBuffer','blob','bytes','json','text','body','bodyUsed','url','method','headers','signal'],Response:['arrayBuffer','blob','bytes','json','text','body','bodyUsed','headers','ok','url','status','statusText','redirected','type']};
 for(const [name,members] of Object.entries(interfaces)) {
  const ctor=globalThis[name],entry={constructor:{name:ctor.name,length:ctor.length},members:{}};
  for(const key of members) {
   const d=Object.getOwnPropertyDescriptor(ctor.prototype,key);
   if(!d){entry.members[key]={missing:true};continue}
   if(typeof d.value==='function')entry.members[key]=shape(d.value);
   if(d.get)entry.members['get '+key]=shape(d.get);
   if(d.set)entry.members['set '+key]=shape(d.set);
  }
  result[name]=entry;
 }
 const target=new EventTarget(),seen=[],add=EventTarget.prototype.addEventListener,remove=EventTarget.prototype.removeEventListener;
 const callback=function(event){seen.push(this===target,event.type)};
 const oldApply=Reflect.apply;
 try {Reflect.apply=()=>{throw new Error('user replacement must not intercept a platform operation')};add.call(target,'owned',callback);target.dispatchEvent(new Event('owned'));remove.call(target,'owned',callback);target.dispatchEvent(new Event('owned'))}
 finally{Reflect.apply=oldApply}
 result.receiverAndCapturedApply=seen;
 const getBody=Object.getOwnPropertyDescriptor(Request.prototype,'body').get;
 try{getBody.call({});result.invalidReceiver='accepted'}catch(error){result.invalidReceiver=error.name}
 result.intrinsics={valuesName:Array.prototype.values.name,valuesLength:Array.prototype.values.length,iteratorIdentity:DOMStringList.prototype[Symbol.iterator]===Array.prototype.values};
 return result;
})()
