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
  // Range geometry is intentionally coarse (Mimic has no inline fragment
  // renderer), but it must share the element layout model. Accessibility
  // clients use a Range around text nodes to determine their visibility.
  if(globalThis.Range){
    const rangeOwners=new WeakMap();
    const rangeState=receiver=>{const state=rangeOwners.get(receiver);if(!state)throw new TypeError('Illegal invocation');return state};
    const rangeAccessor=(proto,name,get)=>{if(!proto)return;Object.defineProperty(proto,name,{get,enumerable:true,configurable:true})};
    const boundaryLength=node=>node?.nodeType===3?(node.textContent||'').length:node?.childNodes?.length||0;
    const checkedBoundary=(state,node,offset)=>{
      if(!(node instanceof Node)||node.ownerDocument!==state.owner&&node!==state.owner)throw new DOMException('The node is not in this document.','WrongDocumentError');
      const value=Number(offset)>>>0;if(value>boundaryLength(node))throw new DOMException('The offset is larger than the node length.','IndexSizeError');return value;
    };
    const commonAncestor=(a,b)=>{const ancestors=[];for(let node=a;node;node=node.parentNode)ancestors.push(node);for(let node=b;node;node=node.parentNode)if(ancestors.includes(node))return node;return a};
    const rangeTarget=node=>node instanceof Element?node:node?.parentElement;
    const rangeRect=state=>{
      const targets=[];
      for(const node of state.nodes){const target=rangeTarget(node);if(target&&!targets.includes(target))targets.push(target)}
      if(!targets.length)return new DOMRect(0,0,0,0);
      let left=Infinity,top=Infinity,right=-Infinity,bottom=-Infinity;
      for(const target of targets){const rect=Element.prototype.getBoundingClientRect.call(target);left=Math.min(left,rect.left);top=Math.min(top,rect.top);right=Math.max(right,rect.right);bottom=Math.max(bottom,rect.bottom)}
      return Number.isFinite(left)?new DOMRect(left,top,Math.max(0,right-left),Math.max(0,bottom-top)):new DOMRect(0,0,0,0);
    };
    const rangeNodesBetween=(start,end)=>{
      if(start===end)return start?[start]:[];
      if(start?.parentNode&&start.parentNode===end?.parentNode){const children=Array.from(start.parentNode.childNodes),a=children.indexOf(start),b=children.indexOf(end);if(a>=0&&b>=0)return children.slice(Math.min(a,b),Math.max(a,b)+1)}
      return [start,end].filter(Boolean);
    };
    method(Document.prototype,'createRange',function(){
      if(!(this instanceof Document))throw new TypeError('Illegal invocation');
      const range=Object.create(Range.prototype);rangeOwners.set(range,{owner:this,nodes:[],startContainer:this,startOffset:0,endContainer:this,endOffset:0,collapsed:true});return range;
    });
    method(Range.prototype,'selectNode',function(node){
      const state=rangeState(this);if(!arguments.length)throw new TypeError('Not enough arguments');if(!(node instanceof Node)||node.ownerDocument!==state.owner)throw new DOMException('The node is not in this document.','WrongDocumentError');
      const parent=node.parentNode;if(!parent)throw new DOMException('The node has no parent.','InvalidNodeTypeError');const index=Array.from(parent.childNodes).indexOf(node);
      state.nodes=[node];state.startContainer=parent;state.startOffset=Math.max(0,index);state.endContainer=parent;state.endOffset=Math.max(0,index)+1;state.collapsed=false;
    });
    method(Range.prototype,'selectNodeContents',function(node){
      const state=rangeState(this);if(!arguments.length)throw new TypeError('Not enough arguments');if(!(node instanceof Node)||node.ownerDocument!==state.owner&&node!==state.owner)throw new DOMException('The node is not in this document.','WrongDocumentError');
      state.nodes=node instanceof Document?[node.documentElement].filter(Boolean):Array.from(node.childNodes);if(!state.nodes.length)state.nodes=[node];state.startContainer=node;state.startOffset=0;state.endContainer=node;state.endOffset=boundaryLength(node);state.collapsed=state.endOffset===0;
    });
    method(Range.prototype,'setStart',function(node,offset){
      const state=rangeState(this);if(arguments.length<2)throw new TypeError('Not enough arguments');state.startContainer=node;state.startOffset=checkedBoundary(state,node,offset);state.nodes=rangeNodesBetween(node,state.endContainer);state.collapsed=node===state.endContainer&&state.startOffset===state.endOffset;
    });
    method(Range.prototype,'setEnd',function(node,offset){
      const state=rangeState(this);if(arguments.length<2)throw new TypeError('Not enough arguments');state.endContainer=node;state.endOffset=checkedBoundary(state,node,offset);state.nodes=rangeNodesBetween(state.startContainer,node);state.collapsed=node===state.startContainer&&state.endOffset===state.startOffset;
    });
    const setAround=(receiver,node,start,after)=>{const state=rangeState(receiver);if(!node?.parentNode)throw new DOMException('The node has no parent.','InvalidNodeTypeError');const parent=node.parentNode,index=Array.from(parent.childNodes).indexOf(node)+(after?1:0);if(start){state.startContainer=parent;state.startOffset=index}else{state.endContainer=parent;state.endOffset=index}state.nodes=rangeNodesBetween(state.startContainer,state.endContainer);state.collapsed=state.startContainer===state.endContainer&&state.startOffset===state.endOffset};
    method(Range.prototype,'setStartBefore',function(node){if(!arguments.length)throw new TypeError('Not enough arguments');setAround(this,node,true,false)});
    method(Range.prototype,'setStartAfter',function(node){if(!arguments.length)throw new TypeError('Not enough arguments');setAround(this,node,true,true)});
    method(Range.prototype,'setEndBefore',function(node){if(!arguments.length)throw new TypeError('Not enough arguments');setAround(this,node,false,false)});
    method(Range.prototype,'setEndAfter',function(node){if(!arguments.length)throw new TypeError('Not enough arguments');setAround(this,node,false,true)});
    method(Range.prototype,'collapse',function(toStart=false){const state=rangeState(this);if(toStart){state.endContainer=state.startContainer;state.endOffset=state.startOffset}else{state.startContainer=state.endContainer;state.startOffset=state.endOffset}state.nodes=[];state.collapsed=true});
    method(Range.prototype,'getBoundingClientRect',function(){return rangeRect(rangeState(this))});
    method(Range.prototype,'getClientRects',function(){const rect=rangeRect(rangeState(this)),values=rect.width||rect.height?[rect]:[];return typeof makeDOMRectList==='function'?makeDOMRectList(values):values});
    method(Range.prototype,'cloneRange',function(){const state=rangeState(this),range=Object.create(Range.prototype);rangeOwners.set(range,{...state,nodes:[...state.nodes]});return range});
    method(Range.prototype,'toString',function(){return rangeState(this).nodes.map(node=>node.textContent||'').join('')});
    rangeAccessor(globalThis.AbstractRange?.prototype,'startOffset',function(){return rangeState(this).startOffset});
    rangeAccessor(globalThis.AbstractRange?.prototype,'endOffset',function(){return rangeState(this).endOffset});
    rangeAccessor(globalThis.AbstractRange?.prototype,'collapsed',function(){return rangeState(this).collapsed});
    rangeAccessor(globalThis.NodeRange?.prototype,'startContainer',function(){return rangeState(this).startContainer});
    rangeAccessor(globalThis.NodeRange?.prototype,'endContainer',function(){return rangeState(this).endContainer});
    rangeAccessor(Range.prototype,'commonAncestorContainer',function(){const state=rangeState(this);return commonAncestor(state.startContainer,state.endContainer)});
    method(Range.prototype,'createContextualFragment',function(value){
      const owner=rangeState(this).owner;if(!arguments.length)throw new TypeError('Not enough arguments');
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
