const compatibilityElementState={};
  // Realm-local semantic state. Canonical nodes remain owned by the DOM host.
  {
    const expose=(name,value)=>Object.defineProperty(globalThis,name,{value,writable:true,configurable:true});
    const definitions=new Map(),constructors=new Map(),upgraded=new WeakMap(),construction=[];
    const waiting=new Map();let defining=false;
    const report=error=>console.error(error&&error.stack||String(error));
    let reactionDepth=0;const reactions=[],elementReactions=new WeakMap();
    // An invocation owns its element queue. A nested CEReactions operation
    // drains the affected element's reactions, never its pending siblings.
    const flushReactions=()=>{if(reactionDepth)return;const pending=reactions.splice(0);for(const node of pending){const queue=elementReactions.get(node);while(queue?.length){const [fn,args]=queue.shift();try{fn.apply(node,args)}catch(error){report(error)}}}};
    const reaction=(node,name,args=[])=>{const fn=upgraded.get(node)?.callbacks[name];if(typeof fn==='function'){let queue=elementReactions.get(node);if(!queue)elementReactions.set(node,queue=[]);queue.push([fn,args]);reactions.push(node);flushReactions()}};
    const validName=name=>/^[a-z][.0-9_a-z\-]*-[.0-9_a-z\-]*$/.test(name)&&!['annotation-xml','color-profile','font-face','font-face-src','font-face-uri','font-face-format','font-face-name','missing-glyph'].includes(name);
    const rawCreate=Document.prototype.createElement;
    constructCustomElement=ctor=>{
      const definition=constructors.get(ctor);
      if(ctor===HTMLElement||!definition)throw new TypeError('Illegal constructor');
      const frame=construction[construction.length-1];
      if(frame&&frame.definition===definition){
        if(frame.used)throw new TypeError('Custom element constructed twice');
        frame.used=true;return frame.node;
      }
      const node=rawCreate.call(document,definition.name);
      Object.setPrototypeOf(node,ctor.prototype);upgraded.set(node,definition);
      return node;
    };
    const upgrade=node=>{
      if(!(node instanceof Element)||upgraded.has(node))return;
      const definition=definitions.get(node.localName);if(!definition)return;
      upgraded.set(node,null);Object.setPrototypeOf(node,definition.prototype);
      const frame={node,definition,used:false},attributes=node.getAttributeNames().filter(name=>definition.attributes.includes(name)).map(name=>[name,null,node.getAttribute(name),null]),connected=node.isConnected;construction.push(frame);reactionDepth++;
      try{
        if(Reflect.construct(definition.ctor,[])!==node)throw new TypeError('Custom element returned a different object');
        upgraded.set(node,definition);
        for(const args of attributes)reaction(node,'attributeChangedCallback',args);
        if(connected)reaction(node,'connectedCallback');
      }catch(error){report(error)}finally{construction.pop();reactionDepth--;flushReactions()}
    };
    const walk=(node,callback)=>{
      callback(node);
      const shadow=elementShadows.get(node);if(shadow)walk(shadow,callback);
      for(const child of Array.from(node.childNodes||[]))walk(child,callback);
    };
    class CustomElementRegistry {
      constructor(){if(arguments[0]!==hostToken)throw new TypeError('Illegal constructor')}
      define(name,ctor,options={}){
        if(arguments.length<2)throw new TypeError('Two arguments required');
        name=String(name);
        if(typeof ctor!=='function')throw new TypeError('Invalid constructor');
        try{Reflect.construct(new Proxy(ctor,{construct(){return {}}}),[])}catch(error){throw new TypeError('Invalid constructor')}
        if(!validName(name))throw new DOMException('Invalid custom element name','SyntaxError');
        if(definitions.has(name)||constructors.has(ctor))throw new DOMException('Already defined','NotSupportedError');
        if(options.extends)throw new DOMException('Customized built-in elements are not supported','NotSupportedError');
        if(defining)throw new DOMException('Definition is running','NotSupportedError');
        let definition;defining=true;
        const sequence=value=>{if(value===undefined)return[];if(value==null||typeof value[Symbol.iterator]!=='function')throw new TypeError('Expected iterable');return Array.from(value,String)};
        try{
          const prototype=ctor.prototype;if(prototype===null||typeof prototype!=='object'&&typeof prototype!=='function')throw new TypeError('Invalid prototype');
          const callbacks={};const readCallback=key=>{const value=prototype[key];if(value!==undefined&&typeof value!=='function')throw new TypeError('Callback is not callable');callbacks[key]=value};
          for(const key of ['connectedCallback','disconnectedCallback','connectedMoveCallback','adoptedCallback','attributeChangedCallback'])readCallback(key);
          const attributes=callbacks.attributeChangedCallback?sequence(ctor.observedAttributes):[];
          const disabledFeatures=sequence(ctor.disabledFeatures),formAssociated=!!ctor.formAssociated;
          if(formAssociated)for(const key of ['formAssociatedCallback','formResetCallback','formDisabledCallback','formStateRestoreCallback','toolFillCallback'])readCallback(key);
          definition={name,ctor,prototype,callbacks,attributes,disabledFeatures,formAssociated};
        }finally{defining=false}
        definitions.set(name,definition);constructors.set(ctor,definition);
        walk(document,node=>{if(node instanceof Element&&node.localName===name)upgrade(node)});
        if(waiting.has(name)){waiting.get(name).resolve(ctor);waiting.delete(name)}
      }
      get(name){return definitions.get(String(name))?.ctor}
      getName(ctor){if(typeof ctor!=='function')throw new TypeError('Expected a constructor');return constructors.get(ctor)?.name||null}
      whenDefined(name){name=String(name);if(!validName(name))return Promise.reject(new DOMException('Invalid custom element name','SyntaxError'));if(definitions.has(name))return Promise.resolve(definitions.get(name).ctor);if(!waiting.has(name)){let resolve;const promise=new Promise(r=>resolve=r);waiting.set(name,{promise,resolve})}return waiting.get(name).promise}
      upgrade(root){walk(root,upgrade)}
    }
    expose('CustomElementRegistry',CustomElementRegistry);expose('customElements',new CustomElementRegistry(hostToken));
    Object.defineProperty(Document.prototype,'createElement',{value:function(name,options){const node=rawCreate.call(this,name,options);if(definitions.size&&!customElementCloneInert)upgrade(node);return node},writable:true,configurable:true,enumerable:true});

    const observers=new Set(),observerSlots=new WeakMap(),pendingObservers=new Set();let deliveryQueued=false,mutationDepth=0,observerSequence=0;
    const scheduleDelivery=()=>{if(!deliveryQueued){deliveryQueued=true;queueMicrotask(deliver)}};
    const deliver=()=>{
      deliveryQueued=false;const active=Array.from(pendingObservers).sort((a,b)=>observerSlots.get(a).sequence-observerSlots.get(b).sequence);pendingObservers.clear();
      for(const observer of active){const s=observerSlots.get(observer),records=s.records.splice(0);s.transients=[];
        if(records.length)try{s.callback.call(observer,records,observer)}catch(error){report(error)}
      }
    };
    const recordSlots=new WeakMap();
    class MutationRecord {constructor(){throw new TypeError('Illegal constructor')}}
    for(const name of ['type','target','addedNodes','removedNodes','previousSibling','nextSibling','attributeName','attributeNamespace','oldValue'])Object.defineProperty(MutationRecord.prototype,name,{get(){const data=recordSlots.get(this);if(!data)throw new TypeError('Illegal invocation');return data[name]},enumerable:true,configurable:true});
    Object.defineProperty(MutationRecord.prototype,Symbol.toStringTag,{value:'MutationRecord',configurable:true});
    expose('MutationRecord',MutationRecord);
    const observerState=observer=>{const state=observerSlots.get(observer);if(!state)throw new TypeError('Illegal invocation');return state};
    const observedRoots=s=>[...s.targets,...s.transients];
    const retainRemoved=node=>{
      for(const observer of observers){const s=observerSlots.get(observer),entries=[];
        for(const [root,options] of observedRoots(s))if(options.subtree&&root!==node&&root.contains(node))entries.push([node,options]);
        if(entries.length){s.transients.push(...entries);pendingObservers.add(observer);scheduleDelivery()}
      }
    };
    const queueRecord=(type,target,details={})=>{
      if(!observers.size||mutationDepth)return;
      for(const observer of observers){
        const s=observerSlots.get(observer);let match=false,old=false;
        for(const [root,options] of observedRoots(s)){
          if(root!==target&&(!options.subtree||!root.contains(target)))continue;
          if(!options[type])continue;
          if(type==='attributes'&&options.attributeFilter&&(details.attributeNamespace!=null||!options.attributeFilter.includes(details.attributeName)))continue;
          match=true;old=old||!!options[type==='attributes'?'attributeOldValue':'characterDataOldValue'];
        }
        if(!match)continue;
        const record=Object.create(MutationRecord.prototype);
        const fields={type,target,addedNodes:[],removedNodes:[],previousSibling:null,nextSibling:null,attributeName:null,attributeNamespace:null,oldValue:null,...details};
        fields.addedNodes=nodeList(fields.addedNodes.map(n=>elementSlot(n)));fields.removedNodes=nodeList(fields.removedNodes.map(n=>elementSlot(n)));
        if(!old)fields.oldValue=null;recordSlots.set(record,fields);s.records.push(record);pendingObservers.add(observer);scheduleDelivery();
      }
    };
    class MutationObserver {
      constructor(callback){if(typeof callback!=='function')throw new TypeError('Callback must be callable');observerSlots.set(this,{callback,sequence:observerSequence++,records:[],targets:new Map(),transients:[]})}
      observe(target,options){
        const s=observerState(this);if(!(target instanceof Node))throw new TypeError('Target must be a Node');
        const init=Object(options);options={attributeFilter:init.attributeFilter,attributeOldValue:init.attributeOldValue,attributes:init.attributes,characterData:init.characterData,characterDataOldValue:init.characterDataOldValue,childList:init.childList,subtree:init.subtree};
        if(options.attributes===undefined&&(options.attributeOldValue!==undefined||options.attributeFilter!==undefined))options.attributes=true;
        if(options.characterData===undefined&&options.characterDataOldValue!==undefined)options.characterData=true;
        if(!options.childList&&!options.attributes&&!options.characterData||!options.attributes&&(options.attributeOldValue||options.attributeFilter)||!options.characterData&&options.characterDataOldValue)throw new TypeError('Invalid observer options');
        if(options.attributeFilter!==undefined){if(options.attributeFilter==null||typeof options.attributeFilter[Symbol.iterator]!=='function')throw new TypeError('attributeFilter must be iterable');options.attributeFilter=Array.from(options.attributeFilter,String)}
        if(s.targets.has(target))s.transients=s.transients.filter(([,registered])=>registered!==s.targets.get(target));
        s.targets.set(target,options);observers.add(this);
      }
      disconnect(){const s=observerState(this);s.targets.clear();s.records.length=0;s.transients=[];observers.delete(this);pendingObservers.delete(this)}
      takeRecords(){return observerState(this).records.splice(0)}
    }
    Object.defineProperty(MutationObserver.prototype,Symbol.toStringTag,{value:'MutationObserver',configurable:true});
    expose('MutationObserver',MutationObserver);
    // CharacterData and traversal share the canonical host node; cached wrapper
    // records only describe identity, and must never serve mutable text.
    const member=(proto,name,value)=>Object.defineProperty(proto,name,{value,writable:true,enumerable:true,configurable:true});
    const accessor=(proto,name,get,set)=>Object.defineProperty(proto,name,{get,set,enumerable:true,configurable:true});
    const exceptionCodes={IndexSizeError:1,HierarchyRequestError:3,WrongDocumentError:4,InvalidCharacterError:5,NoModificationAllowedError:7,NotFoundError:8,NotSupportedError:9,InUseAttributeError:10,InvalidStateError:11,SyntaxError:12,InvalidModificationError:13,NamespaceError:14,InvalidAccessError:15,TypeMismatchError:17,SecurityError:18,NetworkError:19,AbortError:20,URLMismatchError:21,QuotaExceededError:22,TimeoutError:23,InvalidNodeTypeError:24,DataCloneError:25};
    accessor(DOMException.prototype,'code',function(){return exceptionCodes[this.name]||0});
    for(const [name,value] of Object.entries({ELEMENT_NODE:1,ATTRIBUTE_NODE:2,TEXT_NODE:3,CDATA_SECTION_NODE:4,ENTITY_REFERENCE_NODE:5,ENTITY_NODE:6,PROCESSING_INSTRUCTION_NODE:7,COMMENT_NODE:8,DOCUMENT_NODE:9,DOCUMENT_TYPE_NODE:10,DOCUMENT_FRAGMENT_NODE:11,NOTATION_NODE:12,DOCUMENT_POSITION_DISCONNECTED:1,DOCUMENT_POSITION_PRECEDING:2,DOCUMENT_POSITION_FOLLOWING:4,DOCUMENT_POSITION_CONTAINS:8,DOCUMENT_POSITION_CONTAINED_BY:16,DOCUMENT_POSITION_IMPLEMENTATION_SPECIFIC:32}))for(const target of [Node,Node.prototype])Object.defineProperty(target,name,{value,enumerable:true});
    const documentRootID=host.documentRootID(),childLists=new WeakMap();
    const childCount=node=>{if(node===document)return host.nodeChildCount(documentRootID);const slot=elementSlot(node);if(slot)return host.nodeChildCount(slot.nodeId);return fragmentSlots.get(node)?.children.length||0};
    const childAt=(node,index)=>{if(node===document)return wrap(host.nodeChildAt(documentRootID,index));const slot=elementSlot(node);if(slot)return wrap(host.nodeChildAt(slot.nodeId,index));return fragmentSlots.get(node)?.children[index]||null};
    accessor(Node.prototype,'childNodes',function(){let list=childLists.get(this);if(!list){const node=this;list=new Proxy(Object.create(NodeList.prototype),{get(target,key){if(key==='length')return childCount(node);if(key==='item')return index=>childAt(node,Number(index)>>>0);if(key==='forEach')return(callback,thisArg)=>{const length=childCount(node);for(let i=0;i<length;i++){const child=childAt(node,i);if(child)callback.call(thisArg,child,i,list)}};if(key===Symbol.iterator||key==='values')return function*(){for(let i=0;i<childCount(node);i++)yield childAt(node,i)};if(key==='keys')return function*(){for(let i=0;i<childCount(node);i++)yield i};if(key==='entries')return function*(){for(let i=0;i<childCount(node);i++)yield [i,childAt(node,i)]};if(typeof key==='string'&&/^\d+$/.test(key))return childAt(node,Number(key))||undefined;return Reflect.get(target,key)},has(target,key){return typeof key==='string'&&/^(0|[1-9][0-9]*)$/.test(key)&&Number(key)<childCount(node)||Reflect.has(target,key)},ownKeys(target){return [...Array.from({length:childCount(node)},(_,i)=>String(i)),...Reflect.ownKeys(target)]},getOwnPropertyDescriptor(target,key){if(typeof key==='string'&&/^(0|[1-9][0-9]*)$/.test(key)&&Number(key)<childCount(node))return {value:childAt(node,Number(key)),writable:false,enumerable:true,configurable:true};return Reflect.getOwnPropertyDescriptor(target,key)}});childLists.set(this,list)}return list});
    accessor(Document.prototype,'firstChild',function(){return wrap(host.firstChild(documentRootID))});
    const characterText=node=>JSON.parse(host.textContentJSON(elementSlot(node).nodeId));
    const setCharacterText=(node,value)=>{
      const old=observers.size?characterText(node):null;
      value=String(value);host.setCharacterDataJSON(elementSlot(node).nodeId,JSON.stringify(value).replace(/[\ud800-\udfff]/g,unit=>'\\u'+unit.charCodeAt(0).toString(16).padStart(4,'0')));
      queueRecord('characterData',node,{oldValue:old});
    };
    accessor(CharacterData.prototype,'data',function(){return characterText(this)},function(value){setCharacterText(this,value===null?'':value)});
    accessor(CharacterData.prototype,'textContent',function(){return characterText(this)},function(value){setCharacterText(this,value==null?'':value)});
    accessor(CharacterData.prototype,'length',function(){return this.data.length});
    const unsigned=value=>Number(value)>>>0;
    member(CharacterData.prototype,'substringData',function(offset,count){if(arguments.length<2)throw new TypeError('Two arguments required');const text=this.data;offset=unsigned(offset);count=unsigned(count);if(offset>text.length)throw new DOMException('Offset exceeds data length','IndexSizeError');return text.slice(offset,offset+count)});
    member(CharacterData.prototype,'replaceData',function(offset,count,data){if(arguments.length<3)throw new TypeError('Three arguments required');const text=this.data;offset=unsigned(offset);count=unsigned(count);data=String(data);if(offset>text.length)throw new DOMException('Offset exceeds data length','IndexSizeError');setCharacterText(this,text.slice(0,offset)+data+text.slice(offset+count))});
    member(CharacterData.prototype,'appendData',function(data){if(!arguments.length)throw new TypeError('Argument required');this.replaceData(this.length,0,String(data))});
    member(CharacterData.prototype,'insertData',function(offset,data){if(arguments.length<2)throw new TypeError('Two arguments required');this.replaceData(offset,0,String(data))});
    member(CharacterData.prototype,'deleteData',function(offset,count){if(arguments.length<2)throw new TypeError('Two arguments required');this.replaceData(offset,count,'')});
    const sibling=(node,offset)=>{const parent=syntheticParents.get(node);if(parent){const children=fragmentSlots.get(parent).children;return children[children.indexOf(node)+offset]||null}const slot=elementSlot(node);return slot?wrap(host.sibling(slot.nodeId,offset)):null};
    for(const proto of [Node.prototype,Element.prototype]){
      accessor(proto,'nextSibling',function(){return sibling(this,1)});
      accessor(proto,'previousSibling',function(){return sibling(this,-1)});
    }
    accessor(Node.prototype,'lastChild',function(){const nodes=this.childNodes;return nodes[nodes.length-1]||null});
    for(const proto of [Element.prototype,CharacterData.prototype])for(const [name,offset] of [['nextElementSibling',1],['previousElementSibling',-1]])accessor(proto,name,function(){let node=this;do{node=sibling(node,offset)}while(node&&node.nodeType!==1);return node});
    accessor(Element.prototype,'lastElementChild',function(){const nodes=this.children;return nodes[nodes.length-1]||null});
    member(Text.prototype,'splitText',function(offset){if(!arguments.length)throw new TypeError('Argument required');offset=unsigned(offset);const text=this.data;if(offset>text.length)throw new DOMException('Offset exceeds data length','IndexSizeError');const node=document.createTextNode(text.slice(offset)),parent=this.parentNode;if(parent)parent.insertBefore(node,this.nextSibling);this.replaceData(offset,text.length-offset,'');return node});
    accessor(Text.prototype,'wholeText',function(){let node=this;while(node.previousSibling?.nodeType===3)node=node.previousSibling;let text='';for(;node?.nodeType===3;node=node.nextSibling)text+=node.data;return text});
    member(Node.prototype,'normalize',function(){for(let node=this.firstChild;node;){if(node.nodeType===3){if(node.length===0){const next=node.nextSibling;this.removeChild(node);node=next;continue}while(node.nextSibling?.nodeType===3){const next=node.nextSibling;node.appendData(next.data);this.removeChild(next)}}else node.normalize();node=node.nextSibling}});
    member(Node.prototype,'replaceChild',function(node,child){if(!(node instanceof Node)||!(child instanceof Node))throw new TypeError('Expected Nodes');if(child.parentNode!==this)throw new DOMException('Not a child','NotFoundError');if(node===child)return child;this.insertBefore(node,child);this.removeChild(child);return child});
    const removeChildBase=Node.prototype.removeChild;
    member(Node.prototype,'removeChild',function(node){if(!(node instanceof Node))throw new TypeError('Expected a Node');if(node.parentNode!==this)throw new DOMException('Not a child','NotFoundError');return removeChildBase.call(this,node)});
    for(const proto of [Element.prototype,CharacterData.prototype]){
      member(proto,'remove',function(){if(this.parentNode)this.parentNode.removeChild(this)});
      member(proto,'before',function(...nodes){const parent=this.parentNode;if(!parent)return;for(const node of nodes)parent.insertBefore(node instanceof Node?node:document.createTextNode(String(node)),this)});
      member(proto,'after',function(...nodes){const parent=this.parentNode;if(!parent)return;const next=this.nextSibling;for(const node of nodes)parent.insertBefore(node instanceof Node?node:document.createTextNode(String(node)),next)});
      member(proto,'replaceWith',function(...nodes){const parent=this.parentNode;if(!parent)return;this.before(...nodes);if(this.parentNode===parent)parent.removeChild(this)});
    }
    for(const proto of [Element.prototype,DocumentFragment.prototype]){
      member(proto,'append',function(...nodes){for(const node of nodes)this.appendChild(node instanceof Node?node:document.createTextNode(String(node)))});
      member(proto,'prepend',function(...nodes){const before=this.firstChild;for(const node of nodes)this.insertBefore(node instanceof Node?node:document.createTextNode(String(node)),before)});
      member(proto,'replaceChildren',function(...nodes){const fragment=document.createDocumentFragment();fragment.append(...nodes);while(this.firstChild)this.removeChild(this.firstChild);this.appendChild(fragment)});
    }
    const attributeChanged=(node,name,oldValue)=>{
      queueRecord('attributes',node,{attributeName:name,oldValue});
      const definition=upgraded.get(node);
      if(definition&&definition.attributes.includes(name))reaction(node,'attributeChangedCallback',[name,oldValue,node.getAttribute(name),null]);
    };
    for(const name of ['setAttribute','removeAttribute']){
      const original=Element.prototype[name];
      Object.defineProperty(Element.prototype,name,{value:function(key,value){
        if(!observers.size&&!definitions.size&&!compatibilityElementState.hasModal?.())return original.apply(this,arguments);
        key=this.namespaceURI==='http://www.w3.org/1999/xhtml'?String(key).toLowerCase():String(key);const old=this.getAttribute(key);
        const result=original.apply(this,arguments);
        if(name==='setAttribute'||old!==null)attributeChanged(this,key,old);
        return result;
      },writable:true,configurable:true,enumerable:true});
    }
    // Mutation algorithms own one record transaction. Recursive fragment
    // insertion and replaceChild's remove/insert steps must not emit duplicates.
    const mutationOriginals=Object.fromEntries(['appendChild','insertBefore','removeChild','replaceChild'].map(name=>[name,Node.prototype[name]]));
    const nodeSnapshot=node=>({node,parent:node.parentNode,previousSibling:node.previousSibling,nextSibling:node.nextSibling,connected:node.isConnected});
    const detachedReaction=entry=>{if(entry.connected)walk(entry.node,n=>{compatibilityElementState.detached?.(n);if(definitions.size&&upgraded.get(n))reaction(n,'disconnectedCallback')})};
    const insertedReaction=node=>{if(definitions.size&&!customElementCloneInert&&!templateTreeIsInert(node))walk(node,n=>{if(upgraded.get(n)){if(n.isConnected)reaction(n,'connectedCallback')}else upgrade(n)})};
    const emitRemoval=entry=>{if(entry.parent)queueRecord('childList',entry.parent,{removedNodes:[entry.node],previousSibling:entry.previousSibling,nextSibling:entry.nextSibling})};
    for(const method of ['appendChild','insertBefore','removeChild','replaceChild']){
      const original=mutationOriginals[method];
      member(Node.prototype,method,function(node,reference){
        if(mutationDepth||!observers.size&&!definitions.size&&!compatibilityElementState.hasModal?.())return original.apply(this,arguments);
        if(!(node instanceof Node))return original.apply(this,arguments);
        if(method==='removeChild'){if(node.parentNode!==this)throw new DOMException('Not a child','NotFoundError')}else prepareInsertion(this,node,method==='appendChild'?null:reference);
        const fragment=node instanceof DocumentFragment,children=fragment?Array.from(node.childNodes):[node],entries=children.map(nodeSnapshot);
        const removed=method==='removeChild'?node:method==='replaceChild'?reference:null;
        const removedEntry=removed instanceof Node?nodeSnapshot(removed):null;
        // Snapshot transient registrations while the old ancestor chain exists.
        for(const entry of entries)if(entry.parent)retainRemoved(entry.node);
        if(removedEntry)retainRemoved(removed);
        let result;mutationDepth++;reactionDepth++;
        try{
          if(method==='replaceChild'&&node===reference&&node.parentNode===this){
            const next=node.nextSibling;mutationOriginals.removeChild.call(this,node);mutationOriginals.insertBefore.call(this,node,next);result=node;
          }else result=original.apply(this,arguments);
        }catch(error){reactionDepth--;throw error}finally{mutationDepth--}
        try{
          if(method==='removeChild'){
            emitRemoval(removedEntry);detachedReaction(removedEntry);
          }else{
            if(fragment){if(children.length)queueRecord('childList',node,{removedNodes:children})}
            else for(const entry of entries)emitRemoval(entry);
            if(method==='replaceChild'){
              const previous=children.length?children[0].previousSibling:removedEntry.previousSibling;
              const next=children.length?children[children.length-1].nextSibling:removedEntry.nextSibling;
              const removedNodes=node===reference?[]:[reference];
              if(children.length||removedNodes.length)queueRecord('childList',this,{addedNodes:children,removedNodes,previousSibling:previous,nextSibling:next});
            }else if(children.length)queueRecord('childList',this,{addedNodes:children,previousSibling:children[0].previousSibling,nextSibling:children[children.length-1].nextSibling});
            for(const entry of entries)if(entry.parent)detachedReaction(entry);
            if(removedEntry&&removed!==node)detachedReaction(removedEntry);
            for(const child of children)insertedReaction(child);
          }
        }finally{reactionDepth--;flushReactions()}
        return result;
      });
    }
    for(const [prototype,key] of [[Element.prototype,'innerHTML'],[Element.prototype,'textContent'],[Node.prototype,'textContent']]){
      const original=Object.getOwnPropertyDescriptor(prototype,key);if(!original?.set)continue;
      Object.defineProperty(prototype,key,{...original,set(value){
        if(mutationDepth||!observers.size&&!definitions.size&&!compatibilityElementState.hasModal?.())return original.set.call(this,value);
        const character=this.nodeType===3||this.nodeType===8,old=character?this.textContent:null;
        const removed=character?[]:Array.from(this.childNodes),entries=removed.map(nodeSnapshot);
        for(const node of removed)retainRemoved(node);
        mutationDepth++;reactionDepth++;
        try{original.set.call(this,value)}catch(error){reactionDepth--;throw error}finally{mutationDepth--}
        try{
          if(character)queueRecord('characterData',this,{oldValue:old});
          else{
            const added=Array.from(this.childNodes);if(added.length||removed.length)queueRecord('childList',this,{addedNodes:added,removedNodes:removed});
            for(const entry of entries)detachedReaction(entry);for(const node of added)insertedReaction(node);
          }
        }finally{reactionDepth--;flushReactions()}
      }});
    }
    // Markup insertion shares the child-list transaction and CE reaction queue
    // with ordinary node mutations; the host parser remains the tree authority.
    const markupMutation=(parent,removed,invoke)=>{
      if(!parent||mutationDepth||!observers.size&&!definitions.size&&!compatibilityElementState.hasModal?.())return invoke();
      const before=Array.from(parent.childNodes),prior=new Set(before),entry=removed?nodeSnapshot(removed):null;
      if(removed)retainRemoved(removed);
      mutationDepth++;reactionDepth++;
      try{invoke()}catch(error){reactionDepth--;throw error}finally{mutationDepth--}
      try{
        const added=Array.from(parent.childNodes).filter(node=>!prior.has(node));
        const gone=removed&&removed.parentNode!==parent?[removed]:[];
        if(added.length||gone.length)queueRecord('childList',parent,{addedNodes:added,removedNodes:gone,previousSibling:added.length?added[0].previousSibling:entry?.previousSibling||null,nextSibling:added.length?added[added.length-1].nextSibling:entry?.nextSibling||null});
        for(const node of added)insertedReaction(node);
        if(gone.length)detachedReaction(entry);
      }finally{reactionDepth--;flushReactions()}
    };
    const adjacentHTML=Element.prototype.insertAdjacentHTML;
    member(Element.prototype,'insertAdjacentHTML',function(position,text){
      if(arguments.length<2)return adjacentHTML.apply(this,arguments);
      position=String(position).toLowerCase();text=String(text);
      const parent=position==='beforebegin'||position==='afterend'?this.parentNode:this;
      return markupMutation(parent,null,()=>adjacentHTML.call(this,position,text));
    });
    const outerHTML=Object.getOwnPropertyDescriptor(Element.prototype,'outerHTML');
    Object.defineProperty(Element.prototype,'outerHTML',{...outerHTML,set(value){
      value=value==null?'':String(value);
      return markupMutation(this.parentNode,this,()=>outerHTML.set.call(this,value));
    }});
    const convertMutationNodes=values=>{
      const nodes=values.map(value=>value instanceof Node?value:document.createTextNode(String(value)));
      if(nodes.length===1)return nodes[0];
      const fragment=document.createDocumentFragment();for(const node of nodes)fragment.appendChild(node);return fragment;
    };
    for(const prototype of [Element.prototype,DocumentFragment.prototype]){
      member(prototype,'append',function(...values){if(values.length)this.appendChild(convertMutationNodes(values))});
      member(prototype,'prepend',function(...values){if(values.length){const node=convertMutationNodes(values);this.insertBefore(node,this.firstChild)}});
      member(prototype,'replaceChildren',function(...values){
        const node=values.length?convertMutationNodes(values):null;
        if(node)prepareInsertion(this,node,null);
        const removed=Array.from(this.childNodes),entries=removed.map(nodeSnapshot),added=node?(node instanceof DocumentFragment?Array.from(node.childNodes):[node]):[];
        const sources=added.map(nodeSnapshot);for(const child of removed)retainRemoved(child);for(const child of added)if(child.parentNode)retainRemoved(child);
        mutationDepth++;reactionDepth++;
        try{while(this.firstChild)mutationOriginals.removeChild.call(this,this.firstChild);if(node)mutationOriginals.appendChild.call(this,node)}catch(error){reactionDepth--;throw error}finally{mutationDepth--}
        try{
          if(node instanceof DocumentFragment){if(added.length)queueRecord('childList',node,{removedNodes:added})}
          else for(const source of sources)if(source.parent&&source.parent!==this)emitRemoval(source);
          if(removed.length||added.length)queueRecord('childList',this,{addedNodes:added,removedNodes:removed});
          for(const entry of entries)detachedReaction(entry);for(const source of sources)if(source.parent!==this)detachedReaction(source);for(const child of added)insertedReaction(child);
        }finally{reactionDepth--;flushReactions()}
      });
    }
    // The null-namespace aliases share the same canonical attribute path.
    const namespaceGet=Element.prototype.getAttributeNS,namespaceHas=Element.prototype.hasAttributeNS;
    member(Element.prototype,'getAttributeNS',function(namespace,name){return namespace==null||namespace===''?this.getAttribute(name):namespaceGet.call(this,namespace,name)});
    member(Element.prototype,'hasAttributeNS',function(namespace,name){return namespace==null||namespace===''?this.hasAttribute(name):namespaceHas.call(this,namespace,name)});
    const namespaceSet=Element.prototype.setAttributeNS,namespaceRemove=Element.prototype.removeAttributeNS;
    member(Element.prototype,'setAttributeNS',function(namespace,name,value){namespace=namespace==null?'':String(namespace);name=String(name);const local=namespace?name.split(':').at(-1):name,old=this.getAttributeNS(namespace,local);const result=namespaceSet.call(this,namespace,name,value);queueRecord('attributes',this,{attributeName:local,attributeNamespace:namespace||null,oldValue:old});const definition=upgraded.get(this);if(definition&&definition.attributes.includes(local))reaction(this,'attributeChangedCallback',[local,old,this.getAttributeNS(namespace,local),namespace||null]);return result});
    member(Element.prototype,'removeAttributeNS',function(namespace,name){return namespace==null||namespace===''?this.removeAttribute(name):namespaceRemove.call(this,namespace,name)});
    const checkedTokens=tokens=>tokens.map(value=>{const token=String(value);if(!token)throw new DOMException('Empty token','SyntaxError');if(/[\t\n\f\r ]/.test(token))throw new DOMException('Whitespace in token','InvalidCharacterError');return token});
    member(DOMTokenList.prototype,'add',function(...values){const tokens=checkedTokens(values);writeDOMTokens(this,domTokens(this).concat(tokens))});
    member(DOMTokenList.prototype,'remove',function(...values){const tokens=new Set(checkedTokens(values));writeDOMTokens(this,domTokens(this).filter(value=>!tokens.has(value)))});
    member(DOMTokenList.prototype,'toggle',function(value,force){const [token]=checkedTokens([value]);if(!observers.size&&!definitions.size&&!compatibilityElementState.hasModal?.()){const state=domTokenState(this);return host.toggleToken(elementSlot(state.element).nodeId,state.attribute,token,force===undefined?-1:Boolean(force)?1:0)}const tokens=domTokens(this),has=tokens.includes(token);if(has){if(force===undefined||!Boolean(force)){writeDOMTokens(this,tokens.filter(value=>value!==token));return false}return true}if(force!==undefined&&!Boolean(force))return false;writeDOMTokens(this,[...tokens,token]);return true});

    /* shared_abort_encoding */
    Object.defineProperty(Element.prototype,'matches',{value:function(selector){return host.matches(elementSlot(this).nodeId,String(selector))},writable:true,configurable:true,enumerable:true});
    Object.defineProperty(Element.prototype,'webkitMatchesSelector',{value:Element.prototype.matches,writable:true,configurable:true,enumerable:true});
    Object.defineProperty(Document.prototype,'defaultView',{get(){return window},configurable:true,enumerable:true});
    let focused=null,keyboardFocus=true;
    compatibilityElementState.isDefined=node=>{
      const slot=elementSlot(node);
      return slot?.namespaceURI!=='http://www.w3.org/1999/xhtml'||!validName(node.localName)||!!upgraded.get(node);
    };
    compatibilityElementState.focused=()=>focused&&focused.isConnected?focused:null;
    const modalDialogs=new Set(),dialogReturnValues=new WeakMap();
    compatibilityElementState.hasModal=()=>modalDialogs.size!==0;
    compatibilityElementState.modal=node=>modalDialogs.has(node)&&node.isConnected;
    compatibilityElementState.detached=node=>modalDialogs.delete(node);
    if(globalThis.HTMLDialogElement){
      const proto=HTMLDialogElement.prototype;
      const dialog=node=>{if(!(node instanceof HTMLDialogElement))throw new TypeError('Illegal invocation');return node};
      Object.defineProperties(proto,{
        open:{get(){return dialog(this).hasAttribute('open')},set(value){dialog(this);if(value)this.setAttribute('open','');else this.removeAttribute('open')},configurable:true,enumerable:true},
        returnValue:{get(){dialog(this);return dialogReturnValues.get(this)||''},set(value){dialog(this);dialogReturnValues.set(this,String(value))},configurable:true,enumerable:true},
        show:{value:function(){dialog(this);if(this.open){if(modalDialogs.has(this))throw new DOMException('Dialog is already modal','InvalidStateError');return}this.open=true},writable:true,configurable:true,enumerable:true},
        showModal:{value:function(){dialog(this);if(this.open){if(!modalDialogs.has(this))throw new DOMException('Dialog is already open','InvalidStateError');return}if(!this.isConnected)throw new DOMException('Dialog is not connected','InvalidStateError');modalDialogs.add(this);this.open=true},writable:true,configurable:true,enumerable:true},
        close:{value:function(value){dialog(this);if(!this.open)return;if(value!==undefined)this.returnValue=value;this.open=false;modalDialogs.delete(this);setTimeout(()=>dispatchTrusted(this,new Event('close')),0)},writable:true,configurable:true,enumerable:true}
      });
    }
    compatibilityElementState.focusVisible=node=>compatibilityElementState.focused()===node&&
      (keyboardFocus||node.localName==='textarea'||node.localName==='input'&&!['button','checkbox','color','file','hidden','image','radio','range','reset','submit'].includes(String(node.type)));
    compatibilityElementState.noteTrustedInput=type=>{
      if(type==='keydown')keyboardFocus=true;
      else if(type==='mousedown'||type==='pointerdown'||type==='touchstart')keyboardFocus=false;
    };
    document.addEventListener('keydown',event=>{if(event.isTrusted&&!event.altKey&&!event.ctrlKey&&!event.metaKey)keyboardFocus=true},true);
    for(const type of ['mousedown','pointerdown','touchstart'])document.addEventListener(type,event=>{if(event.isTrusted)keyboardFocus=false},true);
    Object.defineProperty(Document.prototype,'activeElement',{get(){return focused&&focused.isConnected?focused:this.body||this.documentElement},configurable:true,enumerable:true});
    Object.defineProperty(HTMLElement.prototype,'focus',{value:function(){if(!this.isConnected||focused===this)return;const previous=focused;focused=this;if(previous)dispatchTrusted(previous,new Event('blur'));dispatchTrusted(this,new Event('focus'));dispatchTrusted(this,new Event('focusin',{bubbles:true}))},writable:true,configurable:true,enumerable:true});
    Object.defineProperty(HTMLElement.prototype,'blur',{value:function(){if(focused!==this)return;focused=null;dispatchTrusted(this,new Event('blur'));dispatchTrusted(this,new Event('focusout',{bubbles:true}))},writable:true,configurable:true,enumerable:true});
    const mediaSlots=new WeakMap();
    class MediaQueryList extends EventTarget {
      constructor(query){super();mediaSlots.set(this,{query:String(query),onchange:null})}
      get media(){return mediaSlots.get(this).query}get matches(){return !!host.media(this.media)}
      get onchange(){return mediaSlots.get(this).onchange}set onchange(value){const s=mediaSlots.get(this);if(s.onchange)this.removeEventListener('change',s.onchange);s.onchange=value;if(typeof value==='function')this.addEventListener('change',value)}
      addListener(callback){this.addEventListener('change',callback)}removeListener(callback){this.removeEventListener('change',callback)}
    }
    expose('MediaQueryList',MediaQueryList);expose('matchMedia',query=>new MediaQueryList(query));
    expose('requestIdleCallback',function(callback,options={}){if(typeof callback!=='function')throw new TypeError('Expected callback');const requested=performance.now();return setTimeout(()=>{const start=performance.now(),didTimeout=options.timeout!==undefined&&start-requested>=Number(options.timeout);callback({didTimeout,timeRemaining:()=>didTimeout?0:Math.max(0,50-(performance.now()-start))})},1)});
    expose('cancelIdleCallback',id=>clearTimeout(id));
    const resizeSlots=new WeakMap();
    class ResizeObserver {
      constructor(callback){if(typeof callback!=='function')throw new TypeError('Expected callback');resizeSlots.set(this,{callback,targets:new Map(),timer:null})}
      observe(target,options={}){if(!(target instanceof Element))throw new TypeError('Expected Element');const s=resizeSlots.get(this);s.targets.set(target,{box:options.box||'content-box',width:null,height:null});if(s.timer!==null)return;s.timer=requestAnimationFrame(()=>{s.timer=null;const entries=[];for(const [node,previous] of s.targets){const rect=node.getBoundingClientRect();if(rect.width===previous.width&&rect.height===previous.height)continue;previous.width=rect.width;previous.height=rect.height;entries.push({target:node,contentRect:new DOMRect(0,0,rect.width,rect.height),contentBoxSize:[{inlineSize:rect.width,blockSize:rect.height}],borderBoxSize:[{inlineSize:rect.width,blockSize:rect.height}],devicePixelContentBoxSize:[{inlineSize:rect.width*devicePixelRatio,blockSize:rect.height*devicePixelRatio}]})}if(entries.length)s.callback.call(this,entries,this)})}
      unobserve(target){resizeSlots.get(this).targets.delete(target)}
      disconnect(){const s=resizeSlots.get(this);s.targets.clear();if(s.timer!==null)cancelAnimationFrame(s.timer);s.timer=null}
    }
    expose('ResizeObserver',ResizeObserver);
    const detailSlots=new WeakMap();
    class CustomEvent extends Event {constructor(type,init={}){super(type,init);detailSlots.set(this,init.detail??null)}get detail(){return detailSlots.get(this)}initCustomEvent(type,bubbles,cancelable,detail){const s=eventSlots.get(this);s.type=String(type);s.bubbles=!!bubbles;s.cancelable=!!cancelable;detailSlots.set(this,detail)}}
    expose('CustomEvent',CustomEvent);
    Object.defineProperty(Document.prototype,'domain',{get(){return location.hostname},configurable:true,enumerable:true});
    Object.defineProperty(Node.prototype,'getRootNode',{value:function(options={}){let node=this;while(node.parentNode)node=node.parentNode;if(options.composed&&node instanceof ShadowRoot)return node.host.getRootNode(options);return node},writable:true,configurable:true,enumerable:true});
    Object.defineProperty(Node.prototype,'cloneNode',{value:function(deep=false){
      let copy;
      if(this.nodeType===1){copy=this.namespaceURI&&this.namespaceURI!=='http://www.w3.org/1999/xhtml'?document.createElementNS(this.namespaceURI,this.localName):document.createElement(this.localName);for(const name of this.getAttributeNames()){const attr=this.getAttributeNode(name);if(attr.namespaceURI)copy.setAttributeNS(attr.namespaceURI,name,attr.value);else copy.setAttribute(name,attr.value)}}
      else if(this.nodeType===3)copy=document.createTextNode(this.textContent);
      else if(this.nodeType===8)copy=document.createComment(this.textContent);
      else if(this.nodeType===11)copy=document.createDocumentFragment();
      else throw new DOMException('Node cannot be cloned','NotSupportedError');
      if(deep){for(const child of Array.from(this.childNodes))copy.appendChild(child.cloneNode(true));if(typeof globalThis.HTMLTemplateElement==='function'&&this instanceof globalThis.HTMLTemplateElement)for(const child of Array.from(this.content.childNodes))copy.content.appendChild(child.cloneNode(true))}return copy;
    },writable:true,configurable:true,enumerable:true});
    Object.defineProperty(Document.prototype,'importNode',{value:function(node,deep=false){return node.cloneNode(deep)},writable:true,configurable:true,enumerable:true});
    Object.defineProperty(Request.prototype,'signal',{get(){const s=requestSlots.get(this);if(!s.signal)s.signal=new AbortSignal(hostToken);return s.signal},configurable:true,enumerable:true});
    for(const [ctor,property,attribute] of [[globalThis.HTMLMetaElement,'name','name'],[globalThis.HTMLStyleElement,'type','type']])if(ctor)Object.defineProperty(ctor.prototype,property,{get(){return this.getAttribute(attribute)||''},set(value){this.setAttribute(attribute,String(value))},enumerable:true,configurable:true});
    Object.defineProperty(Response.prototype,'clone',{value:function(){const s=responseSlots.get(this);if(s.bodyUsed)throw new TypeError('Body already used');const copy=new Response(s.body,{status:s.status,statusText:s.statusText,headers:new Headers(s.headers)});Object.assign(responseSlots.get(copy),{url:s.url,type:s.type,redirected:s.redirected});return copy},writable:true,configurable:true,enumerable:true});
    Object.defineProperty(Node.prototype,'nodeValue',{get(){return this.nodeType===3||this.nodeType===8?this.textContent:null},set(value){if(this.nodeType===3||this.nodeType===8)this.textContent=value??''},enumerable:true,configurable:true});
    Object.defineProperty(Document.prototype,'firstElementChild',{get(){return this.documentElement},enumerable:true,configurable:true});
    Object.defineProperty(Document.prototype,'getElementsByName',{value:function(name){return this.querySelectorAll('[name="'+String(name).replace(/\\/g,'\\\\').replace(/"/g,'\\"')+'"]')},writable:true,enumerable:true,configurable:true});
    // HTMLCollection is live: class mutations, insertion and removal are read
    // from the canonical tree on every access, with ASCII whitespace tokens.
    for(const ctor of [Document,Element])Object.defineProperty(ctor.prototype,'getElementsByClassName',{value:function(names){
      if(arguments.length===0)throw new TypeError('Expected class names');
      const root=this,normalize=value=>(root instanceof Document?root:root.ownerDocument)?.compatMode==='BackCompat'?value.replace(/[A-Z]/g,c=>c.toLowerCase()):value;
      const tokens=[...new Set(normalize(String(names)).split(/[\t\n\f\r ]+/).filter(Boolean))];
      return htmlCollection(()=>tokens.length?Array.from(root.querySelectorAll('*')).filter(node=>{const classes=normalize(node.getAttribute('class')||'').split(/[\t\n\f\r ]+/);return tokens.every(token=>classes.includes(token))}).map(node=>host.nodeData(elementSlot(node).nodeId)):[]);
    },writable:true,enumerable:true,configurable:true});
    Object.defineProperty(DocumentFragment.prototype,'textContent',{get(){return Array.from(this.childNodes).map(node=>node.textContent||'').join('')},set(value){for(const child of Array.from(this.childNodes))this.removeChild(child);if(value!=null&&String(value)!=='')this.appendChild(document.createTextNode(String(value)))},enumerable:true,configurable:true});
    if(typeof globalThis.CSS!=='undefined')Object.defineProperty(globalThis.CSS,'escape',{value:function(value){const s=String(value);let out='';for(let i=0;i<s.length;i++){const c=s.charCodeAt(i);if(c===0){out+='\ufffd';continue}if(c<32||c===127||i===0&&c>=48&&c<=57||i===1&&c>=48&&c<=57&&s[0]==='-'){out+='\\'+c.toString(16)+' ';continue}if(i===0&&s.length===1&&s[i]==='-'){out+='\\-';continue}out+=c>=128||c===45||c===95||c>=48&&c<=57||c>=65&&c<=90||c>=97&&c<=122?s[i]:'\\'+s[i]}return out},writable:true,configurable:true,enumerable:true});
    if(typeof globalThis.ClipboardItem==='function'){
      const items=new WeakMap();
      class ClipboardItem {constructor(data,options={}){const types=Object.keys(data);if(!types.length)throw new TypeError('Empty clipboard item');items.set(this,{data:{...data},types,presentationStyle:options.presentationStyle||'unspecified'})}get types(){return Object.freeze(items.get(this).types.slice())}get presentationStyle(){return items.get(this).presentationStyle}async getType(type){const s=items.get(this);if(!s.types.includes(String(type)))throw new DOMException('Type not found','NotFoundError');const value=await s.data[type];return typeof value==='string'?new Blob([value],{type}):value}static supports(type){return ['text/plain','text/html','image/png'].includes(String(type))}}
      expose('ClipboardItem',ClipboardItem);
    }
  }
