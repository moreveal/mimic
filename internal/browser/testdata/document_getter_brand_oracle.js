(() => {
 const result={},receivers={object:{},null:null,undefined:undefined,primitive:1,documentPrototype:Object.create(Document.prototype),htmlDocumentPrototype:Object.create(HTMLDocument.prototype),element:document.body,window,proxy:new Proxy(document,{}),derived:Object.create(document)};
 const names=['activeElement','cookie','domain','referrer','lastModified','visibilityState','fullscreen','webkitIsFullScreen','pointerLockElement','onabort','onreadystatechange','onmouseenter','onmouseleave','title','body','URL','defaultView','fonts','styleSheets','adoptedStyleSheets'];
 for(const name of names){
  const getter=Object.getOwnPropertyDescriptor(Document.prototype,name).get,row={};
  for(const [kind,receiver] of Object.entries(receivers)){
   try{const value=getter.call(receiver);row[kind]={type:typeof value}}
   catch(error){row[kind]={exception:error.name}}
  }
  result[name]=row;
 }
 const contentType=Object.getOwnPropertyDescriptor(Document.prototype,'contentType').get;
 const inert=document.implementation.createHTMLDocument();Object.setPrototypeOf(inert,null);
 result.privateBrandSurvivesPrototypeMutation=contentType.call(inert)==='text/html';
 return result;
})()
