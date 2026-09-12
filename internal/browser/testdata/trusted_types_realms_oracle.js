(async () => {
  const out={},calls=[],f=document.querySelector('#blank'),w=f.contentWindow;
  const attempt=(name,fn)=>{calls.length=0;try{const v=fn();out[name]={value:v===undefined?'undefined':v,calls:calls.slice()}}catch(e){out[name]={error:e.name,calls:calls.slice(),parentError:e instanceof TypeError,childError:e instanceof w.TypeError}}};
  const p=trustedTypes.createPolicy('parent',{createHTML:s=>s,createScript:s=>s,createScriptURL:s=>s});
  const cp=w.trustedTypes.createPolicy('child',{createHTML:s=>s,createScript:s=>s,createScriptURL:s=>s});
  attempt('brands',()=>[trustedTypes.isHTML(cp.createHTML('x')),w.trustedTypes.isScript(p.createScript('1')),cp.createHTML('x') instanceof TrustedHTML]);
  attempt('childNoDefault',()=>{w.document.body.innerHTML='x'});
  trustedTypes.createPolicy('default',{createHTML:(...a)=>{calls.push(['parent',...a]);return a[0]},createScript:(...a)=>{calls.push(['parent',...a]);return a[0]}});
  attempt('parentDefaultDoesNotSupplyChild',()=>{w.document.body.innerHTML='x'});
  w.trustedTypes.createPolicy('default',{createHTML:(...a)=>{calls.push(['child',...a]);return a[0]},createScript:(...a)=>{calls.push(['child',...a]);return a[0]}});
  attempt('childDefault',()=>{w.document.body.innerHTML='x'});
  attempt('borrowedSetter',()=>Object.getOwnPropertyDescriptor(Element.prototype,'innerHTML').set.call(w.document.body,'x'));
  attempt('parentTrustedToChild',()=>{w.document.body.innerHTML=p.createHTML('x')});
  attempt('childTrustedToParent',()=>{document.querySelector('#box').innerHTML=cp.createHTML('x')});
  attempt('childEval',()=>w.eval('40+2'));
  attempt('childTrustedEval',()=>w.eval(p.createScript('40+2')));
  attempt('borrowedFactory',()=>TrustedTypePolicyFactory.prototype.isHTML.call(w.trustedTypes,p.createHTML('x')));
  attempt('borrowedValue',()=>TrustedHTML.prototype.toString.call(cp.createHTML('x')));
  attempt('inertDocument',()=>{document.implementation.createHTMLDocument('').body.innerHTML='x'});
  attempt('adoptedNode',()=>{const n=w.document.createElement('div');document.adoptNode(n);n.innerHTML='x'});
  attempt('documentOpenRetainsPolicy',()=>{const policy=w.trustedTypes.defaultPolicy;w.document.open();w.document.close();return w.trustedTypes.defaultPolicy===policy});
  return out;
})()
