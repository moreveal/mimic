(async () => {
 const shape=d=>d?{get:typeof d.get,set:typeof d.set,enumerable:d.enumerable,configurable:d.configurable}:null;
 const describe=(fn,...args)=>{try{return{value:fn(...args)}}catch(e){return{exception:e.name}}};
 const result={prototype:Object.fromEntries(['firstChild','location','textContent'].map(n=>[n,shape(Object.getOwnPropertyDescriptor(Document.prototype,n))]))};
 const inert=document.implementation.createHTMLDocument('inert');
 const activeDesc=Object.getOwnPropertyDescriptor(document,'location'),inertDesc=Object.getOwnPropertyDescriptor(inert,'location');
 result.locations={active:shape(activeDesc),inert:shape(inertDesc),sameGetter:!!inertDesc&&activeDesc.get===inertDesc.get};
 result.borrowed={inertLocation:describe(()=>activeDesc.get.call(inert)===null),invalidLocation:describe(()=>activeDesc.get.call({})),nodeText:describe(()=>Object.getOwnPropertyDescriptor(Node.prototype,'textContent').get.call(document)===null),inertNodeText:describe(()=>Object.getOwnPropertyDescriptor(Node.prototype,'textContent').get.call(inert)===null),nodeFirst:describe(()=>Object.getOwnPropertyDescriptor(Node.prototype,'firstChild').get.call(document)===document.firstChild)};
 result.setters={};
 if(activeDesc.set)for(const [name,receiver] of [['invalid',{}],['inert',inert],['active',document]]){
  const calls=[];const outcome=describe(()=>{activeDesc.set.call(receiver,{toString(){calls.push('convert');return '#document-descriptor-local'}});return true});result.setters[name]={outcome,calls};
 }
 const nodeText=Object.getOwnPropertyDescriptor(Node.prototype,'textContent'),nodeFirst=Object.getOwnPropertyDescriptor(Node.prototype,'firstChild').get;
 result.nodeConversions={};
 for(const [name,receiver] of [['invalid',{}],['inert',inert],['doctype',inert.doctype]]){
  const calls=[];result.nodeConversions[name]={outcome:describe(()=>{nodeText.set.call(receiver,{toString(){calls.push('convert');return 'ignored'}});return nodeText.get.call(receiver)}),calls};
  result.nodeConversions[name].symbol=describe(()=>{nodeText.set.call(receiver,Symbol());return true});
 }
 const first=document.firstChild;
 try{Object.defineProperty(document,'childNodes',{get(){throw new Error('public shadow')},configurable:true});result.firstChildPrivateState=nodeFirst.call(document)===first}finally{delete document.childNodes}
 const frame=document.createElement('iframe');frame.srcdoc='<title>child</title><body>child text';await new Promise(resolve=>{frame.onload=resolve;document.body.appendChild(frame)});
 result.borrowed.foreignLocation=describe(()=>activeDesc.get.call(frame.contentDocument)===frame.contentWindow.location);
 const child=frame.contentDocument;frame.remove();result.afterRemoval={borrowedLocationNull:activeDesc.get.call(child)===null,locationNull:child.location===null,defaultViewNull:child.defaultView===null};return result;
})()
