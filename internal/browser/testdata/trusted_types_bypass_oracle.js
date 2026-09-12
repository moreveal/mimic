(async()=>{
  const out={},calls=[],a=(name,fn)=>{calls.length=0;try{const v=fn();out[name]={value:v===undefined?'undefined':v,calls:calls.slice()}}catch(e){out[name]={error:e.name,message:e.message,calls:calls.slice()}}};
  a('plainEval',()=>eval('42'));
  a('plainHTML',()=>{document.body.innerHTML='blocked'});
  a('plainTimer',()=>{clearTimeout(setTimeout('42',10000))});
  const meta=document.createElement('meta');meta.httpEquiv='Content-Security-Policy';meta.content='trusted-types default allowed';document.head.appendChild(meta);
  a('disallowedPolicy',()=>trustedTypes.createPolicy('other',{}));
  trustedTypes.createPolicy('default',{createHTML:(...x)=>{calls.push(x);return x[0]},createScript:(...x)=>{calls.push(x);return x[0]}});
  a('defaultEval',()=>eval('42'));
  a('defaultHTML',()=>{document.body.innerHTML='allowed';return document.body.innerHTML});
  a('defaultTimer',()=>{setTimeout('globalThis.ttBypassTimer=42',0)});
  await new Promise(r=>setTimeout(r,10));
  a('timerExecuted',()=>globalThis.ttBypassTimer);
  return out;
})()
