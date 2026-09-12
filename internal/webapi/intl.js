    // V8 ships Chrome's ECMA-402 implementation. Keep it intact and project
    // only the canonical browser environment into omitted locale/time-zone
    // arguments; host OS defaults are not browser state and must not leak in.
    const canonicalLocales=Intl.getCanonicalLocales;
    for(const name of ['Collator','NumberFormat','DateTimeFormat','PluralRules','RelativeTimeFormat','ListFormat','DisplayNames','Segmenter']){
      const Native=globalThis.Intl[name];
      if(typeof Native!=='function')continue;
      const normalize=args=>{
        args=Array.from(args);
        // Add the context fallback after canonicalization, including [] and
        // well-formed but unavailable languages. Never fall back to OS locale.
        args[0]=[...canonicalLocales(args[0]),intlEnvironment.locale];
        if(name==='DateTimeFormat'){
          if(args[1]===null)throw new TypeError('Cannot convert null to object');
          args[1]=new Proxy(args[1]===undefined?{}:Object(args[1]),{get(target,key){const value=Reflect.get(target,key,target);return key==='timeZone'&&value===undefined?intlEnvironment.timeZone:value}});
        }
        return args;
      };
      const wrapped=new Proxy(Native,{apply(target,self,args){return Reflect.apply(target,self,normalize(args))},construct(target,args,newTarget){return Reflect.construct(target,normalize(args),newTarget)}});
      globalThis.Intl[name]=wrapped;markNative(wrapped,name);
      Object.defineProperty(Native.prototype,'constructor',{value:wrapped,writable:true,configurable:true});
    }
