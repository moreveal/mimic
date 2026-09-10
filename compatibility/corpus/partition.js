(async () => {
  const other = 'http://localhost:' + location.port;
  const own = location.origin;
  async function frame(origin, id, code) {
    const f = document.createElement('iframe');
    const answer = new Promise((resolve,reject) => {
      const listener = e => {if(e.source===f.contentWindow&&e.data.id===id){removeEventListener('message',listener);e.data.error?reject(Error(JSON.stringify(e.data.error))):resolve(e.data.value);}};
      addEventListener('message', listener);
    });
    f.src=origin+'/frame?id='+id+'&code='+encodeURIComponent(code);document.body.append(f);
    try {return await answer;} finally {f.remove();}
  }
  document.cookie='corpus_p=first; Secure; SameSite=None; Partitioned; Path=/';
  document.cookie='corpus_u=shared; Secure; SameSite=None; Path=/';
  await fetch('/set?cookie='+encodeURIComponent('corpus_h=secret; Secure; HttpOnly; SameSite=None; Partitioned; Path=/'));
  const first = {document:document.cookie, wire:(await(await fetch('/echo')).json()).headers.cookie||''};
  const inner = `const before=document.cookie;document.cookie='corpus_p=nested; Secure; SameSite=None; Partitioned; Path=/';return {before,after:document.cookie,wire:(await(await fetch('/echo')).json()).headers.cookie||''}`;
  const middle = `const f=document.createElement('iframe');const a=new Promise(resolve=>{onmessage=e=>{if(e.source===f.contentWindow)resolve(e.data)}});f.src=${JSON.stringify(own+'/frame?id=inner&code='+encodeURIComponent(inner))};document.body.append(f);return await a`;
  const nested = await frame(other, 'middle', middle);
  const after = {document:document.cookie,wire:(await(await fetch('/echo')).json()).headers.cookie||''};
  return {first,nested,after};
})()
