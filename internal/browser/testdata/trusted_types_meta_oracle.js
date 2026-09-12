(() => {
  const out={},a=(n,f)=>{try{out[n]=f()}catch(e){out[n]=e.name}};
  function frame(){const f=document.createElement('iframe');document.body.appendChild(f);return f.contentWindow}
  function meta(w,parent,content){const m=w.document.createElement('meta');m.httpEquiv='Content-Security-Policy';if(content!==undefined)m.content=content;parent.appendChild(m);return m}
  let w=frame(),m=meta(w,w.document.head);m.content="require-trusted-types-for 'script'";a('lateContent',()=>w.eval('42'));
  w=frame();m=meta(w,w.document.head,'');m.content="require-trusted-types-for 'script'";a('changedContent',()=>w.eval('42'));
  w=frame();m=meta(w,w.document.body,"require-trusted-types-for 'script'");a('bodyIgnored',()=>w.eval('42'));w.document.head.appendChild(m);a('movedToHead',()=>w.eval('42'));
  w=frame();const s=w.document.createElement('script');s.src='old';const saved=Object.getOwnPropertyDescriptor(w.Element.prototype,'innerHTML').set;meta(w,w.document.head,"require-trusted-types-for 'script'");a('savedSetter',()=>{saved.call(w.document.body,'x');return true});a('existingAttribute',()=>s.getAttribute('src'));
  w=frame();meta(w,w.document.head,"trusted-types *");for(const n of ['', 'a b', 'allowed', '*', 'a.b#c=1/2@x-%_'])a('wildcard:'+n,()=>w.trustedTypes.createPolicy(n,{}).name);
  return out;
})()
