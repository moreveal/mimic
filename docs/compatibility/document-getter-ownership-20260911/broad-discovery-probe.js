(async () => {
 const result={invalid:{},borrowed:{}},receivers={object:{},null:null,undefined:undefined,primitive:1,documentPrototype:Object.create(Document.prototype),htmlDocumentPrototype:Object.create(HTMLDocument.prototype),element:document.body,window};
 for(const name of Object.getOwnPropertyNames(Document.prototype)) {
  const get=Object.getOwnPropertyDescriptor(Document.prototype,name).get;if(!get)continue;
  const row={};for(const [kind,receiver] of Object.entries(receivers)){try{const value=get.call(receiver);row[kind]={type:typeof value}}catch(error){row[kind]={exception:error.name}}}
  result.invalid[name]=row;
 }
 const frame=document.createElement('iframe');frame.srcdoc='<title>child-title</title><body><input id="child-input">';await new Promise(resolve=>{frame.onload=resolve;document.body.appendChild(frame)});
 const child=frame.contentDocument;
 for(const name of ['title','URL','documentURI','cookie','activeElement','defaultView','body','head','documentElement','readyState','visibilityState','hidden','contentType','compatMode','domain','referrer','currentScript']) {
  const get=Object.getOwnPropertyDescriptor(Document.prototype,name)?.get;if(!get){result.borrowed[name]={missing:true};continue}
  try {const value=get.call(child);result.borrowed[name]={equal:value===child[name],type:typeof value}}catch(error){result.borrowed[name]={exception:error.name}}
 }
 frame.remove();return result;
})()
