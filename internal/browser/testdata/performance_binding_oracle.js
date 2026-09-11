(() => {
  const out={},frame=document.createElement('iframe');document.body.appendChild(frame);
  try {
    const child=frame.contentWindow, foreign=child.performance;
    performance.mark('parent-only');foreign.mark('child-only');
    const methods=['now','getEntries','getEntriesByType','getEntriesByName'];
    const attempt=fn=>{try{const value=fn();return {ok:true,type:typeof value}}catch(e){return {ok:false,name:e.name,local:e instanceof TypeError}}};
    for(const name of methods){
      const fn=Performance.prototype[name],row={length:fn.length};let conversions=0;
      const arg={toString(){conversions++;return 'mark'}};
      row.forged=attempt(()=>fn.call(Object.create(Performance.prototype),arg));row.invalidConversions=conversions;
      row.null=attempt(()=>fn.call(null,arg));row.missing=attempt(()=>fn.call(performance));
      row.symbol=attempt(()=>fn.call(performance,Symbol('x')));
      row.proxy=attempt(()=>fn.call(new Proxy(performance,{}),arg));
      row.borrowed=attempt(()=>fn.call(foreign,'mark'));out[name]=row;
    }
    const byName=Performance.prototype.getEntriesByName, byType=Performance.prototype.getEntriesByType;
    const entries=byName.call(foreign,'child-only');
    out.owner={name:entries[0]?.name,arrayLocal:Object.getPrototypeOf(entries)===Array.prototype,entryLocal:entries[0] instanceof PerformanceMark,entryForeign:entries[0] instanceof child.PerformanceMark,sameEntry:entries[0]===foreign.getEntriesByName('child-only')[0],parentAbsent:byName.call(foreign,'parent-only').length===0,otherDirection:child.Performance.prototype.getEntriesByName.call(performance,'parent-only')[0]?.name};
    let order=[];const name={toString(){order.push('name');return 'missing'}},type={toString(){order.push('type');return 'mark'}};
    byName.call(performance,name,type);out.conversionOrder=order;
    out.typeSymbol=attempt(()=>byName.call(performance,'missing',Symbol('x')));
    const marker={};try{byType.call(performance,{toString(){throw marker}})}catch(e){out.originalException=e===marker}
    const ownEntries=performance.getEntries;performance.getEntries=()=>{throw Error('public override')};
    out.overrideIgnored=attempt(()=>byName.call(performance,'parent-only'));performance.getEntries=ownEntries;
    return out;
  }finally{frame.remove()}
})()
