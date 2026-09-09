// Attached Attr values are projections of the canonical host attribute map.
// Only a removed Attr owns a detached value; wrappers and NamedNodeMap objects
// retain identity without copying the element's mutable attribute state.
const attributeCompatibility=(()=>{
  const slots=new WeakMap(),maps=new WeakMap(),attributes=new WeakMap();
  const member=(prototype,name,value)=>{Object.defineProperty(value,'name',{value:name,configurable:true});markNative(value,name);Object.defineProperty(prototype,name,{value,writable:true,enumerable:true,configurable:true})};
  const accessor=(prototype,name,get,set)=>{markNative(get,name,'get ');markNative(set,name,'set ');Object.defineProperty(prototype,name,{get,set,enumerable:true,configurable:true})};
  const cache=element=>{let value=attributes.get(element);if(!value){value=new Map();attributes.set(element,value)}return value};
  const state=attr=>{const value=slots.get(attr);if(!value)throw new TypeError('Illegal invocation');return value};
  const normalize=(element,name)=>element.namespaceURI==='http://www.w3.org/1999/xhtml'?String(name).toLowerCase():String(name);
  const detach=attr=>{const value=state(attr);if(value.owner){value.value=value.owner.getAttribute(value.name)??value.value;value.document=value.owner.ownerDocument;value.owner=null}return attr};
  const create=(name,value,document,owner=null)=>{const attr=Object.create(globalThis.Attr.prototype);slots.set(attr,{name,value,document,owner});return attr};
  const get=(element,name)=>{
    name=normalize(element,name);const value=element.getAttribute(name),values=cache(element),previous=values.get(name);
    if(value===null){if(previous){detach(previous);values.delete(name)}return null}
    if(previous){state(previous).value=value;return previous}
    const attr=create(name,value,element.ownerDocument,element);values.set(name,attr);return attr;
  };
  const names=element=>element.getAttributeNames();
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
  for(const name of ['name','nodeName','localName'])accessor(globalThis.Attr.prototype,name,function(){return state(this).name});
  for(const name of ['namespaceURI','prefix','parentNode','parentElement'])accessor(globalThis.Attr.prototype,name,function(){state(this);return null});
  accessor(globalThis.Attr.prototype,'nodeType',function(){state(this);return 2});
  accessor(globalThis.Attr.prototype,'specified',function(){state(this);return true});
  accessor(globalThis.Attr.prototype,'ownerElement',function(){const value=state(this);if(value.owner&&!value.owner.hasAttribute(value.name)){cache(value.owner).delete(value.name);detach(this)}return value.owner});
  accessor(globalThis.Attr.prototype,'ownerDocument',function(){const value=state(this);return value.owner?value.owner.ownerDocument:value.document});
  const read=function(){const value=state(this);return value.owner?value.owner.getAttribute(value.name)??value.value:value.value};
  const write=function(input){const value=state(this),text=String(input);if(value.owner)value.owner.setAttribute(value.name,text);else value.value=text};
  accessor(globalThis.Attr.prototype,'value',read,write);
  for(const name of ['nodeValue','textContent'])accessor(globalThis.Attr.prototype,name,read,function(value){write.call(this,value==null?'':value)});
  member(globalThis.Attr.prototype,'cloneNode',function(){const value=state(this);return create(value.name,this.value,this.ownerDocument)});
  member(Document.prototype,'createAttribute',function(name){if(!(this instanceof Document))throw new TypeError('Illegal invocation');name=String(name);if(!name||/[\s<>\/=]/.test(name))throw new DOMException('Invalid attribute name','InvalidCharacterError');return create(name.toLowerCase(),'',this)});
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
    if(previous)detach(previous);
    value.document=this.ownerDocument;value.owner=this;cache(this).set(value.name,attr);this.setAttribute(value.name,value.value);return previous;
  });
  return {namedMap};
})();
