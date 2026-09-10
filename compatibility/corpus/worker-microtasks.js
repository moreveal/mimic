(async()=>{
  const source=`
onmessage=()=>{
 const out={order:[],invalid:[],errors:[],handler:[]};
 if(typeof queueMicrotask!=='function'){postMessage({missing:'queueMicrotask'});return}
 for(const args of [[],[undefined],[null],[0],[{}]]){try{queueMicrotask(...args);out.invalid.push('accepted')}catch(e){out.invalid.push([e.name,e.message])}}
 for(const receiver of [undefined,null,{},self]){try{queueMicrotask.call(receiver,()=>{});out.invalid.push('receiver accepted')}catch(e){out.invalid.push([e.name,e.message])}}
 const sentinel=new Error('microtask sentinel');
 onerror=function(message,filename,line,column,error){out.handler.push({message,identity:error===sentinel,thisGlobal:this===self});return true};
 addEventListener('error',e=>{out.errors.push({message:e.message,identity:e.error===sentinel,trusted:e.isTrusted,cancelable:e.cancelable,prevented:e.defaultPrevented,filename:e.filename===location.href,line:e.lineno>0,column:e.colno>0})});
 let thenRead=0;
 queueMicrotask(function(){'use strict';out.thisGlobal=this===self;out.thisUndefined=this===undefined;out.order.push('microtask');queueMicrotask(()=>out.order.push('nested'));return {get then(){thenRead++;return ()=>{}}}});
 Promise.resolve().then(()=>out.order.push('promise'));
 queueMicrotask(()=>{throw sentinel});
 queueMicrotask(()=>out.order.push('after-error'));
 const saved=Promise.resolve;Promise.resolve=()=>{throw Error('user hook')};
 try{queueMicrotask(()=>out.order.push('intrinsic'));out.poisonedPromise='accepted'}catch(e){out.poisonedPromise=e.message}
 Promise.resolve=saved;
 out.order.push('sync');
 setTimeout(()=>{out.thenRead=thenRead;postMessage(out)},20);
};`;
  const url=URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
  const worker=new Worker(url);
  try{return await new Promise(resolve=>{worker.onmessage=e=>resolve(e.data);worker.onerror=e=>resolve({uncaught:e.message});worker.postMessage(null)})}
  finally{worker.terminate();URL.revokeObjectURL(url)}
})()
