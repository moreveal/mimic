(() => {
 const out={},names=['nodeName','nodeType','nodeValue','ownerDocument','parentNode','parentElement','textContent'];
 const a=document.createAttribute('data-test');a.value='initial';
 const read=fn=>{try {const v=fn();return v===document?'document':v===null?null:typeof v==='object'?Object.prototype.toString.call(v):v} catch(e){return {exception:e.name}}};
 out.placement={};out.borrowed={};out.invalid={};
 for(const n of names){out.placement[n]=Object.hasOwn(Attr.prototype,n);const d=Object.getOwnPropertyDescriptor(Node.prototype,n);out.borrowed[n]=read(()=>d.get.call(a));out.invalid[n]=read(()=>d.get.call({}));}
 out.clonePlacement=Object.hasOwn(Attr.prototype,'cloneNode');out.clone=read(()=>{const c=Node.prototype.cloneNode.call(a);return c.name+':'+c.value+':'+(c.ownerDocument===document)});
 out.writes={};for(const name of ['nodeValue','textContent']){const set=Object.getOwnPropertyDescriptor(Node.prototype,name).set;const x=document.createAttribute('x');out.writes[name]=read(()=>{set.call(x,'changed');return x.value});out.writes[name+'Symbol']=read(()=>set.call(x,Symbol()));}
 const e=document.createElement('div');e.setAttributeNode(a);Object.defineProperty(a,'value',{value:'shadow',configurable:true});out.attached=read(()=>Object.getOwnPropertyDescriptor(Node.prototype,'textContent').get.call(a));delete a.value;
 Object.setPrototypeOf(a,null);out.privateBrand=read(()=>Object.getOwnPropertyDescriptor(Node.prototype,'nodeName').get.call(a));
 return out;
})()