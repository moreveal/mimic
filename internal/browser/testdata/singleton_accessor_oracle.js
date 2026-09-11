(() => {
  const out={},frame=document.createElement('iframe');document.body.appendChild(frame);
  const cap=fn=>{try{const v=fn();return {ok:true,type:typeof v}}catch(e){return {ok:false,name:e.name,local:e instanceof TypeError}}};
  try {
    const child=frame.contentWindow;
    for(const [name,property,keys] of [
      ['Navigator','navigator',['userAgent','language','languages','plugins','mimeTypes','hardwareConcurrency','cookieEnabled']],
      ['Screen','screen',['width','height','availLeft','orientation','onchange']],
      ['History','history',['length','state','scrollRestoration']]
    ]){
      const prototype=globalThis[name].prototype,instance=globalThis[property],foreign=child[property];
      for(const key of keys){
        const get=Object.getOwnPropertyDescriptor(prototype,key).get;
        const row={name:get.name,length:get.length,native:Function.prototype.toString.call(get)};
        row.null=cap(()=>get.call(null));row.forged=cap(()=>get.call(Object.create(prototype)));
        row.proxy=cap(()=>get.call(new Proxy(instance,{})));
        row.other=cap(()=>get.call(property==='history'?screen:history));
        row.borrowed=cap(()=>get.call(foreign));
        const original=Object.getPrototypeOf(instance);
        Object.setPrototypeOf(instance,null);try{row.detachedPrototype=cap(()=>get.call(instance))}finally{Object.setPrototypeOf(instance,original)}
        Object.defineProperty(instance,key,{value:'shadow',configurable:true});
        try{row.ignoresOwnShadow=get.call(instance)!=='shadow'}finally{delete instance[key]}
        out[name+'.'+key]=row;
      }
    }
    history.replaceState({owner:'parent'},'');child.eval("history.replaceState({owner:'child'},'')");
    const getState=Object.getOwnPropertyDescriptor(History.prototype,'state').get;
    out.historyOwner={parent:getState.call(history).owner,child:getState.call(child.history).owner,reverse:Object.getOwnPropertyDescriptor(child.History.prototype,'state').get.call(history).owner};
    const getPlugins=Object.getOwnPropertyDescriptor(Navigator.prototype,'plugins').get;
    out.pluginsOwner={same:getPlugins.call(child.navigator)===child.navigator.plugins,foreign:getPlugins.call(child.navigator) instanceof child.PluginArray};
    const restoration=Object.getOwnPropertyDescriptor(History.prototype,'scrollRestoration');
    let conversions=0;const value={toString(){conversions++;return 'manual'}};
    out.setterWrong=cap(()=>restoration.set.call({},value));out.wrongConversions=conversions;
    out.setterBorrowed=cap(()=>restoration.set.call(child.history,value));out.validConversions=conversions;
    out.restoration={child:cap(()=>restoration.get.call(child.history)),parent:restoration.get.call(history),childManual:cap(()=>restoration.get.call(child.history)==='manual')};
    out.setterSymbol=cap(()=>restoration.set.call(history,Symbol('value')));
    return out;
  }finally{frame.remove()}
})()
