(async () => {
  const out={},a=(name,fn)=>{try{const v=fn();out[name]=v===undefined?'undefined':v}catch(e){out[name]=e.name}};
  const w=document.querySelector('#blank').contentWindow;
  const meta=w.document.createElement('meta');meta.httpEquiv='Content-Security-Policy';meta.content="require-trusted-types-for 'script'";
  a('beforeMeta',()=>w.eval('42'));
  w.document.head.appendChild(meta);meta.remove();
  a('afterRemovedMeta',()=>w.eval('42'));
  a('parentUnaffected',()=>eval('42'));
  const policy=w.trustedTypes.createPolicy('default',{createHTML:s=>s,createScript:s=>s});
  a('childDefault',()=>w.eval('42'));
  a('openPreserves',()=>{w.document.open();w.document.close();return w.trustedTypes.defaultPolicy===policy});
  a('openStillRequires',()=>{const s=w.document.createElement('script');s.src='x'});
  const iframe=document.createElement('iframe');iframe.src=location.origin+'/?policy=required';document.body.appendChild(iframe);await new Promise(resolve=>iframe.onload=resolve);
  a('networkRequired',()=>iframe.contentWindow.eval('42'));
  const old=iframe.contentDocument,oldTypes=iframe.contentWindow.trustedTypes;
  iframe.src=location.origin+'/?policy=open';await new Promise(resolve=>iframe.onload=resolve);
  a('networkOpen',()=>iframe.contentWindow.eval('42'));
  a('newFactory',()=>oldTypes!==iframe.contentWindow.trustedTypes);
  a('oldDocument',()=>{old.body.innerHTML='x'});
  const before=document.createElement('iframe');document.body.appendChild(before);
  const m=document.createElement('meta');m.httpEquiv='Content-Security-Policy';m.content="require-trusted-types-for 'script'";document.head.appendChild(m);
  const after=document.createElement('iframe');document.body.appendChild(after);
  a('existingBlankUnaffected',()=>before.contentWindow.eval('42'));
  a('newBlankInherits',()=>after.contentWindow.eval('42'));
  const p=trustedTypes.createPolicy('allowed',{createHTML:s=>s});
  const sd=document.createElement('iframe');sd.srcdoc=p.createHTML('<body>srcdoc');document.body.appendChild(sd);await new Promise(resolve=>sd.onload=resolve);
  a('srcdocInherits',()=>sd.contentWindow.eval('42'));
  return out;
})()
