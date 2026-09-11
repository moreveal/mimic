(async () => {
 const out={},cap=f=>{try{const v=f();return {value:v===undefined?'undefined':v}}catch(e){return {error:e.name}}};
 for(const [kind,instance,names] of [['History',history,['pushState','replaceState','back','forward','go']],['Storage',localStorage,['getItem','setItem','removeItem','key','clear']]]){
  for(const name of names){const fn=globalThis[kind].prototype[name];let trace=[];const arg={toString(){trace.push('string');return 'probe'},valueOf(){trace.push('number');return 999999}};
   out[kind+'.'+name]={invalid:cap(()=>fn.call({},arg,arg,arg)),trace:trace.slice(),missing:fn.length?cap(()=>fn.call(instance)):null};
  }
 }
 for(const name of ['pushState','replaceState']){
  const trace=[],title={toString(){trace.push('title');return ''}},url={toString(){trace.push('url');return location.href}};
  const state={get x(){trace.push('state');return 1}};
  out[name+'.order']={result:cap(()=>history[name](state,title,url)),trace};
  out[name+'.titleSymbol']=cap(()=>history[name]({},Symbol()));
  out[name+'.urlSymbol']=cap(()=>history[name]({},'',Symbol()));
 }
 out.goBigInt=cap(()=>history.go(1n));out.keyBigInt=cap(()=>localStorage.key(1n));
 for(const name of ['getItem','removeItem','setItem'])out[name+'.symbol']=cap(()=>localStorage[name](Symbol(),'value'));
 localStorage.clear();sessionStorage.clear();
 const trace=[];out.setOrder={result:cap(()=>localStorage.setItem({toString(){trace.push('key');return 'key'}},{toString(){trace.push('value');return 'value'}})),trace};
 out.stored=localStorage.getItem('key');out.wrappedKey=localStorage.key(4294967296);
 const frame=document.createElement('iframe');document.body.appendChild(frame);const child=frame.contentWindow;
 Storage.prototype.setItem.call(child.sessionStorage,'owner','child');
 out.owner={session:child.sessionStorage.getItem('owner'),local:localStorage.getItem('owner'),borrowed:Storage.prototype.getItem.call(child.sessionStorage,'owner'),length:Object.getOwnPropertyDescriptor(Storage.prototype,'length').get.call(child.sessionStorage)};
 out.childHistory=cap(()=>History.prototype.replaceState.call(child.history,7,''));out.childState=child.history.state;out.parentState=history.state;
 out.detachedPrototype=cap(()=>{const storage=localStorage;const proto=Object.getPrototypeOf(storage);try{Object.setPrototypeOf(storage,null);return Storage.prototype.getItem.call(storage,'key')}finally{Object.setPrototypeOf(storage,proto)}});
 localStorage.clear();sessionStorage.clear();frame.remove();return out;
})()
