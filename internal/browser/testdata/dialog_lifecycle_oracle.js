(async()=>{
 const out=[],wait=()=>new Promise(r=>setTimeout(r,30));
 document.body.innerHTML='<button id="a">open</button><dialog id="d"><input id="i" autofocus></dialog>';
 const d=document.getElementById('d'),a=document.getElementById('a');a.focus();
 for(const type of ['beforetoggle','toggle','close','cancel'])d.addEventListener(type,e=>out.push([type,e.constructor.name,e.oldState??null,e.newState??null,e.cancelable,e.isTrusted,d.open,d.matches(':modal'),document.activeElement.id,e.source?.id??null]));
 d.showModal();out.push(['after-show',d.open,d.matches(':modal'),document.activeElement.id]);await wait();
 d.close('ok');out.push(['after-close',d.open,d.matches(':modal'),document.activeElement.id,d.returnValue]);await wait();
 d.showModal();d.close();d.showModal();await wait();
 d.open=false;out.push(['remove-open',d.open,d.matches(':modal'),document.activeElement.id]);await wait();
 d.addEventListener('beforetoggle',e=>{if(e.newState==='open')e.preventDefault()},{once:true});d.showModal();out.push(['canceled',d.open,d.matches(':modal')]);await wait();
 return out;
})()
