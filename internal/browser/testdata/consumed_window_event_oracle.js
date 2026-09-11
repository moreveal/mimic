(async()=>{
 const log=[];globalThis.__windowEventOracleLog=log;
 const frame=document.createElement('iframe');frame.srcdoc='<!doctype html><body>child';
 await new Promise(resolve=>{frame.onload=resolve;document.body.appendChild(frame)});
 await new Promise(resolve=>setTimeout(resolve,0));
 const w=frame.contentWindow;
 const safe=f=>{try{return{ok:true,value:f()}}catch(e){return{ok:false,name:e.name,message:e.message}}};
 const summary=value=>value===undefined?'undefined':value===null?'null':typeof value==='object'?value.type:typeof value;
 const record=(label,e)=>log.push({label,local:summary(window.event),localSame:window.event===e,child:summary(w.event),childSame:w.event===e});
 const original=Object.getOwnPropertyDescriptor(window,'event');
 const descriptor={exists:!!original,enumerable:original?.enumerable,configurable:original?.configurable,get:typeof original?.get,set:typeof original?.set};
 record('initial',null);
 window.addEventListener('inner',e=>record('nested-inner',e),{once:true});
 window.addEventListener('outer',e=>{record('outer-enter',e);window.dispatchEvent(new Event('inner'));record('outer-after-inner',e);queueMicrotask(()=>record('microtask-after-outer',e))},{once:true});
 window.dispatchEvent(new Event('outer'));record('after-dispatch',null);await Promise.resolve();
 const childCallback=w.eval(`(function childCallback(e){parent.__windowEventOracleLog.push({label:'child-callback-parent-target',local:event===undefined?'undefined':event?.type,localSame:event===e,parent:parent.event===undefined?'undefined':parent.event?.type,parentSame:parent.event===e})})`);
 window.addEventListener('childcallback',childCallback,{once:true});window.dispatchEvent(new Event('childcallback'));record('after-child-callback',null);
 w.addEventListener('parentcallback',function(e){record('parent-callback-child-target',e)},{once:true});w.dispatchEvent(new w.Event('parentcallback'));record('after-parent-callback',null);
 const object={handleEvent:w.eval(`(function(e){parent.__windowEventOracleLog.push({label:'child-function-parent-listener-object',local:event===undefined?'undefined':event?.type,localSame:event===e,parent:parent.event===undefined?'undefined':parent.event?.type,parentSame:parent.event===e})})`)};
 window.addEventListener('objectcallback',object,{once:true});window.dispatchEvent(new Event('objectcallback'));record('after-object-callback',null);
 const host=document.createElement('div');document.body.appendChild(host);const shadow=host.attachShadow({mode:'open'}),inner=document.createElement('span');shadow.appendChild(inner);
 window.addEventListener('shadowtest',e=>record('shadow-window-capture',e),{capture:true,once:true});
 shadow.addEventListener('shadowtest',e=>record('shadow-root',e),{once:true});
 inner.addEventListener('shadowtest',e=>record('shadow-inner',e),{once:true});
 host.addEventListener('shadowtest',e=>record('shadow-host',e),{once:true});
 inner.dispatchEvent(new Event('shadowtest',{composed:true,bubbles:true}));record('after-shadow',null);
 const trusted=await new Promise(resolve=>{window.addEventListener('message',function listener(e){if(e.data!=='window-event-oracle')return;window.removeEventListener('message',listener);record('trusted-message',e);resolve(e.isTrusted)});window.postMessage('window-event-oracle','*')});
 record('after-trusted-message',null);
 await new Promise(resolve=>setTimeout(resolve,0));record('later-task-after-message',null);
 let replaced;
 try{window.event='sentinel';const d=Object.getOwnPropertyDescriptor(window,'event');let inDispatch;window.addEventListener('replacement',()=>{inDispatch=window.event},{once:true});window.dispatchEvent(new Event('replacement'));replaced={afterAssignment:window.event,inDispatch,descriptor:{kind:'value'in d?'data':'accessor',enumerable:d.enumerable,configurable:d.configurable,writable:d.writable}}}finally{Object.defineProperty(window,'event',original)}
 frame.remove();host.remove();delete globalThis.__windowEventOracleLog;
 return{descriptor,log,trusted,replaced};
})()
