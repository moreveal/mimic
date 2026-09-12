(() => {
  const out={},calls=[],a=(n,f)=>{calls.length=0;try{const v=f();out[n]={value:v===undefined?'undefined':v,calls:calls.slice()}}catch(e){out[n]={error:e.name,calls:calls.slice()}}};
  const p=trustedTypes.createPolicy('allowed',{createHTML:s=>s,createScript:s=>s,createScriptURL:s=>s});
  trustedTypes.createPolicy('default',{createHTML:(...x)=>{calls.push(x);return x[0]},createScript:(...x)=>{calls.push(x);return x[0]},createScriptURL:(...x)=>{calls.push(x);return x[0]}});
  a('svgHrefBaseVal',()=>{document.createElementNS('http://www.w3.org/2000/svg','script').href.baseVal='/script.js'});
  a('svgHrefTrustedBaseVal',()=>{document.createElementNS('http://www.w3.org/2000/svg','script').href.baseVal=p.createScriptURL('/script.js')});
  a('svgImageHrefBaseVal',()=>{document.createElementNS('http://www.w3.org/2000/svg','image').href.baseVal='/script.js'});
  a('borrowScriptSrc',()=>Object.getOwnPropertyDescriptor(HTMLScriptElement.prototype,'src').set.call(document.createElement('div'),'/script.js'));
  a('borrowScriptText',()=>Object.getOwnPropertyDescriptor(HTMLScriptElement.prototype,'text').set.call(document.createElement('div'),'42'));
  for(const prop of ['value','nodeValue','textContent'])a('attr:'+prop,()=>{const s=document.createElement('script');s.src=p.createScriptURL('');s.getAttributeNode('src')[prop]=p.createScriptURL('/script.js')});
  a('attributeNodeNS',()=>{const s=document.createElement('script'),attr=document.createAttribute('src');attr.value='/script.js';s.setAttributeNodeNS(attr)});
  a('namedItem',()=>{const s=document.createElement('script'),attr=document.createAttribute('src');attr.value='/script.js';s.attributes.setNamedItem(attr)});
  a('scriptMutationObserver',()=>{const s=document.createElement('script'),observer=new MutationObserver(()=>{});observer.observe(s,{childList:true});s.text=p.createScript('42');return observer.takeRecords().map(x=>[x.type,x.addedNodes.length,x.removedNodes.length])});
  return out;
})()
