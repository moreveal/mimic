(async () => {
  const p=trustedTypes.createPolicy('allowed',{createScriptURL:s=>s});
  const source=`const base=${JSON.stringify(location.origin)},out={},calls=[];
  const a=(name,fn)=>{try{const v=fn();out[name]=v===undefined?'undefined':v}catch(e){out[name]=e.name}};
  a('plainImport',()=>importScripts(base+'/trusted-import.js'));
  a('plainTimer',()=>clearTimeout(setTimeout('42',10000)));
  const p=trustedTypes.createPolicy('allowed',{createScriptURL:s=>s,createScript:s=>s});
  a('trustedImport',()=>importScripts(p.createScriptURL(base+'/trusted-import.js')));
  trustedTypes.createPolicy('default',{createScriptURL:(...x)=>{calls.push(x.slice(1));return x[0]},createScript:(...x)=>{calls.push(x.slice(1));return x[0]}});
  a('defaultImport',()=>importScripts(base+'/trusted-import.js'));
  a('defaultTimer',()=>clearTimeout(setTimeout('42',10000)));
  a('defaultInterval',()=>clearInterval(setInterval('42',10000)));
  out.imported=globalThis.imported||0;out.calls=calls;postMessage(out);`;
  const blob=URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
  const result=await new Promise(resolve=>{const w=new Worker(p.createScriptURL(blob));w.onmessage=e=>{w.terminate();resolve(e.data)};w.onerror=e=>{w.terminate();resolve({error:e.message})}});
  URL.revokeObjectURL(blob);return result;
})()
