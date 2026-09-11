(async()=>{
const out={},n=navigation,logs=[];const original=n.currentEntry;
n.addEventListener('navigate',e=>{logs.push(['navigate',e.navigationType,e.destination.sameDocument,e.hashChange]);if(e.info==='cancel')e.preventDefault();if(e.info==='intercept'){e.intercept({handler:async()=>{logs.push(['handler',location.pathname,!!n.transition]);await Promise.resolve();logs.push(['handler-end'])}})}});
for(const type of ['currententrychange','navigatesuccess','navigateerror'])n.addEventListener(type,e=>logs.push([type,e.navigationType===undefined?null:e.navigationType]));
let r=n.navigate('#one',{state:{a:1}});await r.finished;const one=n.currentEntry;one.addEventListener('dispose',()=>logs.push(['dispose']));
r=n.navigate('/intercept',{info:'intercept',state:{b:2}});await r.finished;out.intercept={path:location.pathname,state:n.currentEntry.getState(),transition:n.transition};
r=n.back();out.back=[(await r.committed)===one,(await r.finished)===one,location.hash,n.currentEntry.getState()];
r=n.forward();await r.finished;out.forward=location.pathname;
r=n.navigate('#cancel',{info:'cancel'});out.cancel=[];for(const p of [r.committed,r.finished])try{await p}catch(e){out.cancel.push(e.name)}
try{n.updateCurrentEntry({state:undefined})}catch(e){out.undefinedState=e.name}
out.logs=logs;return out
})()

