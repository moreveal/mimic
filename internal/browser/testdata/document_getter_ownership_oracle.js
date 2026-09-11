(async () => {
 const result={},frame=document.createElement('iframe');
 frame.srcdoc='<title>child-title</title><body><input id="child-input">';
 await new Promise(resolve=>{frame.onload=resolve;document.body.appendChild(frame)});
 const child=frame.contentDocument,owner=frame.contentWindow;
 const names=['title','URL','documentURI','cookie','activeElement','defaultView','body','head','documentElement','readyState','visibilityState','hidden','contentType','compatMode','domain','referrer','currentScript'];
 const getters=Object.fromEntries(names.map(name=>[name,Object.getOwnPropertyDescriptor(Document.prototype,name).get]));
 const compare=doc=>Object.fromEntries(names.map(name=>{try{return [name,getters[name].call(doc)===doc[name]]}catch(e){return [name,e.name]}}));
 result.active=compare(child);
 const inert=owner.document.implementation.createHTMLDocument('inert-title');
 result.inert=compare(inert);
 const original=Object.getOwnPropertyDescriptor(owner.Document.prototype,'URL');
 const expectedURL=child.URL;
 try {
  Object.defineProperty(child,'URL',{get(){throw new Error('own shadow')},configurable:true});
  Object.defineProperty(owner.Document.prototype,'URL',{get(){throw new Error('prototype shadow')},configurable:true});
  result.capturedGetter=getters.URL.call(child)===expectedURL;
 } finally {delete child.URL;Object.defineProperty(owner.Document.prototype,'URL',original)}
 result.localDetached=getters.defaultView.call(document.implementation.createHTMLDocument())===null;
 frame.remove();
 result.retained={title:getters.title.call(child)===child.title,body:getters.body.call(child)===child.body,defaultView:getters.defaultView.call(child)===child.defaultView};
 return result;
})()
