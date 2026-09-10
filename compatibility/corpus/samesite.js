(async()=>{
  const read=async()=>({document:document.cookie,wire:(await(await fetch('/echo')).json()).headers.cookie||''});
  document.cookie='ss_none=1; Secure; SameSite=None; Path=/';
  document.cookie='ss_lax=1; Secure; SameSite=Lax; Path=/';
  document.cookie='ss_strict=1; Secure; SameSite=Strict; Path=/';
  document.cookie='ss_default=1; Secure; Path=/';
  document.cookie='ss_insecure_none=1; SameSite=None; Path=/';
  const first=await read();
  const own=location.origin, other='http://localhost:'+location.port;
  const inner=`const before=document.cookie;document.cookie='ss_nested_lax=1; Secure; SameSite=Lax; Path=/';return {before,after:document.cookie,wire:(await(await fetch('/echo')).json()).headers.cookie||''}`;
  const middle=`const f=document.createElement('iframe');const answer=new Promise(resolve=>{onmessage=e=>{if(e.source===f.contentWindow)resolve(e.data)}});f.src=${JSON.stringify(own+'/frame?id=samesite-inner&code='+encodeURIComponent(inner))};document.body.append(f);return await answer`;
  const f=document.createElement('iframe');
  const nested=new Promise(resolve=>{const listener=e=>{if(e.source===f.contentWindow){removeEventListener('message',listener);resolve(e.data)}};addEventListener('message',listener)});
  f.src=other+'/frame?id=samesite-middle&code='+encodeURIComponent(middle);document.body.append(f);
  try{return {first,nested:await nested,after:await read()}}finally{f.remove()}
})()
