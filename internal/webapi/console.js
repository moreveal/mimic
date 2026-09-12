// Chrome stores text for callable/Date/RegExp/Error console arguments even with
// no debugger attached. This is separate from Runtime's remote-object preview.
const consoleObject=(()=>{
  const apply=Reflect.apply,slice=Array.prototype.slice,string=String;
  const parseInteger=parseInt,parseFloating=parseFloat,dateTime=Date.prototype.getTime;
  const regexpSource=Object.getOwnPropertyDescriptor(RegExp.prototype,'source').get;
  const ErrorClass=Error;
  const nativeKind=host.consoleValueKind,proxy=host.cloneIsProxy||host.historyCloneIsProxy;
  const classify=value=>{
    if(nativeKind)return nativeKind(value);
    if(proxy&&proxy(value))return '';
    if(typeof value==='function')return 'function';
    try{apply(dateTime,value,[]);return 'date'}catch{}
    try{apply(regexpSource,value,[]);return 'regexp'}catch{}
    // Non-V8 fallback; native V8 classification is the frozen-Chrome authority.
    try{if(value instanceof ErrorClass)return 'error'}catch{}
    return '';
  };
  const argumentsFor=(input,format)=>{
    const args=apply(slice,input,[]);
    if(format&&args.length>1&&typeof args[0]==='string'){
      let argument=1;
      for(let i=0;i<args[0].length-1&&argument<args.length;i++){
        if(args[0][i]!=='%')continue;
        const specifier=args[0][++i];
        if(specifier!=='s'&&specifier!=='d'&&specifier!=='i'&&specifier!=='f'&&specifier!=='o'&&specifier!=='O'&&specifier!=='c')continue;
        const v=args[argument];
        if(specifier==='s')args[argument]=string(v);
        else if(specifier==='d'||specifier==='i'||specifier==='f')args[argument]=typeof v==='symbol'?NaN:(specifier==='f'?parseFloating(v):parseInteger(v));
        argument++;
      }
    }
    return args;
  };
  const emit=(kind,input,format=true)=>{
    if(!input.length)return;
    const values=argumentsFor(input,format),text=[];
    for(let i=0;i<values.length;i++){
      const v=values[i],type=typeof v;
      if(v!==null&&(type==='object'||type==='function')){
        text[i]=type==='function'?'[function]':'[object]';
        if(classify(v))try{text[i]=string(v)}catch{} // Chrome suppresses message-text conversion failures.
      }else text[i]=string(v);
    }
    host.console(kind,text,values);
  };
  const counts=new Map(),timers=new Map(),get=Map.prototype.get,set=Map.prototype.set,del=Map.prototype.delete,has=Map.prototype.has;
  const label=value=>{if(value===undefined)return 'default';if(typeof value==='symbol')throw new TypeError('Cannot convert a Symbol value to a string');return string(value)};
  const count=(value,reset)=>{
    let key='default',failure,failed=false;
    try{key=label(value)}catch(e){failed=true;failure=e}
    if(reset){if(!apply(del,counts,[key]))emit('warning',["Count for '"+key+"' does not exist"],false)}
    else{const n=(apply(get,counts,[key])||0)+1;apply(set,counts,[key,n]);emit('count',[key+': '+n],false)}
    if(failed)throw failure;
  };
  const timer=(value,op,extra)=>{
    let key='default',failure,failed=false;
    try{key=label(value)}catch(e){failed=true;failure=e}
    const present=apply(has,timers,[key]);
    if(op==='start'){
      if(present)emit('warning',["Timer '"+key+"' already exists"],false);
      else apply(set,timers,[key,host.performanceNow()]);
    }else if(!present)emit('warning',["Timer '"+key+"' does not exist"],false);
    else{
      const elapsed=host.performanceNow()-apply(get,timers,[key]);
      emit(op==='end'?'timeEnd':'log',[key+': '+elapsed+' ms',...extra],false);
      if(op==='end')apply(del,timers,[key]);
    }
    if(failed)throw failure;
  };
  // Console is a namespace object, not an instance exposing prototype methods.
  const object=Object.create(Object.create(Object.prototype));
  const methods={
    debug(...a){emit('debug',a)},log(...a){emit('log',a)},info(...a){emit('info',a)},warn(...a){emit('warning',a)},error(...a){emit('error',a)},
    dir(...a){emit('dir',a,false)},dirxml(...a){emit('dirxml',a,false)},table(...a){emit('table',a,false)},trace(...a){emit('trace',a.length?a:['console.trace'])},
    group(...a){emit('startGroup',a.length?a:['console.group'])},groupCollapsed(...a){emit('startGroupCollapsed',a.length?a:['console.groupCollapsed'])},groupEnd(...a){emit('endGroup',a.length?a:['console.groupEnd'],false)},
    count(...a){count(a[0],false)},countReset(...a){count(a[0],true)},
    assert(...a){if(!a[0])emit('assert',a.length>1?apply(slice,a,[1]):['console.assert'])},
    clear(){emit('clear',['console.clear'],false)},time(...a){timer(a[0],'start',[])},timeLog(...a){timer(a[0],'log',apply(slice,a,[1]))},timeEnd(...a){timer(a[0],'end',[])}
  };
  for(const [name,value]of Object.entries(methods)){markNative(value,name);Object.defineProperty(object,name,{value,writable:true,enumerable:true,configurable:true})}
  Object.defineProperty(object,Symbol.toStringTag,{value:'console',configurable:true});
  return object;
})();
