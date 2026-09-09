// Every Document has its own canonical host root. The wrapper map preserves
// identity when traversal reaches an inert document through one of its nodes.
const documentWrappers = new Map();
const documentImplementations = new WeakMap();
const fragmentOwnerDocuments = new WeakMap();
function wrapDocumentNode(data) {
  if(data.nodeId===host.documentRootID())return document;
  let value=documentWrappers.get(data.nodeId);
  if(!value){value=Object.create(Document.prototype);elementData.set(value,data);documentWrappers.set(data.nodeId,value)}
  return value;
}
{
  const member=(prototype,name,value)=>{Object.defineProperty(value,'name',{value:name,configurable:true});markNative(value,name);Object.defineProperty(prototype,name,{value,writable:true,enumerable:true,configurable:true})};
  const accessor=(prototype,name,get,set)=>{markNative(get,name,'get ');markNative(set,name,'set ');Object.defineProperty(prototype,name,{get,set,enumerable:true,configurable:true})};
  const docID=value=>value===document?host.documentRootID():elementSlot(value)?.nodeId;
  const validDocument=value=>{if(!(value instanceof Document))throw new TypeError('Illegal invocation');return docID(value)};
  const ownerOf=value=>{
    if(value instanceof Document)return null;
    const slot=elementSlot(value);
    if(slot){const id=host.nodeOwnerDocument(slot.nodeId);return id?wrap(id):null}
    return fragmentOwnerDocuments.get(value)||document;
  };
  accessor(Node.prototype,'ownerDocument',function(){return ownerOf(this)});
  const originalType=Object.getOwnPropertyDescriptor(Node.prototype,'nodeType').get;
  const originalName=Object.getOwnPropertyDescriptor(Node.prototype,'nodeName').get;
  accessor(Node.prototype,'nodeType',function(){const type=elementSlot(this)?.type;return this instanceof Document?9:type==='doctype'?10:originalType.call(this)});
  accessor(Node.prototype,'nodeName',function(){return this instanceof Document?'#document':elementSlot(this)?.type==='doctype'?elementSlot(this).tagName:originalName.call(this)});
  if(globalThis.DocumentType){
    accessor(globalThis.DocumentType.prototype,'name',function(){return elementSlot(this)?.tagName||''});
    for(const name of ['publicId','systemId'])accessor(globalThis.DocumentType.prototype,name,function(){return host.nodeData(elementSlot(this).nodeId).attributes?.[name==='publicId'?'public':'system']||''});
    accessor(globalThis.DocumentType.prototype,'textContent',function(){return null},function(){});
  }
  const connected=Object.getOwnPropertyDescriptor(Node.prototype,'isConnected').get;
  accessor(Node.prototype,'isConnected',function(){if(this instanceof Document||connected.call(this))return true;return this.getRootNode({composed:true}) instanceof Document});
  const adopt=(node,doc)=>{
    const id=validDocument(doc),slot=elementSlot(node);
    if(slot)host.adoptNodeDocument(slot.nodeId,id);
    else if(node instanceof DocumentFragment){fragmentOwnerDocuments.set(node,doc);for(const child of node.childNodes)adopt(child,doc)}
    return node;
  };
  for(const name of ['createElement','createElementNS','createTextNode','createComment','createDocumentFragment']){
    const original=Document.prototype[name];
    member(Document.prototype,name,function(...args){validDocument(this);return adopt(original.apply(this,args),this)});
  }
  for(const name of ['documentElement','head','body'])accessor(Document.prototype,name,function(){validDocument(this);return this.querySelector(name==='documentElement'?'html':name)});
  for(const prototype of [Document.prototype,Element.prototype])member(prototype,'getElementsByTagName',function(name){
    if(!(this instanceof Document)&&!(this instanceof Element))throw new TypeError('Illegal invocation');
    name=String(name);const lower=name.toLowerCase(),root=this;
    return htmlCollection(()=>Array.from(root.querySelectorAll('*')).filter(node=>name==='*'||node.localName===(node.namespaceURI==='http://www.w3.org/1999/xhtml'?lower:name)).map(node=>elementSlot(node).nodeId));
  });
  accessor(Document.prototype,'defaultView',function(){validDocument(this);return this===document?window:null});
  accessor(Document.prototype,'textContent',function(){validDocument(this);return null},function(){validDocument(this)});
  for(const name of ['URL','documentURI'])accessor(Document.prototype,name,function(){validDocument(this);return this===document?location.href:'about:blank'});
  accessor(Document.prototype,'location',function(){validDocument(this);return this===document?location:null});
  const ready=Object.getOwnPropertyDescriptor(Document.prototype,'readyState').get;
  accessor(Document.prototype,'readyState',function(){validDocument(this);return this===document?ready.call(this):'complete'});
  accessor(Document.prototype,'compatMode',function(){validDocument(this);return this===document&&!Array.from(this.childNodes).some(node=>node.nodeType===10)?'BackCompat':'CSS1Compat'});
  accessor(Document.prototype,'doctype',function(){validDocument(this);return Array.from(this.childNodes).find(node=>node.nodeType===10)||null});
  accessor(Document.prototype,'contentType',function(){validDocument(this);return 'text/html'});
  for(const name of ['characterSet','charset','inputEncoding'])accessor(Document.prototype,name,function(){validDocument(this);return 'UTF-8'});
  const current=Object.getOwnPropertyDescriptor(Document.prototype,'currentScript').get;
  accessor(Document.prototype,'currentScript',function(){validDocument(this);return this===document?current.call(this):null});
  const title=Object.getOwnPropertyDescriptor(Document.prototype,'title');
  accessor(Document.prototype,'title',function(){validDocument(this);if(this===document)return title.get.call(this);return (this.querySelector('title')?.textContent||'').replace(/[\t\n\f\r ]+/g,' ').trim()},function(value){validDocument(this);if(this===document)return title.set.call(this,value);let node=this.querySelector('title');if(!node&&this.head){node=this.createElement('title');this.head.appendChild(node)}if(node)node.textContent=String(value)});
  const implSlots=new WeakSet();
  class DOMImplementation {
    constructor(token){if(token!==hostToken)throw new TypeError('Illegal constructor');implSlots.add(this)}
    createHTMLDocument(title){if(!implSlots.has(this))throw new TypeError('Illegal invocation');return wrap(host.createHTMLDocument(...(title!==undefined?[String(title)]:[])))}
    hasFeature(){if(!implSlots.has(this))throw new TypeError('Illegal invocation');return true}
  }
  Object.defineProperty(DOMImplementation.prototype,Symbol.toStringTag,{value:'DOMImplementation',configurable:true});
  markNative(DOMImplementation,'DOMImplementation');
  for(const name of ['createHTMLDocument','hasFeature'])markNative(DOMImplementation.prototype[name],name);
  Object.defineProperty(globalThis,'DOMImplementation',{value:DOMImplementation,writable:true,configurable:true});
  accessor(Document.prototype,'implementation',function(){validDocument(this);let value=documentImplementations.get(this);if(!value){value=new DOMImplementation(hostToken);documentImplementations.set(this,value)}return value});
  member(Document.prototype,'adoptNode',function(node){validDocument(this);if(!(node instanceof Node))throw new TypeError('Expected a Node');if(node instanceof Document||node instanceof ShadowRoot)throw new DOMException('Node cannot be adopted','NotSupportedError');if(node.parentNode)node.parentNode.removeChild(node);return adopt(node,this)});
  const clone=Node.prototype.cloneNode;
  member(Node.prototype,'cloneNode',function(deep=false){const result=clone.call(this,deep);return adopt(result,this.ownerDocument||document)});
  member(Document.prototype,'importNode',function(node,deep=false){validDocument(this);if(!(node instanceof Node))throw new TypeError('Expected a Node');return adopt(node.cloneNode(deep),this)});
  for(const name of ['appendChild','insertBefore']){
    const original=Node.prototype[name];
    member(Node.prototype,name,function(node,before=null){
      if(this instanceof Document){
        if(!(node instanceof Node))throw new TypeError('Expected a Node');
        const added=node instanceof DocumentFragment?Array.from(node.childNodes):[node];
        const children=Array.from(this.childNodes).filter(child=>!added.includes(child));
        const index=before==null?children.length:children.indexOf(before);
        if(before!=null&&before!==node&&index<0)throw new DOMException('Reference node is not a child','NotFoundError');
        children.splice(Math.max(0,index),0,...added);
        if(children.some(child=>![1,7,8,10].includes(child.nodeType))||children.filter(child=>child.nodeType===1).length>1||children.filter(child=>child.nodeType===10).length>1||children.findIndex(child=>child.nodeType===10)>children.findIndex(child=>child.nodeType===1)&&children.some(child=>child.nodeType===1))throw new DOMException('Invalid document children','HierarchyRequestError');
      }
      return name==='appendChild'?original.call(this,node):original.call(this,node,before);
    });
  }
}
