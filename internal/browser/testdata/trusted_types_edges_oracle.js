(() => {
  const out={},calls=[],attempt=(name,fn)=>{calls.length=0;try{const value=fn();out[name]={value:value===undefined?'undefined':value,calls:calls.slice()}}catch(e){out[name]={error:e.name,calls:calls.slice()}}};
  const p=trustedTypes.createPolicy('allowed',{createHTML:s=>s,createScript:s=>s,createScriptURL:s=>s});
  const box=document.querySelector('#box');let behavior='missing',nested=false;
  const methods={createHTML:function(...args){calls.push(['html',this===undefined,...args]);if(behavior==='reentrant'&&!nested){nested=true;try{box.innerHTML='nested'}catch(e){calls.push(['nested',e.name])}finally{nested=false}}if(behavior==='coercion')return{toString(){calls.push(['result']);return 'converted'}};if(behavior==='symbol')return Symbol('x');return behavior==='null'?null:args[0]},createScript:function(...args){calls.push(['script',...args]);return behavior==='rewrite'?args[0].replace('42','43'):args[0]}};
  // Capture callback dictionary once. Default enforcement must not invoke the
  // mutable public createHTML/createScript methods on the policy object.
  const options={createHTML:methods.createHTML};const d=trustedTypes.createPolicy('default',options);
  d.createHTML=()=>{throw new Error('public method')};
  attempt('missingScriptDefault',()=>eval('42'));
  attempt('missingURLDefault',()=>{document.createElement('script').src='x'});
  attempt('defaultReceiver',()=>{box.innerHTML='x'});
  behavior='reentrant';attempt('reentrant',()=>{box.innerHTML='outer';return box.innerHTML});
  behavior='coercion';attempt('resultConversion',()=>{box.innerHTML='x';return box.innerHTML});
  behavior='symbol';attempt('symbolResult',()=>{box.innerHTML='x'});
  behavior='null';attempt('nullResult',()=>{box.innerHTML='x'});
  behavior='normal';
  attempt('coercionOrder',()=>{const x={toString(){calls.push(['input']);return 'x'}};box.insertAdjacentHTML({toString(){calls.push(['position']);return 'beforeend'}},x)});
  attempt('receiverBeforeCoercion',()=>Object.getOwnPropertyDescriptor(Element.prototype,'innerHTML').set.call({}, {toString(){calls.push(['coerce']);return 'x'}}));
  attempt('trustedTampered',()=>{const v=p.createHTML('<b>safe</b>');v.toString=()=>{throw Error('wrong')};Object.setPrototypeOf(v,null);box.innerHTML=v;return box.innerHTML});
  attempt('proxyTrusted',()=>{box.innerHTML=new Proxy(p.createHTML('x'),{})});
  attempt('cloneAttributes',()=>{const s=document.createElement('script');s.src=p.createScriptURL('/script.js');return s.cloneNode().getAttribute('src')});
  attempt('attributeNSNode',()=>{const a=document.createAttribute('src');a.value='/script.js';document.createElement('script').setAttributeNodeNS(a)});
  attempt('emptyWrite',()=>document.write());
  return out;
})()
