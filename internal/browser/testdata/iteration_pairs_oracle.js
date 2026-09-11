(() => {const all=(() => {
 const out={},cap=(k,f)=>{try{out[k]={value:f()}}catch(e){out[k]={error:e.name}}};
 for(const name of ['Headers','URLSearchParams']){
   const C=globalThis[name],make=()=>new C([['a','1'],['c','3']]);
   cap(name+'.shape',()=>{const i=make().entries(),p=Object.getPrototypeOf(i),n=Object.getOwnPropertyDescriptor(p,'next'),t=Object.getOwnPropertyDescriptor(p,Symbol.toStringTag);return {own:Reflect.ownKeys(i).map(String),keys:Reflect.ownKeys(p).map(String),tag:Object.prototype.toString.call(i),next:{name:n.value.name,length:n.value.length,enumerable:n.enumerable,configurable:n.configurable,writable:n.writable},parentIsIterator:Object.getPrototypeOf(p)===Object.getPrototypeOf(Object.getPrototypeOf([][Symbol.iterator]())),tagDescriptor:t&&[t.value,t.enumerable,t.configurable,t.writable],alias:C.prototype.entries===C.prototype[Symbol.iterator],self:i[Symbol.iterator]()===i}});
   for(const kind of ['entries','keys','values']){
     cap(name+'.live.'+kind,()=>{const h=make(),i=h[kind]();const a=i.next();h.set('b','2');h.set('c','4');return[a,i.next(),i.next(),i.next()];});
     cap(name+'.beforeFirst.'+kind,()=>{const h=make(),i=h[kind]();h.delete('a');h.append('z','9');return[...i];});
     cap(name+'.afterDone.'+kind,()=>{const h=new C(),i=h[kind]();const a=i.next();h.append('a','1');return[a,i.next()];});
     cap(name+'.borrow.'+kind,()=>{const h=make();Object.setPrototypeOf(h,null);return [...C.prototype[kind].call(h)];});
   }
   cap(name+'.forEach',()=>{const h=make(),seen=[];h[Symbol.iterator]=()=>{throw Error('override')};h.forEach((v,k,o)=>{seen.push([k,v,o===h]);if(k==='a'){h.append('b','2');h.set('c','4')}});return seen;});
   cap(name+'.nextBrand',()=>{const p=Object.getPrototypeOf(make().entries());return p.next.call({});});
 }
 for(const value of ['x=%E0%A4','x=%FF','x=%zz','x=%EF%BB%BF','x=\ud800','x=a+b','x=%C0%AF'])cap('form.'+JSON.stringify(value),()=>[...new URLSearchParams(value)]);
 cap('form.serializeLone',()=>new URLSearchParams([['\ud800','\udfff']]).toString());
 for(const method of ['encode','encodeInto']){
   cap('encoder.symbol.'+method,()=>{const e=new TextEncoder();return method==='encode'?[...e.encode(Symbol())]:e.encodeInto(Symbol(),new Uint8Array(10));});
   let log=[];cap('encoder.order.'+method,()=>{const e=new TextEncoder(),s={toString(){log.push('source');return 'x'}};return method==='encode'?[...e.encode.call({},s)]:e.encodeInto(s,{});});out['encoder.log.'+method]=log;
 }
 cap('encoder.arity',()=>new TextEncoder().encodeInto());
 return out;
})()
;return Object.fromEntries(Object.entries(all).filter(([key])=>new RegExp("^(Headers|URLSearchParams)\\.").test(key)));})()