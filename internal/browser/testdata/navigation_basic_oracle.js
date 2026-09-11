(async()=>{
const n=navigation, out={tag:Object.prototype.toString.call(n),initial:{entry:!!n.currentEntry,entries:n.entries().length,back:n.canGoBack,forward:n.canGoForward,transition:n.transition,activation:!!n.activation}}, logs=[];
const entry=e=>({url:new URL(e.url).pathname+new URL(e.url).hash,index:e.index,sameDocument:e.sameDocument,state:e.getState(),key:!!e.key,id:!!e.id});
for(const type of ['navigate','currententrychange','navigatesuccess','navigateerror'])n.addEventListener(type,e=>logs.push({type,...(type==='navigate'?{navigationType:e.navigationType,canIntercept:e.canIntercept,cancelable:e.cancelable,hashChange:e.hashChange,userInitiated:e.userInitiated,downloadRequest:e.downloadRequest,formData:e.formData,info:e.info,state:e.destination.getState(),index:e.destination.index,key:!!e.destination.key,id:!!e.destination.id,sameDocument:e.destination.sameDocument}:type==='currententrychange'?{navigationType:e.navigationType,from:entry(e.from)}:{})}));
const original=n.currentEntry;
history.pushState({h:1},'', '#a');out.push={same:n.currentEntry===original,keySame:n.currentEntry.key===original.key,entry:entry(n.currentEntry),old:entry(original),history:history.state};
const pushed=n.currentEntry;history.replaceState({h:2},'','#b');out.replace={same:n.currentEntry===pushed,keySame:n.currentEntry.key===pushed.key,idSame:n.currentEntry.id===pushed.id,old:entry(pushed),entry:entry(n.currentEntry)};
n.updateCurrentEntry({state:{n:3}});const state=n.currentEntry.getState();state.n=9;out.update={state:n.currentEntry.getState(),history:history.state};
const result=n.navigate('#c',{state:{n:4},info:'test'});out.result={keys:Object.keys(result),committed:(await result.committed)===n.currentEntry,finished:(await result.finished)===n.currentEntry};
out.after=entry(n.currentEntry);out.logs=logs;return out
})()
