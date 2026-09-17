(async()=>{
  const frame=document.createElement('iframe');document.body.appendChild(frame);
  const inspect=frame.contentWindow.eval(`(()=>{const h=history;return ()=>{const result={};const probes={state:()=>h.state,length:()=>h.length,scrollGet:()=>h.scrollRestoration,scrollSet:()=>{h.scrollRestoration='manual'},scrollInvalid:()=>{h.scrollRestoration='invalid'},push:()=>h.pushState({x:1},''),replace:()=>h.replaceState({x:1},''),go:()=>h.go(1),back:()=>h.back(),forward:()=>h.forward()};for(const [key,read] of Object.entries(probes)){try{const value=read();result[key]={ok:true,value:value===undefined?'undefined':value}}catch(e){result[key]={ok:false,name:e.name}}}return JSON.stringify(result)}})()`);
  await new Promise(resolve=>{frame.onload=resolve;frame.src='/history-next'});
  return JSON.parse(inspect());
})()
