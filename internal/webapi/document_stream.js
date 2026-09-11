// Stream operations target the Document receiver, while the host call retains
// the calling realm for document.open's URL/origin checks.
{
  const frameOf=value=>{
    if(value===document)return {frame:host.selfFrameID(),realm:host.selfRealmID()};
    const reference=referenceGet(value);if(reference?.document)return reference;
    if(value instanceof Document)throw new DOMException('Document streaming requires an active HTML document.','InvalidStateError');
    throw new TypeError('Illegal invocation');
  };
  const invoke=(receiver,operation,source='')=>{
    const owner=frameOf(receiver);
    const result=host.documentStream(owner.frame,operation,source,owner.realm);
    if(result?.error)throw new DOMException(result.error,result.name||'InvalidStateError');
  };
  const methods={
    open(...args){if(args.length>=3)throw new DOMException('The window.open overload is unsupported.','NotSupportedError');invoke(this,'open');return this},
    write(...args){invoke(this,'write',args.map(String).join(''))},
    writeln(...args){invoke(this,'write',args.map(String).join('')+'\n')},
    close(){invoke(this,'close')}
  };
  for(const [name,value] of Object.entries(methods)){
    markNative(value,name);
    Object.defineProperty(Document.prototype,name,{value,writable:true,enumerable:true,configurable:true});
  }
  Object.defineProperty(Node.prototype,'baseURI',{get(){const owner=this instanceof Document?this:this.ownerDocument;return owner===document?host.documentBaseURI():owner?.URL||'about:blank'},enumerable:true,configurable:true});
  globalThis.__mimicDocumentStreamEvent=type=>dispatchTrusted(type==='load'?window:document,new Event(type,{bubbles:type==='DOMContentLoaded'}));
  const handlerNames=target=>{const names=[];for(const key in target)if(key.startsWith('on'))names.push(key);return names};
  const windowHandlers=handlerNames(window),documentHandlers=handlerNames(document);
  globalThis.__mimicResetDocumentStream=()=>{
    const clear=target=>{
      const listeners=eventListeners.get(target);
      if(listeners){for(const list of listeners.values())for(const record of list)if(record&&typeof record==='object')record.removed=true;listeners.clear()}
      const handlers=elementHandlers.get(target);
      if(handlers)for(const key of Object.keys(handlers))delete handlers[key];
      // Event-handler attributes on Window/Document are independent of the
      // addEventListener lists. Preserve arbitrary application properties.
      if(target===window||target===document)for(const key of target===window?windowHandlers:documentHandlers){
        // Do not invoke getters while resetting handlers: that would execute
        // application code or probe unsupported generated accessors.
        const descriptor=Object.getOwnPropertyDescriptor(target,key);
        if(descriptor&&'value' in descriptor&&typeof descriptor.value==='function'&&descriptor.writable)target[key]=null;
      }
    };
    const visit=node=>{clear(node);const shadow=elementShadows.get(node);if(shadow)visit(shadow);for(const child of node.childNodes||[])visit(child)};
    visit(document);clear(window);
  };
}
