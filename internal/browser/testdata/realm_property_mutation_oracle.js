(() => {
  const out={},frame=document.createElement('iframe');document.body.appendChild(frame);
  try {
    const child=frame.contentWindow,object=new child.Object(),symbol=Symbol('key'),value={marker:1};
    out.define=Reflect.defineProperty(object,'value',{value,writable:true,enumerable:true,configurable:true});
    out.identity=object.value===value;
    out.symbol=Reflect.defineProperty(object,symbol,{value,configurable:true})&&object[symbol]===value;
    out.remove=Reflect.deleteProperty(object,'value')&&!('value' in object);
    out.removeMissing=Reflect.deleteProperty(object,'missing');
    Reflect.defineProperty(object,'fixed',{value,writable:true,configurable:false});
    out.firstDescriptor=Object.getOwnPropertyDescriptor(object,'fixed').value===value;
    const replacement={marker:2};out.lock=Reflect.defineProperty(object,'fixed',{value:replacement,writable:false});
    out.lockedIdentity=object.fixed===replacement;
    out.reject={define:Reflect.defineProperty(object,'fixed',{value:0}),remove:Reflect.deleteProperty(object,'fixed')};
    const getter=function(){return this===object?value:null};
    out.accessor=Reflect.defineProperty(object,'accessor',{get:getter,configurable:false})&&object.accessor===value&&Object.getOwnPropertyDescriptor(object,'accessor').get===getter;
    const marker={},proxy=child.eval('(marker=>new Proxy({}, {defineProperty(){throw marker},deleteProperty(){throw marker}}))')(marker);
    try{Reflect.defineProperty(proxy,'x',{value:1})}catch(e){out.defineException=e===marker}
    try{Reflect.deleteProperty(proxy,'x')}catch(e){out.deleteException=e===marker}
    const array=new child.Array(1,2,3);out.array=Reflect.defineProperty(array,'length',{value:1,writable:false})&&array.length===1&&array[1]===undefined;
    frame.remove();out.retained=Reflect.defineProperty(object,'late',{value:17,configurable:true})&&object.late===17;
    return out;
  }finally{frame.remove()}
})()
