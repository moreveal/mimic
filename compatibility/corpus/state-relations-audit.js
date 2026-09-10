(async () => {
  const out={}, run=async(name,fn)=>{try{out[name]=await fn()}catch(e){out[name]={exception:e.name,message:e.message}}};
  await run('historyStructuredClone',()=>{
    const input={nested:{value:1}};history.pushState(input,'');const saved=history.state;
    input.nested.value=2;
    const value={inputIdentity:saved===input,nestedIdentity:saved.nested===input.nested,value:history.state.nested.value,repeatedIdentity:history.state===saved};
    try{history.pushState(()=>{},'');value.uncloneable='accepted'}catch(e){value.uncloneable=e.name}
    return value;
  });
  await run('corsWithoutAllowOrigin',async()=>{
    const url=new URL('/echo',location.href);url.hostname='localhost';
    try{const r=await fetch(url);return {resolved:true,status:r.status,type:r.type}}catch(e){return{resolved:false,exception:e.name}}
  });
  await run('remotePrototypeMutation',()=>{
    const frame=document.createElement('iframe');document.body.append(frame);
    try{const w=frame.contentWindow,a=w.document.all,before=Object.getPrototypeOf(a);
      w.eval('Object.setPrototypeOf(document.all,{marker:42})');
      return {changed:Object.getPrototypeOf(a)!==before,marker:Object.getPrototypeOf(a).marker===42,read:a.marker===42};
    }finally{frame.remove()}
  });
  await run('borrowedHTMLAllMethod',()=>{
    const frame=document.createElement('iframe');document.body.append(frame);
    try{return HTMLAllCollection.prototype.item.call(frame.contentDocument.all,0)===frame.contentDocument.documentElement}
    finally{frame.remove()}
  });
  await run('srcdocNavigation',async()=>{
    const frame=document.createElement('iframe');document.body.append(frame);
    try{const loaded=new Promise(resolve=>frame.onload=resolve);frame.srcdoc='<p id="inside">srcdoc</p>';
      await Promise.race([loaded,new Promise(resolve=>setTimeout(resolve,500))]);
      return{content:!!frame.contentDocument.getElementById('inside')};
    }finally{frame.remove()}
  });
  return out;
})()
