(() => {
  const out={}, f=document.createElement('iframe');document.body.appendChild(f);
  const methods=[[Document.prototype,'querySelector',document],[Document.prototype,'querySelectorAll',document],[Document.prototype,'getElementById',document],[Element.prototype,'querySelector',document.body],[Element.prototype,'querySelectorAll',document.body],[Element.prototype,'matches',document.body],[Element.prototype,'closest',document.body]];
  const attempt=fn=>{try{const v=fn();return {ok:true,value:v===null?'null':typeof v}}catch(e){return {ok:false,name:e.name,local:e instanceof TypeError}}};
  try {
    for(const [proto,name,receiver] of methods){
      const fn=proto[name],key=proto===Document.prototype?'Document.':'Element.';
      let conversions=0;const argument={toString(){conversions++;return 'body'}};
      const row={};
      row.invalid=attempt(()=>fn.call({},argument));row.invalidConversions=conversions;
      row.nullReceiver=attempt(()=>fn.call(null,argument));row.nullConversions=conversions;
      row.missing=attempt(()=>fn.call(receiver));row.symbol=attempt(()=>fn.call(receiver,Symbol('selector')));
      row.converted=attempt(()=>fn.call(receiver,argument));row.totalConversions=conversions;
      const childReceiver=proto===Document.prototype?f.contentDocument:f.contentDocument.body;
      row.borrowed=attempt(()=>fn.call(childReceiver,'body'));
      row.forged=attempt(()=>fn.call(Object.create(proto),'body'));
      row.constructible=attempt(()=>Reflect.construct(function(){},[],fn));
      out[key+name]=row;
    }
    const marker={},arg={toString(){throw marker}};
    try{document.querySelector(arg)}catch(e){out.originalException=e===marker}
    return out;
  } finally {f.remove();}
})()
