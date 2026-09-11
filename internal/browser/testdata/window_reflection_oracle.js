(async()=>{
 const out={},f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;
 const keys=Object.getOwnPropertyNames(w),remote=Array.from(w.Object.getOwnPropertyNames(w));
 out.namesEqual=JSON.stringify(keys)===JSON.stringify(remote);out.requiredKeys=['Object','Array','document','window','location'].every(k=>keys.includes(k));
 const shape=d=>d===undefined?null:{enumerable:d.enumerable,configurable:d.configurable,...('value'in d?{type:typeof d.value,writable:d.writable}:{get:typeof d.get,set:typeof d.set})};
 out.descriptors={};for(const k of ['Object','document','window','location','undefined','NaN','missing'])out.descriptors[k]=shape(Object.getOwnPropertyDescriptor(w,k));
 out.documentIdentity=Object.getOwnPropertyDescriptor(w,'document').get.call(w)===w.document;
 out.objectIdentity=Object.getOwnPropertyDescriptor(w,'Object').value===w.Object;
 out.prototypeIdentity=Object.getPrototypeOf(w)===w.Object.getPrototypeOf(w);out.preventExtensions=Reflect.preventExtensions(w);out.extensible=Reflect.isExtensible(w);out.samePrototype=Reflect.setPrototypeOf(w,Object.getPrototypeOf(w));out.otherPrototype=Reflect.setPrototypeOf(w,{});
 w.eval("globalThis.reads=0;Object.defineProperty(globalThis,'probe',{get(){reads++;return 17},configurable:true,enumerable:true});Object.defineProperty(globalThis,'fixed',{value:23,configurable:false,writable:false});globalThis[Symbol.for('window-reflection')]=31");
 out.accessor=shape(Object.getOwnPropertyDescriptor(w,'probe'));out.readsBefore=w.reads;out.readValue=w.probe;out.readsAfter=w.reads;
 out.fixed=shape(Object.getOwnPropertyDescriptor(w,'fixed'));out.fixedValue=w.fixed;
 out.symbol=Object.getOwnPropertySymbols(w).includes(Symbol.for('window-reflection'));
 out.symbolValue=Object.getOwnPropertyDescriptor(w,Symbol.for('window-reflection')).value;
 out.define=Reflect.defineProperty(w,'fromParent',{value:41,writable:true,enumerable:true,configurable:true});out.definedValue=w.fromParent;out.delete=Reflect.deleteProperty(w,'fromParent');out.deleted=Object.getOwnPropertyDescriptor(w,'fromParent')===undefined;
 const oldDocument=w.document;await new Promise(resolve=>{f.onload=resolve;f.src=location.href+'?window-reflection-nav'});
 out.windowIdentity=f.contentWindow===w;out.newDocument=w.document!==oldDocument;out.newDescriptor=Object.getOwnPropertyDescriptor(w,'document').get.call(w)===w.document;
 out.fixedGone=Object.getOwnPropertyDescriptor(w,'fixed')===undefined;out.keysAfterNav=!Object.getOwnPropertyNames(w).includes('fixed');
 const cross=new URL(location.href);cross.hostname=location.hostname==='127.0.0.1'?'localhost':'127.0.0.1';await new Promise(resolve=>{f.onload=resolve;f.src=cross.href});
 const attempt=fn=>{try{return fn()}catch(e){return e.name}};
 out.crossKeys=attempt(()=>Reflect.ownKeys(w).map(k=>typeof k==='symbol'?String(k):k));
 out.crossDocument=attempt(()=>Object.getOwnPropertyDescriptor(w,'document'));
 out.crossThen=attempt(()=>shape(Object.getOwnPropertyDescriptor(w,'then')));
 out.crossPrototype=attempt(()=>Object.getPrototypeOf(w)===null);
 f.remove();return out;
})()
