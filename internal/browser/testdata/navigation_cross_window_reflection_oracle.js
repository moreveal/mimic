(async()=>{
  const frame=document.createElement('iframe');
  const target=new URL('/cross-window-reflection',location.href);target.hostname='localhost';
  await new Promise(resolve=>{frame.onload=resolve;frame.src=target.href;document.body.appendChild(frame);});
  const w=frame.contentWindow;
  const attempt=fn=>{try{const value=fn();return value===undefined?'undefined':value;}catch(error){return {error:error.name};}};
  const describe=d=>d===undefined?'undefined':{configurable:d.configurable,enumerable:d.enumerable,writable:d.writable,value:d.value===undefined?'undefined':d.value};
  const result={
    tag:attempt(()=>Object.prototype.toString.call(w)),
    toStringTag:attempt(()=>w[Symbol.toStringTag]),
    tagDescriptor:attempt(()=>describe(Object.getOwnPropertyDescriptor(w,Symbol.toStringTag))),
    ownKeys:attempt(()=>Reflect.ownKeys(w).map(k=>typeof k==='symbol'?String(k):k)),
    iterator:attempt(()=>w[Symbol.iterator]),
    hasInstance:attempt(()=>w[Symbol.hasInstance]),
    isConcatSpreadable:attempt(()=>w[Symbol.isConcatSpreadable]),
    then:attempt(()=>w.then),
    uniqueSymbol:attempt(()=>w[Symbol('unique')]),
    toPrimitive:attempt(()=>w[Symbol.toPrimitive]),
  };
  frame.remove();return result;
})()
