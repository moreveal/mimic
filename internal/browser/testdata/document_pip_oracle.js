(async()=>{
 const d=documentPictureInPicture,events=[],r={};
 r.initial=[Object.prototype.toString.call(d),d instanceof EventTarget,d.window,d.onenter];
 r.invalid=[];for(const o of [{width:1},{height:1},{width:-1,height:1}]){try{await d.requestWindow(o);r.invalid.push('unexpected')}catch(e){r.invalid.push([e.name,e.message])}}
 d.onenter=e=>events.push([e.type,e.isTrusted,e.window===d.window,e instanceof DocumentPictureInPictureEvent]);
 const w=await d.requestWindow({width:420,height:260});
 r.open=[w===d.window,w===w.window,w.top===w,w.parent===w,w.opener===window,w.frameElement,w.location.href,w.document.URL,w.document.baseURI===document.baseURI,w.document.readyState,w.document.referrer===location.href,w.document.visibilityState,w.document.body.innerHTML,w.document.head.innerHTML,navigator.userActivation.isActive];
 w.document.body.innerHTML='<p>hello</p>';r.dom=[w.document.body.textContent,w.document.defaultView===w,w.Array!==Array,w.document instanceof w.Document];
 try{await w.documentPictureInPicture.requestWindow()}catch(e){r.nested=[e.name,e.message]}
 try{await d.requestWindow()}catch(e){r.second=[e.name,e.message]}
 w.onpagehide=e=>events.push(['pagehide',e.persisted,w.closed,d.window===w,w.document.visibilityState]);
 w.onunload=e=>events.push(['unload',w.closed,d.window===w]);
 w.close();r.close=[w.closed,d.window===w,w.document.visibilityState];
 await new Promise(resolve=>setTimeout(resolve,100));
 r.later=[w.closed,d.window===null,w.document.visibilityState,w.top,w.parent,w.innerWidth,w.innerHeight];r.events=events;
 return JSON.stringify(r);
})()