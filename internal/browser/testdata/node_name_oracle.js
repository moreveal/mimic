(() => {
 const out={}, read=fn=>{try{const v=fn();return v===document?'document':v===null?null:typeof v==='object'?Object.prototype.toString.call(v):v}catch(e){return {exception:e.name}}};
 const host=document.createElement('div');document.body.appendChild(host);
 const nodes={text:document.createTextNode('abc'),comment:document.createComment('abc'),fragment:document.createDocumentFragment(),element:document.createElement('div'),doctype:document.doctype,document,shadow:host.attachShadow({mode:'open'}),svg:document.createElementNS('http://www.w3.org/2000/svg','linearGradient')};
 const getter=Object.getOwnPropertyDescriptor(Node.prototype,'nodeName').get;
 for(const [kind,node] of Object.entries(nodes)){
  out[kind]={direct:read(()=>node.nodeName),borrowed:read(()=>getter.call(node))};
  Object.defineProperty(node,'tagName',{value:'shadowed',configurable:true});out[kind].shadowed=read(()=>getter.call(node));delete node.tagName;
  const prototype=Object.getPrototypeOf(node);Object.setPrototypeOf(node,null);
  out[kind].privateName=read(()=>getter.call(node));Object.setPrototypeOf(node,prototype);
 }
 out.placement={};for(const name of ['Element','CharacterData','DocumentFragment','DocumentType','Document','Attr'])out.placement[name]=Object.hasOwn(globalThis[name].prototype,'nodeName');
 out.invalid=read(()=>getter.call({}));out.attr=read(()=>getter.call(document.createAttribute('data-test')));
 host.remove();return out;
})()
