(() => {
  const source=fn=>Function.prototype.toString.call(fn),result={};
  const interfaces={
    AbortController:['abort','signal'],
    AbortSignal:['aborted','reason','onabort','throwIfAborted'],
    TextDecoder:['decode','encoding','fatal','ignoreBOM'],
    TextEncoder:['encode','encodeInto','encoding'],
    Headers:['append','delete','set'],
    Request:['bytes','arrayBuffer','text','json','blob'],
    Response:['bytes','arrayBuffer','text','json','blob'],
    ReadableStream:['locked','cancel','getReader','pipeThrough','pipeTo','tee','values',Symbol.asyncIterator],
    WritableStream:['locked','abort','close','getWriter'],
    TransformStream:['readable','writable'],
    DOMStringList:['item','contains',Symbol.iterator]
  };
  for(const [name,members] of Object.entries(interfaces)) {
    const ctor=globalThis[name],methods={};
    for(const key of members) {
      const d=Object.getOwnPropertyDescriptor(ctor.prototype,key);
      if(!d){methods[String(key)]={missing:true};continue}
      if(typeof d.value==='function')methods[String(key)]=source(d.value);
      if(d.get)methods['get '+String(key)]=source(d.get);
      if(d.set)methods['set '+String(key)]=source(d.set);
    }
    result[name]={source:source(ctor),methods};
  }
  result.statics={};
  for(const [name,members] of [['AbortSignal',['abort','any','timeout']],['Response',['error','json','redirect']]])
    for(const member of members)result.statics[name+'.'+member]=source(globalThis[name][member]);
  result.bodyNames={};
  for(const name of ['Request','Response'])for(const key of interfaces[name])result.bodyNames[name+'.'+key]=globalThis[name].prototype[key].name;
  result.intrinsics={arrayValues:source(Array.prototype.values),arrayIterator:source(Array.prototype[Symbol.iterator]),mapEntries:source(Map.prototype.entries)};
  result.aliasIdentity=DOMStringList.prototype[Symbol.iterator]===Array.prototype.values;
  const user=function visibleUserFunction(){return 42};result.userSource=source(user);
  return result;
})()
