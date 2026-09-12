// Attached Attr values are projections of the canonical host attribute map.
// Only a removed Attr owns a detached value; wrappers and NamedNodeMap objects
// retain identity without copying the element's mutable attribute state.
const attributeCompatibility=(()=>{
  const slots=new WeakMap(),maps=new WeakMap(),attributes=new WeakMap();
  const member=(prototype,name,value)=>{Object.defineProperty(value,'name',{value:name,configurable:true});markNative(value,name);Object.defineProperty(prototype,name,{value,writable:true,enumerable:true,configurable:true})};
  const accessor=(prototype,name,get,set)=>{if(get)Object.defineProperty(get,'name',{value:'get '+name,configurable:true});if(set)Object.defineProperty(set,'name',{value:'set '+name,configurable:true});markNative(get,name,'get ');markNative(set,name,'set ');Object.defineProperty(prototype,name,{get,set,enumerable:true,configurable:true})};
  const htmlElement=value=>{if(!(value instanceof HTMLElement)||!elementSlot(value))throw new TypeError('Illegal invocation');return value};
  for(const C of [globalThis.HTMLInputElement,globalThis.HTMLTextAreaElement]){
    if(typeof C!=='function')continue;
    const control=value=>{if(!(value instanceof C)||!elementSlot(value))throw new TypeError('Illegal invocation');return value};
    accessor(C.prototype,'dirName',function(){return control(this).getAttribute('dirname')||''},function(value){control(this);if(typeof value==='symbol')throw new TypeError('Cannot convert a Symbol value to a string');this.setAttribute('dirname',String(value))});
    accessor(C.prototype,'maxLength',function(){
      const raw=control(this).getAttribute('maxlength')||'',match=/^[\t\n\f\r ]*([+-]?\d+)/.exec(raw),value=match?Number(match[1]):-1;
      return value>=0&&value<=2147483647?value:-1;
    },function(value){control(this);value=(+value)|0;if(value<0)throw new DOMException('The value provided is negative.','IndexSizeError');this.setAttribute('maxlength',String(value))});
  }
  for(const name of ['translate','spellcheck','draggable'])accessor(HTMLElement.prototype,name,function(){
    let node=htmlElement(this);
    while(node){
      const value=node.getAttribute(name)?.toLowerCase();
      if(name==='draggable')return value==='true'?true:value==='false'?false:node.localName==='img'||node.localName==='a'&&node.hasAttribute('href');
      if(value===(name==='translate'?'no':'false'))return false;
      if(value===''||value===(name==='translate'?'yes':'true'))return true;
      const parent=node.parentNode;
      // Chrome spellcheck crosses a shadow host; translate uses DOM ancestry.
      node=parent instanceof HTMLElement?parent:name==='spellcheck'&&parent instanceof ShadowRoot?parent.host:null;
    }
    return !(name==='spellcheck'&&this.localName==='input'&&this.type==='password');
  },function(value){htmlElement(this).setAttribute(name,name==='translate'?(value?'yes':'no'):(value?'true':'false'))});
  const cache=element=>{let value=attributes.get(element);if(!value){value=new Map();attributes.set(element,value)}return value};
  const state=attr=>{const value=slots.get(attr);if(!value)throw new TypeError('Illegal invocation');return value};
  const normalize=(element,name)=>element.namespaceURI==='http://www.w3.org/1999/xhtml'?String(name).toLowerCase():String(name);
  const detach=attr=>{const value=state(attr);if(value.owner){value.value=value.owner.getAttribute(value.name)??value.value;value.document=value.owner.ownerDocument;value.owner=null}return attr};
  const create=(name,value,document,owner=null,namespace=null)=>{const attr=Object.create(globalThis.Attr.prototype);slots.set(attr,{name,value,document,owner,namespace});nonHostNodeBrands.add(attr);return attr};
  const get=(element,name)=>{
    name=normalize(element,name);const value=element.getAttribute(name),values=cache(element),previous=values.get(name);
    if(value===null){if(previous){detach(previous);values.delete(name)}return null}
    if(previous){state(previous).value=value;return previous}
    const attr=create(name,value,element.ownerDocument,element,host.nodeData(elementSlot(element).nodeId).attributeNamespaces?.[name]||null);values.set(name,attr);return attr;
  };
  const names=element=>element.getAttributeNames();
  const nsName=(element,namespace,local)=>{if(!(element instanceof Element))throw new TypeError('Illegal invocation');return host.attributeNameNS(elementSlot(element).nodeId,namespace==null?'':String(namespace),String(local))};
  member(Element.prototype,'getAttributeNS',function(namespace,local){const name=nsName(this,namespace,local);return name?this.getAttribute(name):null});
  member(Element.prototype,'hasAttributeNS',function(namespace,local){return !!nsName(this,namespace,local)});
  member(Element.prototype,'removeAttributeNS',function(namespace,local){const name=nsName(this,namespace,local);if(name)this.removeAttribute(name)});
  member(Element.prototype,'setAttributeNS',function(namespace,name,value){
    if(!(this instanceof Element))throw new TypeError('Illegal invocation');if(arguments.length<3)throw new TypeError('Not enough arguments');
    namespace=namespace==null?'':String(namespace);name=String(name);value=trustedAttributeValue(this,namespace?name.split(':').at(-1):name,value,namespace,"Failed to execute 'setAttributeNS' on 'Element': ");
    if(!/^[A-Za-z_:][A-Za-z0-9_.:\-]*$/.test(name)&&!/^[\p{L}_:][\p{L}\p{N}_.:\-\u00b7\p{M}]*$/u.test(name))throw new DOMException('Invalid XML name','InvalidCharacterError');
    const prefix=name.includes(':')?name.split(':')[0]:null;
    if(prefix&&!namespace||prefix==='xml'&&namespace!=='http://www.w3.org/XML/1998/namespace'||(name==='xmlns'||prefix==='xmlns')&&namespace!=='http://www.w3.org/2000/xmlns/'||namespace==='http://www.w3.org/2000/xmlns/'&&name!=='xmlns'&&prefix!=='xmlns')throw new DOMException('Invalid namespace','NamespaceError');
    host.setAttributeNS(elementSlot(this).nodeId,namespace,name,value);
  });
  const indexed=key=>typeof key==='string'&&/^(0|[1-9][0-9]*)$/.test(key)&&Number(key)<4294967295;
  const mapOwners=new WeakMap();
  function namedMap(element){
    let map=maps.get(element);if(map)return map;
    map=new Proxy(Object.create(globalThis.NamedNodeMap.prototype),{
      get(target,key,receiver){if(indexed(key)){const name=names(element)[Number(key)];return name===undefined?undefined:get(element,name)}if(Reflect.has(target,key))return Reflect.get(target,key,receiver);return typeof key==='string'?get(element,key)||undefined:undefined},
      has(target,key){return indexed(key)?Number(key)<names(element).length:Reflect.has(target,key)||typeof key==='string'&&get(element,key)!==null},
      ownKeys(){const list=names(element);return [...list.map((_,i)=>String(i)),...list.filter(name=>!indexed(name))]},
      getOwnPropertyDescriptor(target,key){if(indexed(key)&&Number(key)<names(element).length)return {value:get(element,names(element)[Number(key)]),writable:false,enumerable:true,configurable:true};if(typeof key==='string'&&get(element,key))return {value:get(element,key),writable:false,enumerable:false,configurable:true};return undefined}
    });
    maps.set(element,map);mapOwners.set(map,element);return map;
  }
  if(!globalThis.Attr||!globalThis.NamedNodeMap)return {namedMap};
  const owner=map=>{const element=mapOwners.get(map);if(!element)throw new TypeError('Illegal invocation');return element};
  accessor(globalThis.NamedNodeMap.prototype,'length',function(){return names(owner(this)).length});
  member(globalThis.NamedNodeMap.prototype,'item',function(index){const element=owner(this),name=names(element)[Number(index)>>>0];return name===undefined?null:get(element,name)});
  member(globalThis.NamedNodeMap.prototype,'getNamedItem',function(name){return get(owner(this),name)});
  member(globalThis.NamedNodeMap.prototype,'removeNamedItem',function(name){const element=owner(this),attr=get(element,name);if(!attr)throw new DOMException('Attribute not found','NotFoundError');return element.removeAttributeNode(attr)});
  member(globalThis.NamedNodeMap.prototype,'setNamedItem',function(attr){return owner(this).setAttributeNode(attr)});
  Object.defineProperty(globalThis.NamedNodeMap.prototype,Symbol.iterator,{value:function*(){const element=owner(this);for(let index=0;index<names(element).length;index++)yield get(element,names(element)[index])},writable:true,configurable:true});
  for(const name of ['name','nodeName'])accessor(globalThis.Attr.prototype,name,function(){return state(this).name});
  accessor(globalThis.Attr.prototype,'namespaceURI',function(){return state(this).namespace});
  accessor(globalThis.Attr.prototype,'localName',function(){const s=state(this);return s.namespace?s.name.split(':').at(-1):s.name});
  accessor(globalThis.Attr.prototype,'prefix',function(){const s=state(this);return s.namespace&&s.name.includes(':')?s.name.split(':')[0]:null});
  for(const name of ['parentNode','parentElement'])accessor(globalThis.Attr.prototype,name,function(){state(this);return null});
  accessor(globalThis.Attr.prototype,'nodeType',function(){state(this);return 2});
  accessor(globalThis.Attr.prototype,'specified',function(){state(this);return true});
  accessor(globalThis.Attr.prototype,'ownerElement',function(){const value=state(this);if(value.owner&&!value.owner.hasAttribute(value.name)){cache(value.owner).delete(value.name);detach(this)}return value.owner});
  accessor(globalThis.Attr.prototype,'ownerDocument',function(){const value=state(this);return value.owner?value.owner.ownerDocument:value.document});
  const read=function(){const value=state(this);return value.owner?value.owner.getAttribute(value.name)??value.value:value.value};
  const write=function(input){const value=state(this),text=bindingString(input);if(value.owner){const approved=trustedAttributeValue(value.owner,value.name,text,value.namespace,"Failed to set the 'value' property on 'Attr': "),kind=trustedAttributeInfo(value.owner.localName,value.name,value.owner.namespaceURI,value.namespace)?.[0];value.owner.setAttribute(value.name,kind?trustedValue(trustedConstructors[kind],approved):approved)}else value.value=text};
  accessor(globalThis.Attr.prototype,'value',read,write);
  for(const name of ['nodeValue','textContent'])accessor(globalThis.Attr.prototype,name,read,function(value){write.call(this,value==null?'':value)});
  member(globalThis.Attr.prototype,'cloneNode',function(){const value=state(this);return create(value.name,this.value,this.ownerDocument,null,value.namespace)});
  member(Document.prototype,'createAttribute',function(name){if(!(this instanceof Document))throw new TypeError('Illegal invocation');name=String(name);if(!name||/[\s<>\/=]/.test(name))throw new DOMException('Invalid attribute name','InvalidCharacterError');return create(name.toLowerCase(),'',this)});
  member(Element.prototype,'getAttributeNodeNS',function(namespace,local){const name=nsName(this,namespace,local);return name?get(this,name):null});
  member(Element.prototype,'getAttributeNode',function(name){if(!(this instanceof Element))throw new TypeError('Illegal invocation');return get(this,name)});
  const remove=Element.prototype.removeAttribute;
  member(Element.prototype,'removeAttribute',function(name){name=normalize(this,name);const attr=cache(this).get(name);if(attr){detach(attr);cache(this).delete(name)}return remove.call(this,name)});
  member(Element.prototype,'removeAttributeNode',function(attr){
    if(!(this instanceof Element))throw new TypeError('Illegal invocation');
    if(!slots.has(attr))throw new TypeError('Parameter is not an Attr');
    if(attr.ownerElement!==this||get(this,attr.name)!==attr)throw new DOMException('Attribute is not owned by this element','NotFoundError');
    this.removeAttribute(attr.name);return attr;
  });
  member(Element.prototype,'setAttributeNode',function(attr){
    if(!(this instanceof Element))throw new TypeError('Illegal invocation');if(!slots.has(attr))throw new TypeError('Parameter is not an Attr');
    const value=state(attr);if(value.owner&&value.owner!==this)throw new DOMException('Attribute is already in use','InUseAttributeError');
    const previous=get(this,value.name);if(previous===attr)return attr;
    const approved=trustedAttributeValue(this,value.namespace?value.name.split(':').at(-1):value.name,value.value,value.namespace,"Failed to execute 'setAttributeNode' on 'Element': "),kind=trustedAttributeInfo(this.localName,value.namespace?value.name.split(':').at(-1):value.name,this.namespaceURI,value.namespace)?.[0];
    value.value=approved;
    if(previous)detach(previous);
    value.document=this.ownerDocument;value.owner=this;cache(this).set(value.name,attr);if(value.namespace)this.setAttributeNS(value.namespace,value.name,kind?trustedValue(trustedConstructors[kind],approved):approved);else this.setAttribute(value.name,kind?trustedValue(trustedConstructors[kind],approved):approved);return previous;
  });
  member(Element.prototype,'setAttributeNodeNS',function(attr){return this.setAttributeNode(attr)});
  // Attr inherits these bindings from Node. Select its private state here so
  // borrowing Node accessors and changing public prototypes preserve semantics.
  for(const name of ['nodeName','nodeType','nodeValue','ownerDocument','parentNode','parentElement','textContent']){
    const attr=Object.getOwnPropertyDescriptor(globalThis.Attr.prototype,name);
    const node=Object.getOwnPropertyDescriptor(Node.prototype,name);
    accessor(Node.prototype,name,function(){
      if(!isDOMNode(this))throw new TypeError('Illegal invocation');
      return functionSourceApply(slots.has(this)?attr.get:node.get,this,[]);
    },node.set?function(value){
      if(!isDOMNode(this))throw new TypeError('Illegal invocation');
      return functionSourceApply(slots.has(this)?attr.set:node.set,this,[value]);
    }:undefined);
    delete globalThis.Attr.prototype[name];
  }
  const clone=Node.prototype.cloneNode;
  member(Node.prototype,'cloneNode',function(deep=false){
    if(!isDOMNode(this))throw new TypeError('Illegal invocation');
    if(!slots.has(this))return functionSourceApply(clone,this,[deep]);
    const value=state(this);
    return create(value.name,functionSourceApply(read,this,[]),value.owner?value.owner.ownerDocument:value.document,null,value.namespace);
  });
  delete globalThis.Attr.prototype.cloneNode;
  return {namedMap};
})();
