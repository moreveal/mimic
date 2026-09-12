(async () => {
  const out={},calls=[],p=trustedTypes.createPolicy('allowed',{createHTML:s=>s,createScript:s=>s,createScriptURL:s=>s});
  let defaults=false;
  globalThis.ttExecution=0;
  const attempt=(name,fn)=>{calls.length=0;globalThis.ttExecution=0;try{fn();out[name]={executed:globalThis.ttExecution,calls:calls.slice()}}catch(e){out[name]={error:e.name,executed:globalThis.ttExecution,calls:calls.slice()}}};
  function run(prefix){
    const source='globalThis.ttExecution++';
    attempt(prefix+'text',()=>{const s=document.createElement('script');s.text=p.createScript(source);document.body.appendChild(s)});
    attempt(prefix+'textNode',()=>{const s=document.createElement('script');s.appendChild(document.createTextNode(source));document.body.appendChild(s)});
    attempt(prefix+'nodeSetter',()=>{const s=document.createElement('script');Object.getOwnPropertyDescriptor(Node.prototype,'textContent').set.call(s,source);document.body.appendChild(s)});
    attempt(prefix+'scriptInnerHTML',()=>{const s=document.createElement('script');s.innerHTML=p.createHTML(source);document.body.appendChild(s)});
    attempt(prefix+'sameTextMutation',()=>{const s=document.createElement('script');s.text=p.createScript(source);s.firstChild.data=source;document.body.appendChild(s)});
    attempt(prefix+'changedTextMutation',()=>{const s=document.createElement('script');s.text=p.createScript('globalThis.ttExecution+=2');s.firstChild.data=source;document.body.appendChild(s)});
    attempt(prefix+'clonedText',()=>{const s=document.createElement('script');s.text=p.createScript(source);document.body.appendChild(s.cloneNode(true))});
    attempt(prefix+'connectedText',()=>{const s=document.createElement('script');document.body.appendChild(s);s.text=p.createScript(source)});
    attempt(prefix+'connectedTextNode',()=>{const s=document.createElement('script');document.body.appendChild(s);s.appendChild(document.createTextNode(source))});
    attempt(prefix+'connectedNodeSetter',()=>{const s=document.createElement('script');document.body.appendChild(s);Object.getOwnPropertyDescriptor(Node.prototype,'textContent').set.call(s,source)});
    attempt(prefix+'connectedInnerHTML',()=>{const s=document.createElement('script');document.body.appendChild(s);s.innerHTML=p.createHTML(source)});
    attempt(prefix+'nestedScript',()=>{const d=document.createElement('div'),s=document.createElement('script');s.text=p.createScript(source);d.appendChild(s);document.body.appendChild(d)});
    attempt(prefix+'eventHandler',()=>{const b=document.createElement('button');b.setAttribute('onclick',p.createScript(source));b.click()});
    attempt(prefix+'parserHTML',()=>{const d=document.createElement('div');d.innerHTML=p.createHTML('<script>'+source+'</script>');document.body.appendChild(d)});
  }
  run('noDefault:');
  trustedTypes.createPolicy('default',{createScript:(...args)=>{calls.push(args);return args[0]}});
  run('default:');
  const workerSource="const out={},a=(n,f)=>{try{out[n]=f()}catch(e){out[n]=e.name}};a('blocked',()=>eval('42'));const c=[];trustedTypes.createPolicy('default',{createScript:(...x)=>{c.push(x);return x[0]}});a('default',()=>eval('42'));a('function',()=>Function('return 42')());out.calls=c;postMessage(out)";
  const blob=URL.createObjectURL(new Blob([workerSource],{type:'text/javascript'}));
  out.worker=await new Promise(resolve=>{const w=new Worker(p.createScriptURL(blob));w.onmessage=e=>{w.terminate();resolve(e.data)};w.onerror=e=>{w.terminate();resolve({error:e.message})}});URL.revokeObjectURL(blob);
  return out;
})()
