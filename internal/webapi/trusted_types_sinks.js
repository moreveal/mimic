// Binding entry points check brands before string conversion and before any
// mutation. Internal parser/clone/node algorithms do not become extra sinks.
const trustedAttributeValue=(element,name,value,ns='',prefix="Failed to execute 'setAttribute' on 'Element': ")=>{
  const slot=elementSlot(element);if(!slot||slot.type!=='element')throw new TypeError('Illegal invocation');
  const info=trustedAttributeInfo(slot.tagName,name,slot.namespaceURI,ns);
  return info?trustedConvert(value,info[0],info[1],prefix,element):bindingString(value);
};
const trustedRequireElement=(element,iface)=>{
  const slot=elementSlot(element),tag={HTMLScriptElement:'SCRIPT',HTMLIFrameElement:'IFRAME',HTMLObjectElement:'OBJECT',HTMLEmbedElement:'EMBED'}[iface];
  if(!slot||slot.type!=='element'||slot.namespaceURI!=='http://www.w3.org/1999/xhtml'||slot.tagName!==tag)throw new TypeError('Illegal invocation');
  return slot;
};
const trustedSetAttribute=Element.prototype.setAttribute;
const trustedTextContentSetter=Object.getOwnPropertyDescriptor(Node.prototype,'textContent').set;
const trustedSetScriptSource=(element,value,property)=>{
  const slot=trustedRequireElement(element,'HTMLScriptElement');
  if(property!=='text'&&value==null)value='';
  const text=trustedConvert(value,'TrustedScript','HTMLScriptElement '+property,"Failed to set the '"+property+"' property on 'HTMLScriptElement': ",element);
  host.markTrustedScriptText(slot.nodeId,text);
  trustedApply(trustedTextContentSetter,element,[text]);
};
const trustedSetAttributeProperty=(element,name,value,kind,iface)=>{
  trustedRequireElement(element,iface);
  const text=trustedConvert(value,kind,iface+' '+name,"Failed to set the '"+name+"' property on '"+iface+"': ",element);
  trustedApply(trustedSetAttribute,element,[name,trustedValue(trustedConstructors[kind],text)]);
};
const trustedDocumentWrite=(receiver,args,name)=>{
  const allTrusted=args.length>0&&args.every(value=>trustedTypeOf(value)==='TrustedHTML');
  const text=args.map(value=>trustedTypeOf(value)==='TrustedHTML'?trustedSource(value,'TrustedHTML'):bindingString(value)).join('');
  return allTrusted?text:trustedConvert(text,'TrustedHTML','Document '+name,"Failed to execute '"+name+"' on 'Document': ",receiver);
};
{
  const method=(proto,name,fn)=>{Object.defineProperty(fn,'name',{value:name,configurable:true});markNative(fn,name);Object.defineProperty(proto,name,{value:fn,writable:true,enumerable:true,configurable:true})};
  const htmlSetter=Object.getOwnPropertyDescriptor(Element.prototype,'innerHTML').set;
  method(Element.prototype,'setHTMLUnsafe',function(value){
    if(!elementSlot(this))throw new TypeError('Illegal invocation');if(!arguments.length)throw new TypeError('Not enough arguments');
    const text=trustedConvert(value,'TrustedHTML','Element setHTMLUnsafe',"Failed to execute 'setHTMLUnsafe' on 'Element': ",this);
    htmlSetter.call(this,trustedValue(TrustedHTML,text));
  });
  const shadowSetter=Object.getOwnPropertyDescriptor(ShadowRoot.prototype,'innerHTML').set;
  method(ShadowRoot.prototype,'setHTMLUnsafe',function(value){
    if(!shadowSlots.has(this))throw new TypeError('Illegal invocation');if(!arguments.length)throw new TypeError('Not enough arguments');
    const text=trustedConvert(value,'TrustedHTML','ShadowRoot setHTMLUnsafe',"Failed to execute 'setHTMLUnsafe' on 'ShadowRoot': ",shadowSlots.get(this).host);
    shadowSetter.call(this,trustedValue(TrustedHTML,text));
  });
  method(Document,'parseHTMLUnsafe',function(value){
    if(!arguments.length)throw new TypeError('Not enough arguments');
    return wrap(host.parseInertDocument(trustedConvert(value,'TrustedHTML','Document parseHTMLUnsafe',"Failed to execute 'parseHTMLUnsafe' on 'Document': "),'text/html'));
  });
  for(const [iface,property] of [['HTMLObjectElement','data'],['HTMLObjectElement','codeBase'],['HTMLEmbedElement','src']]){
    const proto=globalThis[iface]?.prototype;if(!proto)continue;
    htmlElementInterfaces[iface==='HTMLObjectElement'?'OBJECT':'EMBED']=iface;
    Object.defineProperty(proto,property,{get(){const raw=this.getAttribute(property.toLowerCase());return raw===null?'':host.urlParts(raw).href},set(value){trustedSetAttributeProperty(this,property,value,'TrustedScriptURL',iface)},enumerable:true,configurable:true});
  }
  // Contextual fragment parsing uses the existing canonical fragment parser.
  // Range geometry and editing remain outside this binding's implementation.
  if(globalThis.Range){
    const rangeOwners=new WeakMap();
    method(Document.prototype,'createRange',function(){if(!(this instanceof Document))throw new TypeError('Illegal invocation');const range=Object.create(Range.prototype);rangeOwners.set(range,this);return range});
    method(Range.prototype,'createContextualFragment',function(value){
      const owner=rangeOwners.get(this);if(!owner)throw new TypeError('Illegal invocation');if(!arguments.length)throw new TypeError('Not enough arguments');
      const text=trustedConvert(value,'TrustedHTML','Range createContextualFragment',"Failed to execute 'createContextualFragment' on 'Range': ",owner);
      const parsed=owner.createElement('template');host.setInnerHTML(elementSlot(parsed).nodeId,text);return parsed.content;
    });
  }
  // SharedWorker execution remains explicitly unsupported. Its URL binding
  // still performs the required synchronous check before that boundary.
  if(globalThis.SharedWorker){
    const SharedWorker=class SharedWorker {constructor(value){if(!arguments.length)throw new TypeError('Not enough arguments');trustedConvert(value,'TrustedScriptURL','SharedWorker constructor',"Failed to construct 'SharedWorker': ");throw new DOMException('SharedWorker execution is unsupported.','NotSupportedError')}};
    markNative(SharedWorker,'SharedWorker');Object.defineProperty(globalThis,'SharedWorker',{value:SharedWorker,writable:true,configurable:true});
  }
}

const trustedCloneAttribute=(element,name,value,ns)=>{const info=trustedAttributeInfo(element.localName,ns?name.split(':').at(-1):name,element.namespaceURI,ns);return info?trustedValue(trustedConstructors[info[0]],value):value};
