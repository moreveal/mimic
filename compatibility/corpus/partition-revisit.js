(async () => {
  const target='http://127.0.0.1:'+location.port;
  const f=document.createElement('iframe');
  const p=new Promise(resolve=>{const cb=e=>{if(e.source===f.contentWindow&&e.data.id==='revisit'){removeEventListener('message',cb);resolve(e.data)}};addEventListener('message',cb)});
  f.src=target+'/frame?id=revisit&code='+encodeURIComponent("return {document:document.cookie,wire:(await(await fetch('/echo')).json()).headers.cookie||''}");
  document.body.append(f);try{return {top:location.hostname,child:await p}}finally{f.remove()}
})()
