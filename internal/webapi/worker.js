(function(host){
  'use strict';
  const listenerTargets=new WeakMap(),handlers={message:null,messageerror:null,error:null};
  const targetListeners=target=>{let result=listenerTargets.get(target);if(!result){result=new Map();listenerTargets.set(target,result)}return result};
  class EventTarget {
    addEventListener(type,listener,options={}){if(typeof listener!=='function'&&!(listener&&typeof listener.handleEvent==='function'))return;const listeners=targetListeners(this),key=String(type),capture=typeof options==='boolean'?options:!!options?.capture,values=listeners.get(key)||[];if(!values.some(x=>x.listener===listener&&x.capture===capture&&!x.removed))values.push({listener,capture,once:!!options?.once,removed:false});listeners.set(key,values)}
    removeEventListener(type,listener,options={}){const listeners=targetListeners(this),key=String(type),capture=typeof options==='boolean'?options:!!options?.capture,values=listeners.get(key)||[];for(const record of values)if(record.listener===listener&&record.capture===capture)record.removed=true;listeners.set(key,values.filter(record=>!record.removed))}
    dispatchEvent(event){for(const record of (targetListeners(this).get(event.type)||[]).slice()){if(record.removed)continue;if(record.once)EventTarget.prototype.removeEventListener.call(this,event.type,record.listener,record.capture);const listener=record.listener;if(typeof listener==='function')listener.call(this,event);else listener.handleEvent(event)}const handler=this===globalThis?handlers[event.type]:this['on'+event.type];if(typeof handler==='function')handler.call(this,event);return true}
  }
  const eventSlots=new WeakMap(),messageEventSlots=new WeakMap();
  class Event {
    constructor(type){eventSlots.set(this,{type:String(type),trusted:false});Object.defineProperty(this,'isTrusted',{get:()=>!!eventSlots.get(this).trusted,enumerable:true,configurable:false})}
    get type(){return eventSlots.get(this).type}
  }
  class MessageEvent extends Event {
    constructor(type,init={}){super(type);messageEventSlots.set(this,{data:init.data,origin:init.origin||'',source:null,ports:[]})}
    get data(){return messageEventSlots.get(this).data}
    get origin(){return messageEventSlots.get(this).origin}
    get source(){return messageEventSlots.get(this).source}
    get ports(){return messageEventSlots.get(this).ports.slice()}
  }
  class WorkerNavigator { constructor(){throw new TypeError('Illegal constructor')} }
  class WorkerLocation { constructor(){throw new TypeError('Illegal constructor')} toString(){return this.href} }
  class Crypto { constructor(){throw new TypeError('Illegal constructor')} getRandomValues(view){if(!ArrayBuffer.isView(view)||view instanceof Float32Array||view instanceof Float64Array||view instanceof DataView)throw new TypeError("Failed to execute 'getRandomValues' on 'Crypto': parameter 1 is not of type 'ArrayBufferView'.");if(view.byteLength>65536)throw new DOMException('The ArrayBufferView byte length exceeds 65536 bytes','QuotaExceededError');const bytes=host.randomBytes(view.byteLength),raw=new Uint8Array(view.buffer,view.byteOffset,view.byteLength);for(let i=0;i<raw.length;i++)raw[i]=bytes[i];return view} randomUUID(){return host.randomUUID()} }
  class SubtleCrypto { constructor(){throw new TypeError('Illegal constructor')} digest(algorithm,data){const name=typeof algorithm==='string'?algorithm:algorithm&&algorithm.name;if(!ArrayBuffer.isView(data)&&!(data instanceof ArrayBuffer))return Promise.reject(new TypeError('Data must be an ArrayBuffer or ArrayBufferView'));const bytes=data instanceof ArrayBuffer?new Uint8Array(data):new Uint8Array(data.buffer,data.byteOffset,data.byteLength);return Promise.resolve(host.cryptoDigest(String(name),Array.from(bytes))).then(result=>new Uint8Array(result).buffer)} }
  class Performance { constructor(){throw new TypeError('Illegal constructor')} get timeOrigin(){return host.performanceTimeOrigin()} now(){return host.performanceNow()} getEntries(){return[]} getEntriesByType(){return[]} getEntriesByName(){return[]} }
  const trustedValueSlots=new WeakMap(),trustedPolicySlots=new WeakMap(),trustedFactorySlots=new WeakMap(),illegalTrusted=name=>{throw new TypeError('Illegal constructor: '+name)},trustedValue=(Ctor,value)=>{const result=Object.create(Ctor.prototype);trustedValueSlots.set(result,{type:Ctor,value:String(value)});return result};
  class TrustedHTML { constructor(){illegalTrusted('TrustedHTML')} toString(){return trustedValueSlots.get(this)?.value} toJSON(){return this.toString()} }
  class TrustedScript { constructor(){illegalTrusted('TrustedScript')} toString(){return trustedValueSlots.get(this)?.value} toJSON(){return this.toString()} }
  class TrustedScriptURL { constructor(){illegalTrusted('TrustedScriptURL')} toString(){return trustedValueSlots.get(this)?.value} toJSON(){return this.toString()} }
  class TrustedTypePolicy { constructor(token,name,options){if(token!==host.token())illegalTrusted('TrustedTypePolicy');trustedPolicySlots.set(this,{name,options:options||{}})} get name(){return trustedPolicySlots.get(this).name} createHTML(input,...args){const state=trustedPolicySlots.get(this),callback=state.options.createHTML;if(typeof callback!=='function')throw new TypeError('Policy '+state.name+' disallows creating TrustedHTML');return trustedValue(TrustedHTML,callback(String(input),...args))} createScript(input,...args){const state=trustedPolicySlots.get(this),callback=state.options.createScript;if(typeof callback!=='function')throw new TypeError('Policy '+state.name+' disallows creating TrustedScript');return trustedValue(TrustedScript,callback(String(input),...args))} createScriptURL(input,...args){const state=trustedPolicySlots.get(this),callback=state.options.createScriptURL;if(typeof callback!=='function')throw new TypeError('Policy '+state.name+' disallows creating TrustedScriptURL');return trustedValue(TrustedScriptURL,callback(String(input),...args))} }
  class TrustedTypePolicyFactory { constructor(token){if(token!==host.token())illegalTrusted('TrustedTypePolicyFactory');trustedFactorySlots.set(this,{defaultPolicy:null})} createPolicy(name,options={}){const policy=new TrustedTypePolicy(host.token(),String(name),options);if(String(name)==='default')trustedFactorySlots.get(this).defaultPolicy=policy;return policy} isHTML(value){return trustedValueSlots.get(value)?.type===TrustedHTML} isScript(value){return trustedValueSlots.get(value)?.type===TrustedScript} isScriptURL(value){return trustedValueSlots.get(value)?.type===TrustedScriptURL} get emptyHTML(){return trustedValue(TrustedHTML,'')} get emptyScript(){return trustedValue(TrustedScript,'')} get defaultPolicy(){return trustedFactorySlots.get(this).defaultPolicy} getAttributeType(tagName,attribute){const tag=String(tagName).toLowerCase(),name=String(attribute).toLowerCase();if(name.startsWith('on'))return'TrustedScript';if(tag==='script'&&name==='src')return'TrustedScriptURL';if(tag==='iframe'&&name==='srcdoc')return'TrustedHTML';return null} getPropertyType(tagName,property){const tag=String(tagName).toLowerCase(),name=String(property);if(name==='innerHTML'||name==='outerHTML'||(tag==='iframe'&&name==='srcdoc'))return'TrustedHTML';if(tag==='script'&&name==='src')return'TrustedScriptURL';if(tag==='script'&&(name==='text'||name==='textContent'||name==='innerText'))return'TrustedScript';if(name.startsWith('on'))return'TrustedScript';return null} getTypeMapping(){return{http:{script:{src:'TrustedScriptURL',text:'TrustedScript'},iframe:{srcdoc:'TrustedHTML'},'*':{innerHTML:'TrustedHTML',outerHTML:'TrustedHTML'}}}} }
  const readableSlots=new WeakMap(),readerSlots=new WeakMap();
  class ReadableStreamDefaultController { constructor(stream){this.stream=stream} enqueue(value){const state=readableSlots.get(this.stream);if(state.reads.length)state.reads.shift()({value,done:false});else state.queue.push(value)} close(){const state=readableSlots.get(this.stream);state.closed=true;while(state.reads.length)state.reads.shift()({value:undefined,done:true})} error(error){const state=readableSlots.get(this.stream);state.error=error;while(state.rejects.length)state.rejects.shift()(error)} }
  class ReadableStreamDefaultReader { constructor(stream){const state=readableSlots.get(stream);if(!state||state.locked)throw new TypeError('ReadableStream is locked');state.locked=true;readerSlots.set(this,stream)} read(){const stream=readerSlots.get(this),state=readableSlots.get(stream);if(state.queue.length)return Promise.resolve({value:state.queue.shift(),done:false});if(state.error)return Promise.reject(state.error);if(state.closed)return Promise.resolve({value:undefined,done:true});return new Promise((resolve,reject)=>{state.reads.push(resolve);state.rejects.push(reject)})} releaseLock(){const stream=readerSlots.get(this);if(stream){readableSlots.get(stream).locked=false;readerSlots.delete(this)}} cancel(reason){const stream=readerSlots.get(this);return stream?stream.cancel(reason):Promise.reject(new TypeError('Reader has been released'))} }
  class ReadableStream { constructor(source={}){const state={queue:[],reads:[],rejects:[],closed:false,error:null,locked:false,source};readableSlots.set(this,state);const controller=new ReadableStreamDefaultController(this);try{Promise.resolve(typeof source.start==='function'?source.start(controller):undefined).catch(error=>controller.error(error))}catch(error){controller.error(error)}} get locked(){return readableSlots.get(this).locked} getReader(){return new ReadableStreamDefaultReader(this)} cancel(reason){const state=readableSlots.get(this);state.closed=true;state.queue.length=0;return Promise.resolve(typeof state.source.cancel==='function'?state.source.cancel(reason):undefined)} }
  // Use the internal brand/value, never a user-defined toString or prototype.
  const trustedValueState=WeakMap.prototype.get.bind(trustedValueSlots);
  const evalSourceResolver=value=>{const state=trustedValueState(value);return state?.type===TrustedScript?state.value:undefined};
  Object.assign(globalThis,{EventTarget,Event,MessageEvent,WorkerNavigator,WorkerLocation,Crypto,SubtleCrypto,Performance,TrustedHTML,TrustedScript,TrustedScriptURL,TrustedTypePolicy,TrustedTypePolicyFactory,ReadableStream,ReadableStreamDefaultController,ReadableStreamDefaultReader});
  const navigatorData=host.navigator(),workerSubtle=Object.create(SubtleCrypto.prototype);
  const workerNavigator=Object.create(WorkerNavigator.prototype),workerLocation=Object.create(WorkerLocation.prototype),workerCrypto=Object.create(Crypto.prototype),workerPerformance=Object.create(Performance.prototype),workerTrustedTypes=new TrustedTypePolicyFactory(host.token());
  for(const name of ['userAgent','appVersion','platform','languages','language','hardwareConcurrency','deviceMemory','onLine'])Object.defineProperty(WorkerNavigator.prototype,name,{get(){const value=navigatorData[name];return Array.isArray(value)?value.slice():value},enumerable:true,configurable:true});
  for(const name of ['href','origin','protocol','host','hostname','port','pathname','search','hash'])Object.defineProperty(WorkerLocation.prototype,name,{get(){return host.location()[name]},enumerable:true,configurable:true});
  for(const [prototype,tag] of [[EventTarget.prototype,'EventTarget'],[Event.prototype,'Event'],[MessageEvent.prototype,'MessageEvent'],[WorkerNavigator.prototype,'WorkerNavigator'],[WorkerLocation.prototype,'WorkerLocation'],[Crypto.prototype,'Crypto'],[SubtleCrypto.prototype,'SubtleCrypto'],[Performance.prototype,'Performance'],[TrustedHTML.prototype,'TrustedHTML'],[TrustedScript.prototype,'TrustedScript'],[TrustedScriptURL.prototype,'TrustedScriptURL'],[TrustedTypePolicy.prototype,'TrustedTypePolicy'],[TrustedTypePolicyFactory.prototype,'TrustedTypePolicyFactory'],[ReadableStream.prototype,'ReadableStream'],[ReadableStreamDefaultController.prototype,'ReadableStreamDefaultController'],[ReadableStreamDefaultReader.prototype,'ReadableStreamDefaultReader']])Object.defineProperty(prototype,Symbol.toStringTag,{value:tag,configurable:true});
  if(host.isSecureContext())Object.defineProperty(Crypto.prototype,'subtle',{get(){return workerSubtle},enumerable:true,configurable:true});
  Object.defineProperty(globalThis,'trustedTypes',{value:workerTrustedTypes,writable:false,enumerable:true,configurable:true});
  if(!host.isSecureContext()){delete Crypto.prototype.randomUUID}
  const operations={
    addEventListener:(...args)=>EventTarget.prototype.addEventListener.apply(globalThis,args),
    removeEventListener:(...args)=>EventTarget.prototype.removeEventListener.apply(globalThis,args),
    dispatchEvent:(...args)=>EventTarget.prototype.dispatchEvent.apply(globalThis,args),
    postMessage:data=>host.postMessage(data),
    close:()=>host.close(),
    setTimeout:(fn,delay=0,...args)=>host.setTimer(()=>fn(...args),Number(delay),false),
    setInterval:(fn,delay=0,...args)=>host.setTimer(()=>fn(...args),Number(delay),true),
    clearTimeout:id=>host.clearTimer(Number(id)),
    clearInterval:id=>host.clearTimer(Number(id)),
  };
  Object.assign(globalThis,operations);
  globalThis.__deliver=data=>{const event=new MessageEvent('message',{data});eventSlots.get(event).trusted=true;return operations.dispatchEvent(event)};
  globalThis.__applyWorkerExposure=exposure=>{
    const expected=new Map((exposure.properties||[]).map(property=>[property.name,property]));
    for(const name of Object.getOwnPropertyNames(globalThis)){
      if(name.startsWith('__')||expected.has(name))continue;
      const descriptor=Object.getOwnPropertyDescriptor(globalThis,name);
      if(descriptor&&descriptor.configurable)delete globalThis[name];
    }
    for(const property of exposure.properties||[]){
      if(Object.prototype.hasOwnProperty.call(globalThis,property.name))continue;
      let value;
      if(property.valueType==='function'){
        const functionName=property.functionName||property.name;
        value={[functionName]:function(){return host.semanticMissing('DedicatedWorkerGlobalScope.'+property.name)}}[functionName];
        if(property.functionLength!==null&&property.functionLength!==undefined)Object.defineProperty(value,'length',{value:property.functionLength,configurable:true});
      }else if(property.valueType==='boolean')value=false;
      else if(property.valueType==='number')value=0;
      else if(property.valueType==='string')value='';
      else value={};
      Object.defineProperty(globalThis,property.name,{value,writable:property.writable!==false,enumerable:property.enumerable,configurable:property.configurable});
    }
  };
  globalThis.__applyWorkerPrototypeExposure=exposure=>{
    for(const [interfaceName,properties] of Object.entries(exposure.prototypes||{})){
      const ctor=globalThis[interfaceName],prototype=ctor&&ctor.prototype;if(!prototype)continue;
      const expected=new Set(properties.map(property=>property.name));
      for(const name of Object.getOwnPropertyNames(prototype)){if(!expected.has(name)){const descriptor=Object.getOwnPropertyDescriptor(prototype,name);if(descriptor&&descriptor.configurable)delete prototype[name]}}
    }
  };
  globalThis.__finishWorkerSurface=()=>{
    const hostToken=host.token(),markNative=()=>{};
    const expose=(name,value)=>Object.defineProperty(globalThis,name,{value,writable:true,configurable:true});
    const dispatchTrusted=(target,event)=>{eventSlots.get(event).trusted=true;return EventTarget.prototype.dispatchEvent.call(target,event)};
    /* shared_worker_fetch */
    globalThis.__mimicEvalSourceResolver=evalSourceResolver;
    const workerPrototype=globalThis.WorkerGlobalScope&&globalThis.WorkerGlobalScope.prototype;
    const dedicatedPrototype=globalThis.DedicatedWorkerGlobalScope&&globalThis.DedicatedWorkerGlobalScope.prototype;
    if(workerPrototype){
	  Object.defineProperty(workerPrototype,'fetch',{value:globalThis.fetch,writable:true,enumerable:true,configurable:true});
	  delete globalThis.fetch;
	  Object.defineProperty(workerPrototype,Symbol.toStringTag,{value:'WorkerGlobalScope',configurable:true});
      for(const name of ['setTimeout','setInterval','clearTimeout','clearInterval'])Object.defineProperty(workerPrototype,name,{value:operations[name],writable:true,enumerable:true,configurable:true});
      for(const name of ['onerror','onmessageerror'])Object.defineProperty(workerPrototype,name,{get(){return handlers[name.slice(2)]},set(value){handlers[name.slice(2)]=typeof value==='function'?value:null},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'self',{get(){return globalThis},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'navigator',{get(){return workerNavigator},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'location',{get(){return workerLocation},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'crypto',{get(){return workerCrypto},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'performance',{get(){return workerPerformance},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'trustedTypes',{get(){return workerTrustedTypes},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'isSecureContext',{get(){return host.isSecureContext()},enumerable:true,configurable:true});
      Object.defineProperty(workerPrototype,'queueMicrotask',{value:globalThis.queueMicrotask,writable:true,enumerable:true,configurable:true});
    }
    if(dedicatedPrototype){
	  Object.defineProperty(dedicatedPrototype,Symbol.toStringTag,{value:'DedicatedWorkerGlobalScope',configurable:true});
      for(const name of ['postMessage','close'])Object.defineProperty(dedicatedPrototype,name,{value:operations[name],writable:true,enumerable:true,configurable:true});
      Object.defineProperty(dedicatedPrototype,'onmessage',{get(){return handlers.message},set(value){handlers.message=typeof value==='function'?value:null},enumerable:true,configurable:true});
      Object.setPrototypeOf(globalThis,dedicatedPrototype);
      Object.defineProperty(globalThis,Symbol.toStringTag,{value:'DedicatedWorkerGlobalScope',configurable:true});
    }
    for(const name of Object.keys(operations))delete globalThis[name];
    // A top-level `onmessage = fn` assignment is handled by the ECMAScript
    // global environment, which does not reliably perform [[Set]] through a
    // host-installed prototype accessor in every embeddable engine. Keep the
    // DedicatedWorkerGlobalScope handler as an own global accessor, matching
    // Chrome's observable worker global and preserving ordinary script syntax.
    Object.defineProperty(globalThis,'onmessage',{get(){return handlers.message},set(value){handlers.message=typeof value==='function'?value:null},enumerable:true,configurable:true});
    Object.defineProperty(globalThis,'onmessageerror',{get(){return handlers.messageerror},set(value){handlers.messageerror=typeof value==='function'?value:null},enumerable:true,configurable:true});
    Object.defineProperty(globalThis,'postMessage',{value:operations.postMessage,writable:true,enumerable:true,configurable:true});
    Object.defineProperty(globalThis,'close',{value:operations.close,writable:true,enumerable:true,configurable:true});
    if(!host.isSecureContext()){delete Crypto.prototype.randomUUID;delete WorkerNavigator.prototype.deviceMemory}
    delete globalThis.performance;delete globalThis.queueMicrotask;
    delete globalThis.onerror;
    delete globalThis.InternalError;
  };
})(__workerHost);
