(() => {
 const result={},receivers={object:{},null:null,undefined:undefined,primitive:1,documentPrototype:Object.create(Document.prototype),htmlDocumentPrototype:Object.create(HTMLDocument.prototype),element:document.body,window};
 for(const name of ['adoptedStyleSheets','cookie','domain','title','designMode','onabort','onreadystatechange','onmouseenter','onmouseleave','fullscreen','fullscreenElement','fullscreenEnabled']){
  const set=Object.getOwnPropertyDescriptor(Document.prototype,name).set;if(!set)continue;
  const row={};for(const [kind,receiver] of Object.entries(receivers)){
   const calls=[],value={toString(){calls.push('string');return 'local-fixture-value'},get [Symbol.iterator](){calls.push('iterator');return function*(){}}};
   try{set.call(receiver,value);row[kind]={status:'returned',calls}}
   catch(error){row[kind]={exception:error.name,calls}}
  }result[name]=row;
 }result.validReadonly={};
 for(const name of ['fullscreen','fullscreenElement','fullscreenEnabled']){
  const descriptor=Object.getOwnPropertyDescriptor(Document.prototype,name),calls=[];
  try{const before=descriptor.get.call(document);descriptor.set.call(document,{toString(){calls.push('convert');return 'yes'}});descriptor.set.call(document,Symbol());result.validReadonly[name]={same:descriptor.get.call(document)===before,calls}}
  catch(error){result.validReadonly[name]={exception:error.name,calls}}
 }
 result.symbolConversion={};
 for(const name of ['xmlVersion','domain','cookie','title','dir','designMode','fgColor','linkColor','vlinkColor','alinkColor','bgColor']){
  const setter=Object.getOwnPropertyDescriptor(Document.prototype,name).set,doc=document.implementation.createHTMLDocument();
  try{setter.call(doc,Symbol('input'));result.symbolConversion[name]='returned'}catch(error){result.symbolConversion[name]=error.name}
 }
 result.validHandlers={};
 for(const name of ['onabort','onreadystatechange','onmouseenter','onmouseleave']){
  const d=Object.getOwnPropertyDescriptor(Document.prototype,name),fn=()=>{};
  d.set.call(document,fn);result.validHandlers[name]=d.get.call(document)===fn;d.set.call(document,null);
 }
 const inert=document.implementation.createHTMLDocument(),design=Object.getOwnPropertyDescriptor(Document.prototype,'designMode');Object.setPrototypeOf(inert,null);design.set.call(inert,'on');result.privateBrand=design.get.call(inert)==='on';
 return result;
})()
