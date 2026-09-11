(function(host) {
  'use strict';
  let hostToken=host.token();
  // Snapshot restoration rebinds host state without replacing canonical objects.
  const bootstrapRestoreHooks=[];
  const bootstrapCallbacks=[];
  const registerBootstrapCallback=(name,...callbacks)=>{bootstrapCallbacks.push([name,callbacks]);host[name](...callbacks)};
  // Private brands and captured operations travel with owner references. Public
  // methods/prototypes are mutable and cannot validate or dispatch a borrowed
  // WebIDL operation. Conversion still runs in the calling method's realm.
  const realmBindings=new WeakMap();
  const bindingGet=Function.prototype.call.bind(WeakMap.prototype.get,realmBindings);
  const bindingSet=Function.prototype.call.bind(WeakMap.prototype.set,realmBindings);
  const registerRealmBinding=(value,kind,operations,unpreventable=false)=>{
    const invoke=(receiver,operation,args)=>{
      const binding=bindingGet(receiver);
      if(!binding||binding.kind!==kind)throw new TypeError('Illegal invocation');
      return binding.operations[operation](...args);
    };
    bindingSet(value,{kind,operations,invoke,unpreventable});
  };
  const requireRealmBinding=(value,kind)=>{
    const local=bindingGet(value);
    if(local?.kind===kind)return local;
    const reference=referenceGet(value);
    if(reference?.binding?.kind===kind)return reference;
    throw new TypeError('Illegal invocation');
  };
  const callRealmBinding=(receiver,binding,operation,args)=>bindingGet(receiver)===binding?
    binding.operations[operation](...args):unwrapCrossRealm(binding.frame,binding.binding.invoke)(receiver,operation,args);
  const bindingString=value=>{if(typeof value==='symbol')throw new TypeError('Cannot convert a Symbol value to a string');return String(value)};
  // Match the realm trace's once-per-(name,supported) contract before FFI.
  const tracedAccesses=new Map();
  const recordAPIAccess=(name,supported)=>{const bit=supported?1:2,seen=tracedAccesses.get(name)||0;if(seen&bit)return;tracedAccesses.set(name,seen|bit);host.apiAccess(name,supported)};
  Object.defineProperty(globalThis,'__mimicAttributeUnsafeInterfaces',{value:new Set(['WritableStream','WritableStreamDefaultWriter','TransformStream','TransformStreamDefaultController','RTCDataChannel','RTCPeerConnection','GPUDevice','Plugin','PluginArray','MimeType','MimeTypeArray']),configurable:true});
  const snapshotPublication=globalThis.__mimicSnapshotPublication;
  const engineGlobals=new Set(snapshotPublication?snapshotPublication.engineKeys:Reflect.ownKeys(globalThis));
  const illegal = n => { throw new TypeError('Illegal constructor: ' + n); };
  const def = (o,n,d) => Object.defineProperty(o,n,Object.assign({enumerable:true,configurable:true},d));
  /* shared_native_functions */
  // QuickJS intentionally keeps ECMA-402 optional. Chrome does not, so expose
  // the target profile's locale/time-zone through a small deterministic Intl
  // layer while richer CLDR-backed semantics are added behind the same API.
  let intlEnvironment=host.intlEnvironment();
  if(typeof globalThis.Intl==='undefined'){
    const canonicalLocale=value=>String(value||intlEnvironment.locale).replace(/_/g,'-');
    // ECMA-402 locale availability comes from Chrome's bundled ICU data, not
    // from arbitrary syntactically valid BCP-47 tags. Keep the availability
    // decision separate from locale canonicalization so unknown tags such as
    // zz-ZZ are correctly omitted by supportedLocalesOf().
    const intlLanguageBases=new Set('af am ar as az be bg bn bo br bs ca cs cy da de dsb el en es et eu fa fi fil fo fr ga gd gl gu ha haw he hi hr hsb hu hy id ig is it ja ka kk km kn ko ky lb lo lt lv mk ml mn mr ms mt my nb ne nl nn no or pa pl ps pt ro ru si sk sl sq sr sv sw ta te th tk tr uk ur uz vi wo xh yo zh zu'.split(' '));
    const supportedLocales=locales=>Intl.getCanonicalLocales(locales).filter(locale=>intlLanguageBases.has(locale.split('-')[0].toLowerCase()));
    class Collator { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} compare(a,b){return String(a).localeCompare(String(b))} resolvedOptions(){return{locale:this.__locale,usage:'sort',sensitivity:this.__options.sensitivity||'variant',ignorePunctuation:false,collation:'default',numeric:!!this.__options.numeric,caseFirst:'false'}} static supportedLocalesOf(locales){return supportedLocales(locales)} }
    class NumberFormat { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} format(value){const n=Number(value);return Number.isFinite(n)?String(n):String(n)} formatToParts(value){return[{type:'integer',value:this.format(value)}]} formatRange(a,b){return this.format(a)+'вЂ“'+this.format(b)} formatRangeToParts(a,b){return[{type:'shared',source:'startRange',value:this.format(a)},{type:'literal',source:'shared',value:'вЂ“'},{type:'shared',source:'endRange',value:this.format(b)}]} resolvedOptions(){return{locale:this.__locale,numberingSystem:'latn',style:this.__options.style||'decimal',minimumIntegerDigits:1,minimumFractionDigits:0,maximumFractionDigits:3,useGrouping:'auto',notation:'standard',signDisplay:'auto',roundingIncrement:1,roundingMode:'halfExpand',roundingPriority:'auto',trailingZeroDisplay:'auto'}} static supportedLocalesOf(locales){return supportedLocales(locales)} }
    class DateTimeFormat { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} format(value=Date.now()){const d=new Date(value);return d.toLocaleString?d.toLocaleString():d.toString()} formatToParts(value=Date.now()){return[{type:'literal',value:this.format(value)}]} formatRange(a,b){return this.format(a)+' вЂ“ '+this.format(b)} formatRangeToParts(a,b){return[{type:'shared',source:'startRange',value:this.format(a)},{type:'literal',source:'shared',value:' вЂ“ '},{type:'shared',source:'endRange',value:this.format(b)}]} resolvedOptions(){const dateFields=this.__options.dateStyle||this.__options.timeStyle||['weekday','era','year','month','day','dayPeriod','hour','minute','second','fractionalSecondDigits','timeZoneName'].some(key=>this.__options[key]!==undefined)?{}:{year:'numeric',month:'2-digit',day:'2-digit'};return{locale:this.__locale,calendar:'gregory',numberingSystem:'latn',timeZone:this.__options.timeZone||intlEnvironment.timeZone,...dateFields,...(this.__options.hour12===undefined?{}:{hourCycle:this.__options.hour12?'h12':'h23',hour12:!!this.__options.hour12})}} static supportedLocalesOf(locales){return supportedLocales(locales)} }
    class PluralRules { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} select(value){return Number(value)===1?'one':'other'} selectRange(){return'other'} resolvedOptions(){return{locale:this.__locale,type:this.__options.type||'cardinal',minimumIntegerDigits:1,minimumFractionDigits:0,maximumFractionDigits:3,pluralCategories:['one','other']}} static supportedLocalesOf(locales){return Intl.getCanonicalLocales(locales)} }
    class RelativeTimeFormat { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} format(value,unit){return String(value)+' '+String(unit)} formatToParts(value,unit){return[{type:'integer',value:String(value),unit:String(unit)}]} resolvedOptions(){return{locale:this.__locale,style:this.__options.style||'long',numeric:this.__options.numeric||'always',numberingSystem:'latn'}} static supportedLocalesOf(locales){return Intl.getCanonicalLocales(locales)} }
    class ListFormat { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} format(values){return Array.from(values,String).join(', ')} formatToParts(values){return Array.from(values,String).map((value,index)=>({type:index?'literal':'element',value}))} resolvedOptions(){return{locale:this.__locale,type:this.__options.type||'conjunction',style:this.__options.style||'long'}} static supportedLocalesOf(locales){return Intl.getCanonicalLocales(locales)} }
    class DisplayNames { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__options=options} of(code){return String(code)} resolvedOptions(){return{locale:this.__locale,style:this.__options.style||'long',type:this.__options.type||'language',fallback:this.__options.fallback||'code',languageDisplay:this.__options.languageDisplay||'dialect'}} static supportedLocalesOf(locales){return Intl.getCanonicalLocales(locales)} }
    class Locale { constructor(tag,options={}){this.__tag=canonicalLocale(tag);this.language=this.__tag.split('-')[0];this.baseName=this.__tag;this.calendar=options.calendar;this.caseFirst=options.caseFirst;this.collation=options.collation;this.hourCycle=options.hourCycle;this.numberingSystem=options.numberingSystem;this.numeric=!!options.numeric;this.region=options.region;this.script=options.script} toString(){return this.__tag} maximize(){return new Locale(this.__tag)} minimize(){return new Locale(this.__tag)} getCalendars(){return['gregory']} getCollations(){return['emoji','eor']} getHourCycles(){return['h12','h23']} getNumberingSystems(){return['latn']} getTimeZones(){return[intlEnvironment.timeZone]} getTextInfo(){return{direction:'ltr'}} getWeekInfo(){return{firstDay:7,weekend:[6,7],minimalDays:1}} }
    class Segmenter { constructor(locales,options={}){this.__locale=canonicalLocale(Array.isArray(locales)?locales[0]:locales);this.__granularity=options.granularity||'grapheme'} segment(input){const source=String(input),values=this.__granularity==='word'?source.match(/\S+|\s+/g)||[]:Array.from(source);return{containing(index){let offset=0;for(const segment of values){const value={segment,index:offset,input:source,isWordLike:/\S/.test(segment)};if(index>=offset&&index<offset+segment.length)return value;offset+=segment.length}},*[Symbol.iterator](){let index=0;for(const segment of values){yield{segment,index,input:source,isWordLike:/\S/.test(segment)};index+=segment.length}}}} resolvedOptions(){return{locale:this.__locale,granularity:this.__granularity}} static supportedLocalesOf(locales){return Intl.getCanonicalLocales(locales)} }
    const callable=(name,Constructor)=>new Proxy(Constructor,{apply(target,_self,args){recordAPIAccess('Intl.'+name,true);return Reflect.construct(target,args)},construct(target,args,newTarget){recordAPIAccess('Intl.'+name,true);return Reflect.construct(target,args,newTarget)}});
    globalThis.Intl={getCanonicalLocales(locales){recordAPIAccess('Intl.getCanonicalLocales',true);if(locales===undefined)return[];return(Array.isArray(locales)?locales:[locales]).map(canonicalLocale)},supportedValuesOf(key){recordAPIAccess('Intl.supportedValuesOf',true);if(String(key)==='timeZone')return[intlEnvironment.timeZone];return[]},Collator:callable('Collator',Collator),NumberFormat:callable('NumberFormat',NumberFormat),DateTimeFormat:callable('DateTimeFormat',DateTimeFormat),PluralRules:callable('PluralRules',PluralRules),RelativeTimeFormat:callable('RelativeTimeFormat',RelativeTimeFormat),ListFormat:callable('ListFormat',ListFormat),DisplayNames:callable('DisplayNames',DisplayNames),Locale:callable('Locale',Locale),Segmenter:callable('Segmenter',Segmenter)};
    for(const [name,value] of Object.entries(globalThis.Intl)){markNative(value,name);if(typeof value==='function'&&value.prototype)for(const member of Reflect.ownKeys(value.prototype)){if(member==='constructor')continue;const descriptor=Object.getOwnPropertyDescriptor(value.prototype,member);markNative(descriptor&&descriptor.value,String(member));markNative(descriptor&&descriptor.get,String(member),'get ')}}
  }else{
    // V8 ships Chrome's ECMA-402 implementation. Keep it intact and project
    // only the canonical browser environment into omitted locale/time-zone
    // arguments; host OS defaults are not browser state and must not leak in.
    for(const name of ['Collator','NumberFormat','DateTimeFormat','PluralRules','RelativeTimeFormat','ListFormat','DisplayNames','Segmenter']){
      const Native=globalThis.Intl[name];
      if(typeof Native!=='function')continue;
      const normalize=args=>{
        args=Array.from(args);
        if(args[0]===undefined)args[0]=intlEnvironment.locale;
        if(name==='DateTimeFormat'){
          const options=args[1]===undefined?{}:{...args[1]};
          if(options.timeZone===undefined)options.timeZone=intlEnvironment.timeZone;
          args[1]=options;
        }
        return args;
      };
      globalThis.Intl[name]=new Proxy(Native,{apply(target,self,args){return Reflect.apply(target,self,normalize(args))},construct(target,args,newTarget){return Reflect.construct(target,normalize(args),newTarget)}});
    }
  }
  /* shared_fetch_primitives */
  const eventSlots=new WeakMap(),messageEventSlots=new WeakMap(),errorEventSlots=new WeakMap();
  class Event { constructor(type,init={}){eventSlots.set(this,{type:String(type),bubbles:!!init.bubbles,cancelable:!!init.cancelable,composed:!!init.composed,defaultPrevented:false,trusted:false});Object.defineProperty(this,'isTrusted',{get:()=>!!eventSlots.get(this).trusted,enumerable:true,configurable:false})} get type(){return eventSlots.get(this).type} get bubbles(){return eventSlots.get(this).bubbles} get cancelable(){return eventSlots.get(this).cancelable} get defaultPrevented(){return eventSlots.get(this).defaultPrevented} preventDefault(){const state=eventSlots.get(this);if(state.cancelable)state.defaultPrevented=true} }
  class MessageEvent extends Event { constructor(type,init={}){super(type,init);messageEventSlots.set(this,{data:init.data,origin:String(init.origin||''),lastEventId:String(init.lastEventId||''),source:init.source||null,ports:init.ports||[]})} get data(){return messageEventSlots.get(this).data} get origin(){return messageEventSlots.get(this).origin} get lastEventId(){return messageEventSlots.get(this).lastEventId} get source(){return messageEventSlots.get(this).source} get ports(){return messageEventSlots.get(this).ports} }
  class ErrorEvent extends Event { constructor(type,init={}){super(type,init);errorEventSlots.set(this,{message:String(init.message||''),filename:String(init.filename||''),lineno:Number(init.lineno||0),colno:Number(init.colno||0),error:init.error})} get message(){return errorEventSlots.get(this).message} get filename(){return errorEventSlots.get(this).filename} get lineno(){return errorEventSlots.get(this).lineno} get colno(){return errorEventSlots.get(this).colno} get error(){return errorEventSlots.get(this).error} }
  const eventListeners=new WeakMap(),listenersFor=value=>{let listeners=eventListeners.get(value);if(!listeners){listeners=new Map();eventListeners.set(value,listeners)}return listeners};
  const eventHandlerListeners=new WeakMap();
  const eventHandlerRecord=(target,type)=>{let map=eventHandlerListeners.get(target);if(!map){map=new Map();eventHandlerListeners.set(target,map)}let record=map.get(type);if(!record){record={value:null,listener:null};map.set(type,record)}return record};
  const setEventHandlerValue=(target,type,value)=>{const record=eventHandlerRecord(target,type);value=value!==null&&(typeof value==='function'||typeof value==='object')?value:null;record.value=value;if(value===null&&record.listener){target.removeEventListener(type,record.listener);record.listener=null}if(value!==null&&!record.listener){record.listener=function(event){if(typeof record.value==='function'&&record.value.call(this,event)===false)event.preventDefault()};target.addEventListener(type,record.listener)}};
  const internalEventHandler=(target,type)=>{if(eventHandlerListeners.get(target)?.has(type))return null;if(elementHandlers.has(target))return elementHandlers.get(target)[type]||null;if(xhrSlots.has(target))return xhrSlots.get(target)['on'+type]||null;if(rtcPeerStates.has(target))return rtcPeerStates.get(target)['on'+type]||null;if(rtcDataStates.has(target))return rtcDataStates.get(target)['on'+type]||null;const name='on'+type;return name in target?target[name]:null};
  let dispatchEventCore=(target,event,trusted)=>{const state=eventSlots.get(event);if(!state)throw new TypeError('parameter 1 is not of type Event');if(trusted)state.trusted=true;for(const listener of (listenersFor(target).get(state.type)||[]).slice())listener.call(target,event);const handler=internalEventHandler(target,state.type);if(typeof handler==='function')handler.call(target,event);return !state.defaultPrevented};
  class EventTarget { constructor(){listenersFor(this)} addEventListener(t,f){const target=this==null?globalThis:this;if(typeof f==='function'){const listeners=listenersFor(target),a=listeners.get(String(t))||[];a.push(f);listeners.set(String(t),a)}} removeEventListener(t,f){const target=this==null?globalThis:this,a=listenersFor(target).get(String(t))||[];const i=a.indexOf(f);if(i>=0)a.splice(i,1)} dispatchEvent(e){return dispatchEventCore(this==null?globalThis:this,e,false)} }
  const dispatchTrusted=(target,event)=>dispatchEventCore(target,event,true);
  const messagePortSlots=new WeakMap();
  class MessagePort extends EventTarget { constructor(token){super();if(token!==hostToken)illegal('MessagePort');messagePortSlots.set(this,{peer:null,closed:false,started:false,onmessage:null,onmessageerror:null})} postMessage(message,transfer=[]){const state=messagePortSlots.get(this),peer=state.peer,peerState=peer&&messagePortSlots.get(peer);if(state.closed||!peerState||peerState.closed)return;host.queuePostedMessage(()=>{if(peerState.closed)return;dispatchTrusted(peer,new MessageEvent('message',{data:message,ports:[]}))})} start(){messagePortSlots.get(this).started=true} close(){messagePortSlots.get(this).closed=true} get onmessage(){return messagePortSlots.get(this).onmessage} set onmessage(value){const state=messagePortSlots.get(this);state.onmessage=typeof value==='function'?value:null;if(state.onmessage)state.started=true} get onmessageerror(){return messagePortSlots.get(this).onmessageerror} set onmessageerror(value){messagePortSlots.get(this).onmessageerror=typeof value==='function'?value:null} }
  class MessageChannel { constructor(){const port1=new MessagePort(hostToken),port2=new MessagePort(hostToken);messagePortSlots.get(port1).peer=port2;messagePortSlots.get(port2).peer=port1;Object.defineProperties(this,{port1:{value:port1,enumerable:true},port2:{value:port2,enumerable:true}})} }
  const broadcastChannels=new Map(),broadcastChannelSlots=new WeakMap();
  class BroadcastChannel extends EventTarget { constructor(name){super();name=String(name);broadcastChannelSlots.set(this,{name,closed:false,onmessage:null,onmessageerror:null});const channels=broadcastChannels.get(name)||new Set();channels.add(this);broadcastChannels.set(name,channels)} get name(){return broadcastChannelSlots.get(this).name} get onmessage(){return broadcastChannelSlots.get(this).onmessage} set onmessage(value){broadcastChannelSlots.get(this).onmessage=typeof value==='function'?value:null} get onmessageerror(){return broadcastChannelSlots.get(this).onmessageerror} set onmessageerror(value){broadcastChannelSlots.get(this).onmessageerror=typeof value==='function'?value:null} postMessage(message){const state=broadcastChannelSlots.get(this);if(state.closed)throw new DOMException('BroadcastChannel is closed.','InvalidStateError');for(const channel of broadcastChannels.get(state.name)||[]){if(channel===this||broadcastChannelSlots.get(channel).closed)continue;host.queuePostedMessage(()=>dispatchTrusted(channel,new MessageEvent('message',{data:message,origin:host.locationPart('origin')})))}} close(){const state=broadcastChannelSlots.get(this);if(state.closed)return;state.closed=true;broadcastChannels.get(state.name)?.delete(this)} }
  const browserMessagePortSlots=new WeakMap(),browserMessagePortWrappers=new Map();
  const wrapBrowserMessagePort=id=>{id=String(id);const existing=browserMessagePortWrappers.get(id);if(existing)return existing;const port=new BrowserMessagePort(hostToken,id);browserMessagePortWrappers.set(id,port);return port};
  const takeMessagePorts=transfer=>Array.from(transfer||[]).filter(value=>browserMessagePortSlots.has(value)).map(port=>{const state=browserMessagePortSlots.get(port);if(state.closed||state.transferred)throw new DOMException('MessagePort at index 0 is already neutered.','DataCloneError');state.transferred=true;browserMessagePortWrappers.delete(state.id);return state.id});
  class BrowserMessagePort extends MessagePort { constructor(token,id){super(hostToken);if(token!==hostToken)illegal('MessagePort');browserMessagePortSlots.set(this,{id:String(id),closed:false,transferred:false,started:false,onmessage:null,onmessageerror:null,pending:[]})} postMessage(message,transfer=[]){const state=browserMessagePortSlots.get(this);if(state.closed||state.transferred)return;host.messagePortPost(state.id,message,takeMessagePorts(transfer))} start(){const state=browserMessagePortSlots.get(this);state.started=true;while(state.pending.length){const event=state.pending.shift();host.queuePostedMessage(()=>dispatchTrusted(this,event))}} close(){const state=browserMessagePortSlots.get(this);if(state.closed)return;state.closed=true;host.messagePortClose(state.id)} get onmessage(){return browserMessagePortSlots.get(this).onmessage} set onmessage(value){const state=browserMessagePortSlots.get(this);state.onmessage=typeof value==='function'?value:null;if(state.onmessage)this.start()} get onmessageerror(){return browserMessagePortSlots.get(this).onmessageerror} set onmessageerror(value){const state=browserMessagePortSlots.get(this);state.onmessageerror=typeof value==='function'?value:null} }
  class BrowserMessageChannel { constructor(){const ids=host.newMessageChannel(),port1=wrapBrowserMessagePort(ids[0]),port2=wrapBrowserMessagePort(ids[1]);Object.defineProperties(this,{port1:{value:port1,enumerable:true},port2:{value:port2,enumerable:true}})} }
  globalThis.__receiveMessagePort=(id,data,portIds=[])=>{const port=browserMessagePortWrappers.get(String(id));if(!port)return;const state=browserMessagePortSlots.get(port),event=new MessageEvent('message',{data,ports:Array.from(portIds,wrapBrowserMessagePort)});if(state.started)dispatchTrusted(port,event);else state.pending.push(event)};
  class Node extends EventTarget { constructor(token){super();if(token!==hostToken)illegal('Node')} get nodeType(){const slot=elementSlot(this);if(slot)return slot.type==='element'?1:slot.type==='text'?3:slot.type==='comment'?8:slot.type==='fragment'?11:0;if(this instanceof Document)return 9;if(fragmentSlots.has(this))return 11;return 0} get nodeName(){const slot=elementSlot(this);if(slot)return slot.type==='element'?slot.tagName:slot.type==='text'?'#text':slot.type==='comment'?'#comment':'';if(this instanceof Document)return'#document';if(fragmentSlots.has(this))return'#document-fragment';return''} get textContent(){const slot=elementSlot(this);return slot?host.textContent(slot.nodeId):null} set textContent(value){const slot=elementSlot(this);if(slot)host.setTextContent(slot.nodeId,value==null?'':String(value))} get parentNode(){const slot=elementSlot(this);return slot?wrap(host.parentNode(slot.nodeId)):null} get firstChild(){const slot=elementSlot(this);if(slot)return wrap(host.firstChild(slot.nodeId));const state=fragmentSlots.get(this);return state&&state.children[0]||null} get childNodes(){const slot=elementSlot(this);if(slot)return nodeList(host.nodeChildren(slot.nodeId));const state=fragmentSlots.get(this);return state?nodeList(state.children.map(child=>elementSlot(child))):nodeList([])} hasChildNodes(){return this.childNodes.length!==0} get isConnected(){if(syntheticParents.has(this))return syntheticParents.get(this).isConnected;if(typeof ShadowRoot==='function'&&this instanceof ShadowRoot)return this.host.isConnected;const slot=elementSlot(this);return slot?host.isConnected(slot.nodeId):this instanceof Document} contains(other){if(other==null)return false;if(other===this)return true;const own=elementSlot(this),child=elementSlot(other);if(own&&(own.type==='text'||own.type==='comment'))return false;if(own&&child&&!syntheticParents.has(other))return host.contains(own.nodeId,child.nodeId);for(let node=other;node;node=node.parentNode)if(node===this)return true;return false} }
  class CharacterData extends Node { constructor(token,data){super(token);elementData.set(this,data)} get nodeName(){const slot=elementSlot(this);return slot&&slot.type==='comment'?'#comment':'#text'} get data(){return elementSlot(this).text||''} set data(value){this.textContent=String(value)} get length(){return this.data.length} }
  class Text extends CharacterData { constructor(token,data){super(token,data)} }
  class Comment extends CharacterData { constructor(token,data){super(token,data)} }
  const domTokenSlots=new WeakMap(),domTokenState=value=>domTokenSlots.get(value),domTokens=value=>{const state=domTokenState(value);return(state.element.getAttribute(state.attribute)||'').trim().split(/\s+/).filter(Boolean)},writeDOMTokens=(value,tokens)=>{const state=domTokenState(value);state.element.setAttribute(state.attribute,[...new Set(tokens)].join(' '))};
  class DOMTokenList { constructor(element,attribute){domTokenSlots.set(this,{element,attribute})} get length(){return domTokens(this).length} get value(){return domTokens(this).join(' ')} set value(v){const state=domTokenState(this);state.element.setAttribute(state.attribute,String(v))} item(i){return domTokens(this)[Number(i)]??null} contains(token){return domTokens(this).includes(String(token))} add(...tokens){writeDOMTokens(this,domTokens(this).concat(tokens.map(String)))} remove(...tokens){const gone=new Set(tokens.map(String));writeDOMTokens(this,domTokens(this).filter(x=>!gone.has(x)))} toggle(token,force){const state=domTokenState(this);return host.toggleToken(elementSlot(state.element).nodeId,state.attribute,String(token),force===true?1:force===false?0:-1)} replace(oldToken,newToken){const a=domTokens(this),i=a.indexOf(String(oldToken));if(i<0)return false;a[i]=String(newToken);writeDOMTokens(this,a);return true} supports(){return false} forEach(cb,thisArg){domTokens(this).forEach((v,i)=>cb.call(thisArg,v,v,this))} entries(){return Array.from(domTokens(this).entries())[Symbol.iterator]()} keys(){return Array.from(domTokens(this).keys())[Symbol.iterator]()} values(){return domTokens(this)[Symbol.iterator]()} toString(){return this.value} [Symbol.iterator](){return this.values()} }
  /* shared_webkit_css */
  const cssName=n=>{n=String(n);if(n.startsWith('--'))return n;n=n.toLowerCase();return webkitCSSAliases.get(n)||n};
  const cssJSName=n=>webkitJSNames.get(n)||(n==='cssFloat'?'float':n.startsWith('--')?n:n.replace(/[A-Z]/g,m=>'-'+m.toLowerCase()));
  const cssJSInputName=name=>webkitCSSAliases.has(name)?name:/^[wW]ebkit[A-Z]/.test(name)?'-webkit'+name.slice(6).replace(/[A-Z]/g,c=>'-'+c.toLowerCase()):cssJSName(name);
  const parseCSS=text=>{const out=[];for(const raw of splitCSSDeclarations(String(text||''))){const i=raw.indexOf(':');if(i<0)continue;const name=cssName(raw.slice(0,i).trim());if(!name)continue;const extracted=cssExtractPriority(raw.slice(i+1).trim());if(!extracted)continue;let {value,priority}=extracted;value=normalizeCSSValue(name,value,raw.slice(0,i).trim());if(value===null||value==='')continue;for(const entry of expandCSSDeclaration({name,value,priority})){const old=out.findIndex(x=>x.name===entry.name);if(old>=0){if(out[old].priority==='important'&&priority!=='important')continue;out[old]=entry}else out.push(entry)}}return out};
  const serializeCSS=serializeCSSDeclarations;
  const inlineDisplayTags=new Set(['A','ABBR','B','CODE','EM','I','IFRAME','IMG','INPUT','LABEL','SMALL','SPAN','STRONG']);
  const simpleCSSMatch=(element,selector)=>{if(selector.includes(':root')){if(element.tagName!=='HTML')return false;selector=selector.replace(/:root/g,'')}if(selector.includes(':'))return false;const tag=/^[a-zA-Z][\w-]*|^\*/.exec(selector)?.[0];if(tag&&tag!=='*'&&element.tagName!==tag.toUpperCase())return false;for(const id of selector.matchAll(/#([\w-]+)/g))if(element.id!==id[1])return false;for(const cls of selector.matchAll(/\.([\w-]+)/g))if(!element.classList.contains(cls[1]))return false;for(const attr of selector.matchAll(/\[([\w-]+)(?:\s*=\s*(["']?)([^\]"']*)\2)?\]/g)){const actual=element.getAttribute(attr[1]);if(actual===null||(attr[3]!==undefined&&actual!==attr[3]))return false}return true};
  // Stylesheet selectors are a forgiving input boundary: invalid or unsupported
  // rules do not abort computed style. DOM selector APIs retain their SyntaxError.
  const cssSelectorMatch=(element,selector)=>{try{return compatibilitySelectors.matches(element,selector)}catch(error){if(error&&error.name==='SyntaxError')return false;throw error}};
  const containingShadowRoot=element=>{for(let node=element;node;node=node.parentNode)if(typeof ShadowRoot==='function'&&node instanceof ShadowRoot)return node;return null};
  const styleSheetRules=element=>{const rules=[];let order=0,root=containingShadowRoot(element);const sources=constructedStyleSheets.sources(root||element.ownerDocument||document);for(const text of sources){const source=String(text).replace(/\/\*[\s\S]*?\*\//g,'');const pattern=/([^{}]+)\{([^{}]*)\}/g;let match;while((match=pattern.exec(source))){const selectors=match[1].split(',').map(value=>value.trim()).filter(value=>value&&!value.startsWith('@')&&!value.includes('%'));const declarations=parseCSS(match[2]);for(const selector of selectors){const specificity=[...(selector.match(/#[\w-]+/g)||[])].length*100+[...(selector.match(/\.[\w-]+|\[[^\]]+\]|::?[\w-]+/g)||[])].length*10+[...(selector.match(/(^|[\s>+~])[a-zA-Z][\w-]*/g)||[])].length;rules.push({selector,declarations,specificity,order:order++})}}}return rules};
  const computedCSSDeclarations=element=>{const winners=new Map();const accept=(entry,specificity,order)=>{const old=winners.get(entry.name),important=entry.priority==='important';if(!old||Number(important)>Number(old.important)||(important===old.important&&(specificity>old.specificity||(specificity===old.specificity&&order>=old.order))))winners.set(entry.name,{entry,specificity,order,important})};for(const rule of styleSheetRules(element))if(cssSelectorMatch(element,rule.selector))for(const entry of rule.declarations)accept(entry,rule.specificity,rule.order);for(const entry of inlineCSSDeclarations(element))accept(entry,1000,Number.MAX_SAFE_INTEGER);return Array.from(winners.values(),value=>value.entry)};
  const blockifiedDisplay=value=>({inline:'block','inline-block':'block','inline-table':'table','inline-flex':'flex','inline-grid':'grid'}[String(value).toLowerCase()]||value);
  // Resolve fallback geometry only when its value is read. Unrelated style reads
  // and property enumeration must not run layout or invoke author-defined getters.
  const inlineCSSDeclarations=element=>{const value=host.inlineStyleState(elementSlot(element).nodeId);return value[0]==='j'?JSON.parse(value.slice(1)):parseCSS(value.slice(1))};
  const cssResolvedColor=element=>{for(let n=element;n&&elementSlot(n);n=n.parentElement){const value=computedCSSDeclarations(n).find(e=>e.name==='color')?.value;if(!value||['inherit','unset','currentcolor'].includes(value.toLowerCase()))continue;if(value==='initial')break;const rgba=cssColorRGBA(value);if(rgba)return cssSerializeColor(rgba);host.semanticMissingAt('surface.js/cssResolvedColor','CSS.colorResolution',JSON.stringify({reason:'unsupported computed color',value}));return value}return 'rgb(0, 0, 0)'};
  const cssSlots=new WeakMap(),cssState=value=>cssSlots.get(value),cssEntries=value=>{const state=cssState(value),entries=state.computed?computedCSSDeclarations(state.element):inlineCSSDeclarations(state.element);if(state.computed){const specified=entries.map(entry=>({...entry}));let colorEntry=entries.find(e=>e.name==='color');if(!colorEntry){colorEntry={name:'color',priority:''};entries.push(colorEntry)}Object.defineProperty(colorEntry,'value',{get:()=>cssResolvedColor(state.element),enumerable:true,configurable:true});let fontEntry=entries.find(e=>e.name==='font-size');if(!fontEntry){fontEntry={name:'font-size',priority:''};entries.push(fontEntry)}Object.defineProperty(fontEntry,'value',{get:()=>{const size=cssComputedFontSize(state.element);if(size===null){host.semanticMissingAt('surface.js:104','CSS.computedFontSize');return specified.find(e=>e.name==='font-size')?.value||''}return cssSerializeNumber(size)+'px'},enumerable:true,configurable:true});for(const name of webkitCSSInitial.keys()){let entry=entries.find(e=>e.name===name);if(!entry){entry={name,priority:''};entries.push(entry)}Object.defineProperty(entry,'value',{get:()=>resolveWebkitCSS(state.element,name,specified),enumerable:true,configurable:true})}const position=String(entries.find(x=>x.name==='position')?.value||'static').toLowerCase(),uaDisplay=inlineDisplayTags.has(state.element.tagName)?'inline':'block',display=entries.find(x=>x.name==='display');if(['absolute','fixed'].includes(position)){if(display)display.value=blockifiedDisplay(display.value);else entries.push({name:'display',value:blockifiedDisplay(uaDisplay),priority:''})}else if(!display)entries.push({name:'display',value:uaDisplay,priority:''});const defaults=[['position','static'],['visibility','visible'],['content-visibility','visible'],['opacity','1'],['transform','none'],['box-sizing','content-box'],['min-height','0px'],['padding-top','0px'],['padding-bottom','0px'],['width',()=>layoutRectFor(state.element).width+'px'],['height',()=>layoutRectFor(state.element).height+'px']];for(const [name,entryValue] of defaults)if(!entries.some(x=>x.name===name))entries.push({name,get value(){return typeof entryValue==='function'?entryValue():entryValue},priority:''})}return entries},writeCSSEntries=(value,entries)=>{const state=cssState(value);if(state.computed)throw new DOMException('These styles are computed, and therefore the CSSStyleDeclaration is read-only.','NoModificationAllowedError');const text=serializeCSS(entries),old=host.setInlineStyle(elementSlot(state.element).nodeId,text,JSON.stringify(entries));elementSlot(state.element).attributes.style=text;compatibilityElementState.inlineStyleChanged(state.element,old)};
  class CSSStyleDeclaration { constructor(token,element,computed=false){if(token!==hostToken)illegal('CSSStyleDeclaration');cssSlots.set(this,{element,computed})} get length(){return cssEntries(this).length} get cssText(){return cssState(this).computed?'':serializeCSS(cssEntries(this))} set cssText(v){writeCSSEntries(this,parseCSS(String(v)))} item(i){return cssEntries(this)[Number(i)]?.name||''} getPropertyValue(name){return readCSSDeclaration(cssEntries(this),cssName(name))} getPropertyPriority(name){name=cssName(name);const entries=cssEntries(this),components=cssShorthandComponents[name];if(components){const selected=components.map(n=>entries.find(e=>e.name===n));return selected.every(e=>e?.priority==='important')?'important':''}return entries.find(e=>e.name===name)?.priority||''} setProperty(name,value,priority=''){const inputName=name;name=cssName(name);if(/^webkit/i.test(name))return;const normalized=normalizeCSSValue(name,value,inputName);if(normalized===null)return;value=normalized;priority=String(priority).toLowerCase();if(!name||(priority&&priority!=='important'))return;const components=cssShorthandComponents[name]||[name],entries=cssEntries(this);if(String(value)===''){for(let i=entries.length-1;i>=0;i--)if(components.includes(entries[i].name)||entries[i].name===name)entries.splice(i,1)}else for(const entry of expandCSSDeclaration({name,value:String(value),priority})){const index=entries.findIndex(e=>e.name===entry.name);if(index<0)entries.push(entry);else entries[index]=entry}writeCSSEntries(this,entries)} removeProperty(name){const shorthand=webkitCSSLegacyBreakShorthands.has(String(name).toLowerCase());name=cssName(name);const entries=cssEntries(this),old=readCSSDeclaration(entries,name),components=cssShorthandComponents[name]||[name];writeCSSEntries(this,entries.filter(x=>!components.includes(x.name)&&x.name!==name));return shorthand||cssShorthandComponents[name]?'':old} }
  const cssDeclaration=(element,computed=false)=>{const target=new CSSStyleDeclaration(hostToken,element,computed),proxy=new Proxy(target,{get(t,p,r){if(typeof p==='string'){if(/^\d+$/.test(p))return t.item(Number(p));if(!Reflect.has(t,p)){if(/^-?webkit/i.test(p)&&!webkitJSNames.has(p))return undefined;return t.getPropertyValue(cssJSName(p))}}return Reflect.get(t,p,r)},set(t,p,v,r){if(typeof p==='string'&&!Reflect.has(t,p)){if(/^-?webkit/i.test(p)&&!webkitJSNames.has(p))return Reflect.set(t,p,v,r);t.setProperty(cssJSInputName(p),String(v));return true}return Reflect.set(t,p,v,r)},has(t,p){return typeof p==='string'&&/^(0|[1-9][0-9]*)$/.test(p)&&Number(p)<t.length||webkitJSNames.has(p)||Reflect.has(t,p)},ownKeys(t){return Array.from(new Set([...Array.from({length:t.length},(_,i)=>String(i)),...Reflect.ownKeys(t),...Object.keys(webkitCSSNames)]))},getOwnPropertyDescriptor(t,p){if(typeof p==='string'&&/^(0|[1-9][0-9]*)$/.test(p)&&Number(p)<t.length)return {value:t.item(Number(p)),writable:false,enumerable:true,configurable:true};if(webkitJSNames.has(p))return {value:t.getPropertyValue(cssJSName(p)),writable:true,enumerable:true,configurable:true};return Reflect.getOwnPropertyDescriptor(t,p)}});cssSlots.set(proxy,cssSlots.get(target));return proxy};
  const elementData=new WeakMap(),elementClassLists=new WeakMap(),elementShadows=new WeakMap(),syntheticParents=new WeakMap(),elementSlot=value=>elementData.get(value);
  // WebIDL Node branding is independent of the caller realm's instanceof.
  const nonHostNodeBrands=new WeakSet();
  const isDOMNode=value=>elementData.has(value)||fragmentSlots.has(value)||nonHostNodeBrands.has(value)||value===document;
  const isDOMFragment=value=>elementSlot(value)?.type==='fragment'||fragmentSlots.has(value);
  const fragmentSlots=new WeakMap(),shadowSlots=new WeakMap(),fragmentState=value=>fragmentSlots.get(value);
  class DocumentFragment extends Node {
    constructor(token){super(hostToken);if(new.target===ShadowRoot){fragmentSlots.set(this,{children:[],html:''})}else{const data=host.createDocumentFragment();elementData.set(this,data);elementWrappers.set(String(data.nodeId),this)}}
    get nodeName(){return'#document-fragment'}
    get textContent(){const state=fragmentState(this);return state.children.length?state.children.filter(child=>child.nodeType!==8).map(child=>child.textContent||'').join(''):state.html.replace(/<[^>]*>/g,'')}
    set textContent(value){const state=fragmentState(this);for(const child of state.children)syntheticParents.delete(child);state.children=[];state.html=value==null?'':String(value)}
    get children(){return cachedHTMLCollection(this,'children','',()=>fragmentState(this).children.map(child=>elementSlot(child)).filter(Boolean))}
    get firstElementChild(){return fragmentState(this).children.find(child=>child instanceof Element)||null}
    get lastElementChild(){return fragmentState(this).children.findLast?fragmentState(this).children.findLast(child=>child instanceof Element):fragmentState(this).children.slice().reverse().find(child=>child instanceof Element)||null}
    get childElementCount(){return fragmentState(this).children.filter(child=>child instanceof Element).length}
    appendChild(node){const state=fragmentState(this);if(!isDOMNode(node))throw new TypeError("Failed to execute 'appendChild' on 'Node': parameter 1 is not of type 'Node'.");const old=syntheticParents.get(node);if(old&&fragmentSlots.has(old))old.removeChild(node);state.children.push(node);state.html='';syntheticParents.set(node,this);return node}
    removeChild(node){const state=fragmentState(this),index=state.children.indexOf(node);if(index<0)throw new DOMException("The node to be removed is not a child of this node.",'NotFoundError');state.children.splice(index,1);syntheticParents.delete(node);return node}
    append(...nodes){for(const node of nodes)this.appendChild(isDOMNode(node)?node:document.createElement('span'))}
    prepend(...nodes){for(let i=nodes.length-1;i>=0;i--){const node=isDOMNode(nodes[i])?nodes[i]:document.createElement('span');fragmentState(this).children.unshift(node);syntheticParents.set(node,this)}}
    replaceChildren(...nodes){const state=fragmentState(this);for(const child of state.children)syntheticParents.delete(child);state.children=[];state.html='';this.append(...nodes)}
    querySelector(selector){return this.querySelectorAll(selector)[0]||null}
    querySelectorAll(selector){const query=String(selector).trim(),result=[];const matches=node=>node instanceof Element&&(query.startsWith('#')?node.id===query.slice(1):query.startsWith('.')?node.classList.contains(query.slice(1)):node.localName===query.toLowerCase());const visit=node=>{if(matches(node))result.push(node);if(node instanceof Element)for(const child of Array.from(node.children))visit(child)};for(const child of fragmentState(this).children)visit(child);return new Proxy(Object.create(NodeList.prototype),{get(target,key){if(key==='length')return result.length;if(key==='item')return index=>result[Number(index)]||null;if(key===Symbol.iterator)return result[Symbol.iterator].bind(result);if(typeof key==='string'&&/^\d+$/.test(key))return result[Number(key)];return Reflect.get(target,key)}})}
    getElementById(id){return this.querySelector('#'+String(id))}
  }
  class ShadowRoot extends DocumentFragment {
    constructor(token,hostElement,mode,init){if(token!==hostToken)illegal('ShadowRoot');super(token);shadowSlots.set(this,{host:hostElement,mode,delegatesFocus:!!init.delegatesFocus,slotAssignment:String(init.slotAssignment||'named'),serializable:!!init.serializable,clonable:!!init.clonable,onslotchange:null})}
    get mode(){return shadowSlots.get(this).mode} get host(){return shadowSlots.get(this).host} get delegatesFocus(){return shadowSlots.get(this).delegatesFocus} get slotAssignment(){return shadowSlots.get(this).slotAssignment} get serializable(){return shadowSlots.get(this).serializable} get clonable(){return shadowSlots.get(this).clonable}
    get onslotchange(){return shadowSlots.get(this).onslotchange} set onslotchange(value){shadowSlots.get(this).onslotchange=typeof value==='function'?value:null}
    get innerHTML(){return fragmentState(this).html} set innerHTML(value){const state=fragmentState(this);for(const child of state.children)syntheticParents.delete(child);state.children=[];state.html=value==null?'':String(value)}
    getHTML(){return this.innerHTML} setHTMLUnsafe(value){this.innerHTML=value}
  }
  const fireFor=n=>type=>dispatchTrusted(n,new Event(type));
  const insertHostNode=(parent,child,before,node)=>{if(!['SCRIPT','IMG','LINK','IFRAME'].includes(child.tagName)){return host.insertPlain(parent,child.nodeId,before?before.nodeId:0)}const fire=fireFor(node);if(before)host.insert(parent,child,before,()=>fire('load'),()=>fire('error'));else host.append(parent,child,()=>fire('load'),()=>fire('error'));return true};
  class Element extends Node { constructor(token,data){super(token);elementData.set(this,data)} get tagName(){return elementSlot(this).tagName} get nodeName(){return this.tagName} get localName(){return this.tagName.toLowerCase()} get namespaceURI(){return elementSlot(this).namespaceURI||null} get id(){return this.getAttribute('id')||''} set id(v){this.setAttribute('id',String(v))} get className(){return this.getAttribute('class')||''} set className(v){this.setAttribute('class',String(v))} get role(){return this.getAttribute('role')} set role(v){if(v==null)this.removeAttribute('role');else this.setAttribute('role',String(v))} get classList(){let list=elementClassLists.get(this);if(!list){list=new DOMTokenList(this,'class');elementClassLists.set(this,list)}return list} get shadowRoot(){const root=elementShadows.get(this);return root&&root.mode==='open'?root:null} attachShadow(init){if(!init||!['open','closed'].includes(String(init.mode)))throw new TypeError("Failed to execute 'attachShadow' on 'Element': Failed to read the 'mode' property from 'ShadowRootInit'");if(elementShadows.has(this))throw new DOMException('Shadow root cannot be created on a host which already hosts a shadow tree.','NotSupportedError');const root=new ShadowRoot(hostToken,this,String(init.mode),init);elementShadows.set(this,root);return root} get textContent(){return host.textContent(elementSlot(this).nodeId)} set textContent(v){host.setTextContent(elementSlot(this).nodeId,v==null?'':String(v))} get innerHTML(){return host.innerHTML(elementSlot(this).nodeId)} set innerHTML(v){host.setInnerHTML(elementSlot(this).nodeId,String(v))} get outerHTML(){return host.outerHTML(elementSlot(this).nodeId)} set outerHTML(v){const name=host.setOuterHTML(elementSlot(this).nodeId,v==null?'':String(v));if(name)throw new DOMException('Cannot replace this element.',name)} insertAdjacentHTML(position,text){const id=elementSlot(this).nodeId;if(arguments.length<2)throw new TypeError('Not enough arguments');position=String(position).toLowerCase();text=String(text);const name=host.insertAdjacentHTML(id,position,text);if(name)throw new DOMException('Cannot insert adjacent HTML.',name)} get parentNode(){return syntheticParents.get(this)||wrap(host.parentNode(elementSlot(this).nodeId))} get parentElement(){const parent=this.parentNode;return parent instanceof Element?parent:null} get firstElementChild(){return wrap(host.firstElementChild(elementSlot(this).nodeId))} get nextSibling(){return wrap(host.sibling(elementSlot(this).nodeId,1))} get previousSibling(){return wrap(host.sibling(elementSlot(this).nodeId,-1))} get children(){return cachedHTMLCollection(this,'children','',()=>host.elementChildren(elementSlot(this).nodeId))} get childElementCount(){return this.children.length} getAttribute(n){return host.getAttribute(elementSlot(this).nodeId,String(n))} hasAttribute(n){return this.getAttribute(String(n))!==null} hasAttributes(){return Object.keys(host.nodeData(elementSlot(this).nodeId).attributes||{}).length!==0} getAttributeNames(){return host.nodeData(elementSlot(this).nodeId).attributeNames||[]} setAttribute(n,v){n=this.namespaceURI==='http://www.w3.org/1999/xhtml'?String(n).toLowerCase():String(n);v=String(v);elementSlot(this).attributes[n]=v;host.setAttribute(elementSlot(this).nodeId,n,v)} removeAttribute(n){n=this.namespaceURI==='http://www.w3.org/1999/xhtml'?String(n).toLowerCase():String(n);delete elementSlot(this).attributes[n];host.removeAttribute(elementSlot(this).nodeId,n)} toggleAttribute(n,force){n=String(n);const present=this.hasAttribute(n);if(force===true||(!present&&force!==false)){this.setAttribute(n,'');return true}if(present)this.removeAttribute(n);return false} querySelector(s){return wrap(host.queryWithin(elementSlot(this).nodeId,String(s)))} querySelectorAll(s){return nodeList(host.queryAllWithin(elementSlot(this).nodeId,String(s)))} appendChild(n){insertHostNode(elementSlot(this).nodeId,elementSlot(n),null,n);return n} insertBefore(n,before){insertHostNode(elementSlot(this).nodeId,elementSlot(n),before?elementSlot(before):null,n);return n} removeChild(n){host.removeNode(elementSlot(this).nodeId,elementSlot(n).nodeId);return n} remove(){const p=this.parentNode;if(p)p.removeChild(this)} }
  Object.defineProperty(Element.prototype,'previousElementSibling',{get:function(){for(let node=this.previousSibling;node;node=node.previousSibling)if(node instanceof Element)return node;return null},enumerable:true,configurable:true});
  const namedAttributes=element=>attributeCompatibility.namedMap(element);
  Object.defineProperty(Element.prototype,'attributes',{get:function(){return namedAttributes(this)},enumerable:true,configurable:true});
  for(const name of ['appendChild','insertBefore','removeChild','parentNode','parentElement'])delete Element.prototype[name];
  for(const name of ['appendChild','removeChild'])delete DocumentFragment.prototype[name];
  def(Node.prototype,'parentNode',{get(){if(syntheticParents.has(this))return syntheticParents.get(this);const slot=elementSlot(this);return slot?wrap(host.parentNode(slot.nodeId)):null}});
  def(Node.prototype,'parentElement',{get(){const parent=this.parentNode;return parent instanceof Element?parent:null}});
  const runSyntheticInsertionSteps=node=>{if(node instanceof HTMLIFrameElement&&node.isConnected)host.iframeWindow(elementSlot(node).nodeId,true);if(!(node instanceof Element))return;for(const child of Array.from(node.children))runSyntheticInsertionSteps(child);const shadow=elementShadows.get(node),state=shadow&&fragmentState(shadow);if(state)for(const child of state.children)runSyntheticInsertionSteps(child)};
  def(Node.prototype,'appendChild',{value:function(node){if(fragmentSlots.has(this)){if(!isDOMNode(node))throw new TypeError("Failed to execute 'appendChild' on 'Node': parameter 1 is not of type 'Node'.");const state=fragmentState(this),old=syntheticParents.get(node);if(old&&fragmentSlots.has(old)){const oldState=fragmentState(old),index=oldState.children.indexOf(node);if(index>=0)oldState.children.splice(index,1)}state.children.push(node);state.html='';syntheticParents.set(node,this);if(this instanceof ShadowRoot)runSyntheticInsertionSteps(node);return node}const target=elementSlot(this),child=elementSlot(node);if(!target||!child)throw new DOMException('The operation is not supported for this node.','HierarchyRequestError');if(insertHostNode(target.nodeId,child,null,node))runSyntheticInsertionSteps(node);return node},writable:true});
  def(Node.prototype,'insertBefore',{value:function(node,before){if(fragmentSlots.has(this)){const state=fragmentState(this),index=before==null?state.children.length:state.children.indexOf(before);if(index<0)throw new DOMException("The node before which the new node is to be inserted is not a child of this node.",'NotFoundError');state.children.splice(index,0,node);syntheticParents.set(node,this);if(this instanceof ShadowRoot)runSyntheticInsertionSteps(node);return node}const target=elementSlot(this),child=elementSlot(node);if(!target||!child)throw new DOMException('The operation is not supported for this node.','HierarchyRequestError');if(insertHostNode(target.nodeId,child,before?elementSlot(before):null,node))runSyntheticInsertionSteps(node);return node},writable:true});
  def(Node.prototype,'removeChild',{value:function(node){if(fragmentSlots.has(this)){const state=fragmentState(this),index=state.children.indexOf(node);if(index<0)throw new DOMException("The node to be removed is not a child of this node.",'NotFoundError');state.children.splice(index,1);syntheticParents.delete(node);return node}const target=elementSlot(this),child=elementSlot(node);if(!target||!child)throw new DOMException('The operation is not supported for this node.','NotFoundError');if(host.prepareNodeRemoval(child.nodeId)){dispatchTrusted(window,new Event('load'));host.completeSynchronousLoad()}host.removeNode(target.nodeId,child.nodeId);return node},writable:true});
  def(Element.prototype,'matches',{value:function(selectors){return String(selectors).split(',').some(selector=>cssSelectorMatch(this,selector))},writable:true});
  def(Element.prototype,'webkitMatchesSelector',{value:Element.prototype.matches,writable:true});
  def(Element.prototype,'closest',{value:function(selectors){for(let element=this;element instanceof Element;element=element.parentElement)if(element.matches(selectors))return element;return null},writable:true});
  def(Node.prototype,'ownerDocument',{get:function(){return this instanceof Document?null:document},enumerable:true});
  // A fragment contributes its children, in order, and is empty afterwards.
  // Keep canonical host nodes; only detach the synthetic fragment parent link.
  const appendNode=Node.prototype.appendChild,insertNode=Node.prototype.insertBefore;
  let validateTemplateInsertion=null;
  const prepareInsertion=(parent,node,before)=>{
    if(!isDOMNode(node))throw new TypeError('Expected a Node');
    if(!['element','document'].includes(elementSlot(parent)?.type)&&!isDOMFragment(parent)&&parent!==document)throw new DOMException('Unsupported parent node.','HierarchyRequestError');
    if(before!=null&&before.parentNode!==parent)throw new DOMException('Reference node is not a child.','NotFoundError');
    if(node===parent||node.contains(parent))throw new DOMException('Insertion would create a cycle.','HierarchyRequestError');
    if(validateTemplateInsertion)validateTemplateInsertion(parent,node);
  };
  const detachForInsertion=(parent,node)=>{
    const old=syntheticParents.get(node)||(fragmentSlots.has(parent)?node.parentNode:null);
    if(old)old.removeChild(node);
  };
  // Drain fragment membership once. Repeated removal from the front shifts
  // the remaining array for every child, making one insertion quadratic.
  const takeFragmentChildren=node=>{
    const slot=elementSlot(node);if(slot&&slot.type==='fragment'){const children=Array.from(node.childNodes);for(const child of children)node.removeChild(child);return children}
    const state=fragmentState(node),children=state.children;
    state.children=[];state.html='';
    for(const child of children)syntheticParents.delete(child);
    return children;
  };
  def(Node.prototype,'appendChild',{value:function(node){
    prepareInsertion(this,node,null);
    if(isDOMFragment(node)){for(const child of takeFragmentChildren(node))this.appendChild(child);return node}
    detachForInsertion(this,node);return appendNode.call(this,node);
  },writable:true});
  def(Node.prototype,'insertBefore',{value:function(node,before){
    prepareInsertion(this,node,before);
    if(node===before)return node;
    if(isDOMFragment(node)){for(const child of takeFragmentChildren(node))this.insertBefore(child,before);return node}
    detachForInsertion(this,node);return insertNode.call(this,node,before);
  },writable:true});
  const styleCache=new WeakMap();
  const domRectSlots=new WeakMap();
  class DOMRectReadOnly { constructor(x=0,y=0,width=0,height=0){const left=Number(x),top=Number(y),w=Number(width),h=Number(height);domRectSlots.set(this,{x:left,y:top,width:w,height:h,left:Math.min(left,left+w),right:Math.max(left,left+w),top:Math.min(top,top+h),bottom:Math.max(top,top+h)})} get x(){return domRectSlots.get(this).x} get y(){return domRectSlots.get(this).y} get width(){return domRectSlots.get(this).width} get height(){return domRectSlots.get(this).height} get top(){return domRectSlots.get(this).top} get right(){return domRectSlots.get(this).right} get bottom(){return domRectSlots.get(this).bottom} get left(){return domRectSlots.get(this).left} toJSON(){return{...domRectSlots.get(this)}} static fromRect(other={}){return new DOMRectReadOnly(other.x||0,other.y||0,other.width||0,other.height||0)} }
  class DOMRect extends DOMRectReadOnly { constructor(x=0,y=0,width=0,height=0){super(x,y,width,height)} get x(){return domRectSlots.get(this).x} set x(value){const state=domRectSlots.get(this);state.x=Number(value);state.left=Math.min(state.x,state.x+state.width);state.right=Math.max(state.x,state.x+state.width)} get y(){return domRectSlots.get(this).y} set y(value){const state=domRectSlots.get(this);state.y=Number(value);state.top=Math.min(state.y,state.y+state.height);state.bottom=Math.max(state.y,state.y+state.height)} get width(){return domRectSlots.get(this).width} set width(value){const state=domRectSlots.get(this);state.width=Number(value);state.left=Math.min(state.x,state.x+state.width);state.right=Math.max(state.x,state.x+state.width)} get height(){return domRectSlots.get(this).height} set height(value){const state=domRectSlots.get(this);state.height=Number(value);state.top=Math.min(state.y,state.y+state.height);state.bottom=Math.max(state.y,state.y+state.height)} static fromRect(other={}){return new DOMRect(other.x||0,other.y||0,other.width||0,other.height||0)} }
  /* shared_dom_matrix */
  const layoutPositionFor=element=>String(computedCSSDeclarations(element).find(entry=>entry.name==='position')?.value||'static').toLowerCase(),participatesInFlow=element=>!['absolute','fixed'].includes(layoutPositionFor(element));
  const layoutRectFor=element=>{const value=host.rect(elementSlot(element).nodeId);if(value.height===0){for(const child of Array.from(element.children||[])){if(!participatesInFlow(child))continue;const childRect=layoutRectFor(child);value.height=Math.max(value.height,childRect.height)}const shadow=elementShadows.get(element),state=shadow&&fragmentState(shadow);if(state){for(const child of state.children){if(!(child instanceof Element)||!participatesInFlow(child))continue;const childRect=layoutRectFor(child);value.height=Math.max(value.height,childRect.height)}}value.bottom=value.top+value.height}return value},makeDOMRect=(value,element)=>{const resolved=element?layoutRectFor(element):value;return new DOMRect(resolved.x,resolved.y,resolved.width,resolved.height)};
  let constructCustomElement=null,customElementCloneInert=0;
  let templateTreeIsInert=()=>false;
  class HTMLElement extends Element { constructor(token,data){if(token===hostToken){super(token,data);return}if(!constructCustomElement)illegal('HTMLElement');return constructCustomElement(new.target)} get nonce(){return this.getAttribute('nonce')||''} set nonce(v){this.setAttribute('nonce',String(v))} get title(){return this.getAttribute('title')||''} set title(v){this.setAttribute('title',String(v))} get innerText(){return this.textContent} set innerText(v){this.textContent=v==null?'':String(v)} get ariaLive(){return this.getAttribute('aria-live')} set ariaLive(v){if(v==null)this.removeAttribute('aria-live');else this.setAttribute('aria-live',String(v))} get ariaAtomic(){return this.getAttribute('aria-atomic')} set ariaAtomic(v){if(v==null)this.removeAttribute('aria-atomic');else this.setAttribute('aria-atomic',String(v))} get style(){let style=styleCache.get(this);if(!style){style=cssDeclaration(this);styleCache.set(this,style)}return style} get offsetWidth(){return layoutRectFor(this).width} get offsetHeight(){return layoutRectFor(this).height} get clientWidth(){return this.offsetWidth} get clientHeight(){return this.offsetHeight} getBoundingClientRect(){return makeDOMRect(null,this)} }
  const datasetCache=new WeakMap(),datasetName=name=>String(name).replace(/[A-Z]/g,c=>'-'+c.toLowerCase()),datasetKey=name=>String(name).slice(5).replace(/-([a-z])/g,(_m,c)=>c.toUpperCase());Object.defineProperty(HTMLElement.prototype,'dataset',{get:function(){let value=datasetCache.get(this);if(!value){value=new Proxy({}, {get:(_target,key)=>typeof key==='string'?this.getAttribute('data-'+datasetName(key))??undefined:undefined,set:(_target,key,next)=>{this.setAttribute('data-'+datasetName(key),String(next));return true},deleteProperty:(_target,key)=>{this.removeAttribute('data-'+datasetName(key));return true},ownKeys:()=>this.getAttributeNames().filter(name=>name.startsWith('data-')).map(datasetKey),getOwnPropertyDescriptor:(_target,key)=>this.hasAttribute('data-'+datasetName(key))?{value:this.getAttribute('data-'+datasetName(key)),writable:true,enumerable:true,configurable:true}:undefined});datasetCache.set(this,value)}return value},enumerable:true,configurable:true});
  class SVGElement extends Element { get style(){let style=styleCache.get(this);if(!style){style=cssDeclaration(this);styleCache.set(this,style)}return style} }
  const svgRectSlots=new WeakMap();
  class SVGRect { constructor(token){if(token!==hostToken)illegal('SVGRect');svgRectSlots.set(this,{x:0,y:0,width:0,height:0})} get x(){return svgRectSlots.get(this).x} set x(value){svgRectSlots.get(this).x=Number(value)} get y(){return svgRectSlots.get(this).y} set y(value){svgRectSlots.get(this).y=Number(value)} get width(){return svgRectSlots.get(this).width} set width(value){svgRectSlots.get(this).width=Number(value)} get height(){return svgRectSlots.get(this).height} set height(value){svgRectSlots.get(this).height=Number(value)} }
  class SVGSVGElement extends SVGElement { createSVGRect(){return new SVGRect(hostToken)} }
  const svgElementInterfaces={svg:'SVGSVGElement',g:'SVGGElement',a:'SVGAElement',defs:'SVGDefsElement',symbol:'SVGSymbolElement',switch:'SVGSwitchElement',rect:'SVGRectElement',circle:'SVGCircleElement',ellipse:'SVGEllipseElement',line:'SVGLineElement',polyline:'SVGPolylineElement',polygon:'SVGPolygonElement',path:'SVGPathElement',text:'SVGTextElement',tspan:'SVGTSpanElement',textPath:'SVGTextPathElement',image:'SVGImageElement',use:'SVGUseElement',foreignObject:'SVGForeignObjectElement',clipPath:'SVGClipPathElement',mask:'SVGMaskElement',pattern:'SVGPatternElement',marker:'SVGMarkerElement',linearGradient:'SVGLinearGradientElement',radialGradient:'SVGRadialGradientElement',stop:'SVGStopElement',title:'SVGTitleElement',desc:'SVGDescElement',metadata:'SVGMetadataElement',"style":"SVGStyleElement","filter":"SVGFilterElement","feBlend":"SVGFEBlendElement","feColorMatrix":"SVGFEColorMatrixElement","feComponentTransfer":"SVGFEComponentTransferElement","feComposite":"SVGFECompositeElement","feConvolveMatrix":"SVGFEConvolveMatrixElement","feDiffuseLighting":"SVGFEDiffuseLightingElement","feDisplacementMap":"SVGFEDisplacementMapElement","feDistantLight":"SVGFEDistantLightElement","feDropShadow":"SVGFEDropShadowElement","feFlood":"SVGFEFloodElement","feFuncA":"SVGFEFuncAElement","feFuncB":"SVGFEFuncBElement","feFuncG":"SVGFEFuncGElement","feFuncR":"SVGFEFuncRElement","feGaussianBlur":"SVGFEGaussianBlurElement","feImage":"SVGFEImageElement","feMerge":"SVGFEMergeElement","feMergeNode":"SVGFEMergeNodeElement","feMorphology":"SVGFEMorphologyElement","feOffset":"SVGFEOffsetElement","fePointLight":"SVGFEPointLightElement","feSpecularLighting":"SVGFESpecularLightingElement","feSpotLight":"SVGFESpotLightElement","feTile":"SVGFETileElement","feTurbulence":"SVGFETurbulenceElement","animate":"SVGAnimateElement","animateMotion":"SVGAnimateMotionElement","animateTransform":"SVGAnimateTransformElement","set":"SVGSetElement","mpath":"SVGMPathElement","view":"SVGViewElement","script":"SVGScriptElement"};
  const svgElementPrototypes=new Map();
  const createSVGWrapper=data=>{const prototype=svgElementPrototypes.get((data.qualifiedName||String(data.tagName).toLowerCase()).split(':').at(-1));if(prototype){const value=Object.create(prototype);elementData.set(value,data);return value}return new SVGElement(hostToken,data)};
  const elementHandlers=new WeakMap(),scriptStates=new WeakMap(),handlersFor=value=>{let handlers=elementHandlers.get(value);if(!handlers){handlers={};elementHandlers.set(value,handlers)}return handlers};
  class HTMLScriptElement extends HTMLElement { constructor(token,data){super(token,data);scriptStates.set(this,{forceAsync:true})} get onload(){return handlersFor(this).load||null} set onload(v){handlersFor(this).load=typeof v==='function'?v:null} get onerror(){return handlersFor(this).error||null} set onerror(v){handlersFor(this).error=typeof v==='function'?v:null} get src(){const value=this.getAttribute('src');return value===null?'':host.urlParts(value).href} set src(v){this.setAttribute('src',String(v))} get text(){return this.textContent} set text(v){this.textContent=String(v)} get textContent(){return super.textContent} set textContent(v){super.textContent=v} get async(){return scriptStates.get(this)?.forceAsync||this.getAttribute('async')!==null} set async(v){scriptStates.set(this,{forceAsync:false});if(v)this.setAttribute('async','');else host.removeAttribute(elementSlot(this).nodeId,'async')} get defer(){return this.getAttribute('defer')!==null} set defer(v){if(v)this.setAttribute('defer','');else host.removeAttribute(elementSlot(this).nodeId,'defer')} get crossOrigin(){return this.getAttribute('crossorigin')} set crossOrigin(v){if(v==null)this.removeAttribute('crossorigin');else this.setAttribute('crossorigin',String(v))} get referrerPolicy(){return this.getAttribute('referrerpolicy')||''} set referrerPolicy(v){this.setAttribute('referrerpolicy',String(v))} }
  class HTMLImageElement extends HTMLElement { constructor(token,data){super(token,data)} get onload(){return handlersFor(this).load||null} set onload(v){handlersFor(this).load=typeof v==='function'?v:null} get onerror(){return handlersFor(this).error||null} set onerror(v){handlersFor(this).error=typeof v==='function'?v:null} get src(){return host.urlParts(this.getAttribute('src')||'').href} set src(v){this.setAttribute('src',String(v))} get alt(){return this.getAttribute('alt')||''} set alt(v){this.setAttribute('alt',String(v))} get crossOrigin(){return this.getAttribute('crossorigin')} set crossOrigin(v){if(v==null)this.removeAttribute('crossorigin');else this.setAttribute('crossorigin',String(v))} get referrerPolicy(){return this.getAttribute('referrerpolicy')||''} set referrerPolicy(v){this.setAttribute('referrerpolicy',String(v))} }
  const iframeSandboxCache=new WeakMap();
  class HTMLIFrameElement extends HTMLElement { get allow(){return this.getAttribute('allow')||''} set allow(v){this.setAttribute('allow',String(v))} get referrerPolicy(){const value=(this.getAttribute('referrerpolicy')||'').toLowerCase();return ['no-referrer','no-referrer-when-downgrade','same-origin','origin','strict-origin','origin-when-cross-origin','strict-origin-when-cross-origin','unsafe-url'].includes(value)?value:''} set referrerPolicy(v){this.setAttribute('referrerpolicy',String(v))} get src(){return host.urlParts(this.getAttribute('src')||'').href} set src(v){this.setAttribute('src',String(v))} get srcdoc(){return this.getAttribute('srcdoc')||''} set srcdoc(v){this.setAttribute('srcdoc',String(v))} get width(){return this.getAttribute('width')||''} set width(v){this.setAttribute('width',String(v))} get height(){return this.getAttribute('height')||''} set height(v){this.setAttribute('height',String(v))} get sandbox(){let list=iframeSandboxCache.get(this);if(!list){list=new DOMTokenList(this,'sandbox');iframeSandboxCache.set(this,list)}return list} set sandbox(v){this.sandbox.value=String(v)} get contentWindow(){const id=host.iframeWindow(elementSlot(this).nodeId,this.isConnected);return id==null?null:remoteWindow(id)} get contentDocument(){const id=host.iframeWindow(elementSlot(this).nodeId,this.isConnected);return id==null?null:remoteDocument(id,true)} }
  const anchorRelLists=new WeakMap();
  class HTMLAnchorElement extends HTMLElement { get href(){return host.urlParts(this.getAttribute('href')||'').href} set href(v){this.setAttribute('href',String(v))} get target(){return this.getAttribute('target')||''} set target(v){this.setAttribute('target',String(v))} get rel(){return this.getAttribute('rel')||''} set rel(v){this.setAttribute('rel',String(v))} get relList(){let list=anchorRelLists.get(this);if(!list){list=new DOMTokenList(this,'rel');anchorRelLists.set(this,list)}return list} get origin(){return host.urlParts(this.href).origin} get protocol(){return host.urlParts(this.href).protocol} set protocol(v){const p=host.urlParts(this.href);this.href=String(v)+p.href.slice(p.protocol.length)} get host(){return host.urlParts(this.href).host} get hostname(){return host.urlParts(this.href).hostname} get port(){return host.urlParts(this.href).port} get pathname(){return host.urlParts(this.href).pathname} get search(){return host.urlParts(this.href).search} get hash(){return host.urlParts(this.href).hash} toString(){return this.href} }
  class HTMLLinkElement extends HTMLElement { get onload(){return handlersFor(this).load||null} set onload(value){handlersFor(this).load=typeof value==='function'?value:null} get onerror(){return handlersFor(this).error||null} set onerror(value){handlersFor(this).error=typeof value==='function'?value:null} get href(){const value=this.getAttribute('href');return value===null?'':host.urlParts(value).href} set href(value){this.setAttribute('href',String(value))} get rel(){return this.getAttribute('rel')||''} set rel(value){this.setAttribute('rel',String(value))} get as(){return this.getAttribute('as')||''} set as(value){this.setAttribute('as',String(value))} get crossOrigin(){return this.getAttribute('crossorigin')} set crossOrigin(value){if(value==null)this.removeAttribute('crossorigin');else this.setAttribute('crossorigin',String(value))} }
  class HTMLMetaElement extends HTMLElement { get httpEquiv(){return this.getAttribute('http-equiv')||''} set httpEquiv(value){this.setAttribute('http-equiv',String(value))} get content(){return this.getAttribute('content')||''} set content(value){this.setAttribute('content',String(value))} }
  const htmlElementInterfaces={DIALOG:'HTMLDialogElement',HTML:'HTMLHtmlElement',HEAD:'HTMLHeadElement',BODY:'HTMLBodyElement',DIV:'HTMLDivElement',H1:'HTMLHeadingElement',H2:'HTMLHeadingElement',H3:'HTMLHeadingElement',H4:'HTMLHeadingElement',H5:'HTMLHeadingElement',H6:'HTMLHeadingElement',P:'HTMLParagraphElement',SPAN:'HTMLSpanElement',STYLE:'HTMLStyleElement',LINK:'HTMLLinkElement',META:'HTMLMetaElement',OL:'HTMLOListElement',UL:'HTMLUListElement',LI:'HTMLLIElement',FORM:'HTMLFormElement',INPUT:'HTMLInputElement',BUTTON:'HTMLButtonElement',LABEL:'HTMLLabelElement',TABLE:'HTMLTableElement',THEAD:'HTMLTableSectionElement',TBODY:'HTMLTableSectionElement',TFOOT:'HTMLTableSectionElement',TR:'HTMLTableRowElement',TD:'HTMLTableCellElement',TH:'HTMLTableCellElement',SELECT:'HTMLSelectElement',OPTION:'HTMLOptionElement',TEXTAREA:'HTMLTextAreaElement',CANVAS:'HTMLCanvasElement'};
  const elementWrappers=new Map();
  const wrap=d=>{if(typeof d==='number'){const cached=elementWrappers.get(String(d));if(cached)return cached;d=host.nodeData(d)}if(!d)return null;if(d.type==='document')return wrapDocumentNode(d);const key=String(d.nodeId),cached=elementWrappers.get(key);if(cached){const slot=elementSlot(cached);if(slot)Object.assign(slot,d);return cached}let element;if(d.type==='doctype'){element=Object.create((globalThis.DocumentType||Node).prototype);elementData.set(element,d)}else if(d.type==='fragment'){element=Object.create(DocumentFragment.prototype);elementData.set(element,d)}else if(d.type==='text')element=new Text(hostToken,d);else if(d.type==='comment')element=new Comment(hostToken,d);else if(d.namespaceURI==='http://www.w3.org/2000/svg')element=createSVGWrapper(d);else if(d.tagName==='SCRIPT')element=new HTMLScriptElement(hostToken,d);else if(d.tagName==='IMG')element=new HTMLImageElement(hostToken,d);else if(d.tagName==='IFRAME')element=new HTMLIFrameElement(hostToken,d);else if(d.tagName==='A')element=new HTMLAnchorElement(hostToken,d);else{const name=htmlElementInterfaces[d.tagName],ctor=name&&globalThis[name];if(typeof ctor==='function'&&ctor.prototype){element=Object.create(ctor.prototype);elementData.set(element,d)}else element=new HTMLElement(hostToken,d)}const label=d.type==='element'?'Element<'+String(d.tagName||'').toLowerCase()+'>':d.type==='comment'?'Comment':'Text';const proxy=observe(label,element);elementWrappers.set(key,proxy);return proxy};
  globalThis.__mimicDispatchFrameLoad=nodeId=>{const element=elementWrappers.get(String(nodeId));if(element)dispatchTrusted(element,new Event('load'))};
  globalThis.__mimicDispatchResourceEvent=(nodeId,type)=>{const element=wrap(nodeId);if(element)dispatchTrusted(element,new Event(type))};
  // Cache by owner, query kind and original argument: equivalent selectors can
  // still identify distinct collections in Chrome. Weak owners release on teardown.
  const liveCollectionCache=new WeakMap();
  const cachedHTMLCollection=(owner,kind,key,read)=>{
    let kinds=liveCollectionCache.get(owner);if(!kinds){kinds=new Map();liveCollectionCache.set(owner,kinds)}
    let queries=kinds.get(kind);if(!queries){queries=new Map();kinds.set(kind,queries)}
    if(!queries.has(key))queries.set(key,htmlCollection(read));return queries.get(key);
  };
  const htmlCollection=get=>{
    const indexed=key=>typeof key==='string'&&/^(0|[1-9]\d*)$/.test(key)&&Number(key)<4294967295;
    const named=(values,name)=>{name=String(name);if(!name)return null;for(const value of values){const node=wrap(value);if(node&&(node.getAttribute('id')===name||node.namespaceURI==='http://www.w3.org/1999/xhtml'&&node.getAttribute('name')===name))return node}return null};
    return new Proxy(Object.create(HTMLCollection.prototype),{
      ownKeys(o){
        const values=get(),keys=values.map((_,index)=>String(index));
        for(const value of values){const node=wrap(value);for(const name of [node.getAttribute('id'),node.namespaceURI==='http://www.w3.org/1999/xhtml'?node.getAttribute('name'):null])if(name&&!keys.includes(name))keys.push(name)}
        // A JS Proxy cannot reproduce Blink's duplicate numeric keys when an
        // element ID equals an index: proxy invariants require unique keys.
        // Exact parity for that case requires a native legacy-object interceptor.
        return [...new Set([...keys,...Reflect.ownKeys(o)])];
      },
      getOwnPropertyDescriptor(o,key){
        if(indexed(key)){const values=get();if(Number(key)<values.length)return {value:wrap(values[Number(key)]),writable:false,enumerable:true,configurable:true}}
        const own=Reflect.getOwnPropertyDescriptor(o,key);if(own)return own;
        if(typeof key==='string'&&!Reflect.has(o,key)){const value=named(get(),key);if(value!==null)return {value,writable:false,enumerable:false,configurable:true}}
      },
      has(o,key){if(indexed(key))return Number(key)<get().length;return Reflect.has(o,key)||typeof key==='string'&&named(get(),key)!==null},
      get(o,key){const values=get();if(key==='length')return values.length;if(key==='item')return index=>wrap(get()[Number(index)>>>0]);if(key==='namedItem')return name=>named(get(),name);
        if(key===Symbol.iterator)return function*(){for(let i=0;i<get().length;i++)yield wrap(get()[i])};
        if(indexed(key))return Number(key)<values.length?wrap(values[Number(key)]):undefined;
        if(Reflect.has(o,key))return Reflect.get(o,key);return typeof key==='string'?named(values,key)||undefined:undefined;
      }
    });
  };
  class HTMLCollection { constructor(){illegal('HTMLCollection')} item(){} namedItem(){} }
  const nodeList=data=>{const read=()=>typeof data==='function'?data():data;let proxy;proxy=new Proxy(Object.create(NodeList.prototype),{get(o,p){if(p==='length')return read().length;if(p==='item')return i=>wrap(read()[Number(i)>>>0]);if(p==='forEach')return (cb,thisArg)=>{const length=read().length;for(let i=0;i<length;i++){const rows=read();if(i<rows.length)cb.call(thisArg,wrap(rows[i]),i,proxy)}};if(p===Symbol.iterator||p==='values')return function*(){for(let i=0;i<read().length;i++)yield wrap(read()[i])};if(p==='keys')return function*(){for(let i=0;i<read().length;i++)yield i};if(p==='entries')return function*(){for(let i=0;i<read().length;i++)yield [i,wrap(read()[i])]};if(typeof p==='string'&&/^\d+$/.test(p)){const row=read()[Number(p)];return row===undefined?undefined:wrap(row)}return Reflect.get(o,p)},has(o,p){return typeof p==='string'&&/^(0|[1-9][0-9]*)$/.test(p)&&Number(p)<read().length||Reflect.has(o,p)},ownKeys(o){return [...read().map((_,i)=>String(i)),...Reflect.ownKeys(o)]},getOwnPropertyDescriptor(o,p){if(typeof p==='string'&&/^(0|[1-9][0-9]*)$/.test(p)&&Number(p)<read().length)return {value:wrap(read()[Number(p)]),writable:false,enumerable:true,configurable:true};return Reflect.getOwnPropertyDescriptor(o,p)}});return proxy};
  class NodeList { constructor(){illegal('NodeList')} item(){} forEach(){} }
  // New node shape is already known here; only canonical identity crosses back.
  const freshNodeData=(nodeId,type,tagName,namespaceURI,text)=>({attributes:{},children:[],namespaceURI,nodeId,parentId:0,tagName,text,type});
  const freshCharacterData=(id,type,text)=>wrap(/[\uD800-\uDFFF]/.test(text)?id:freshNodeData(id,type,'','',text));
  class Document extends Node { get currentScript(){return wrap(host.currentScript())} get title(){return host.title()} set title(v){host.setTitle(String(v))} get lang(){return this.documentElement.getAttribute('lang')||''} set lang(v){this.documentElement.setAttribute('lang',String(v))} get dir(){return this.documentElement.getAttribute('dir')||''} set dir(v){this.documentElement.setAttribute('dir',String(v))} get readyState(){return host.readyState()} get visibilityState(){return'visible'} get hidden(){return false} get prerendering(){return false} get wasDiscarded(){return false} get documentElement(){return wrap(host.query('html'))} get head(){return wrap(host.query('head'))} get body(){return wrap(host.query('body'))} get location(){return loc} get URL(){return loc.href} get documentURI(){return loc.href} get cookie(){return host.documentCookie()} set cookie(v){host.setDocumentCookie(String(v))} get featurePolicy(){return documentPolicy} get permissionsPolicy(){return documentPolicy} querySelector(s){return wrap(host.query(String(s)))} querySelectorAll(s){return nodeList(host.queryAll(String(s)))} getElementById(id){return this.querySelector('#'+id)} getElementsByTagName(tag){const name=String(tag);return htmlCollection(()=>host.elementsByTagName(name))} createElement(tag){tag=String(tag);const value=host.create(tag);return wrap(typeof value==='number'?freshNodeData(value,'element',tag.toUpperCase(),'http://www.w3.org/1999/xhtml',''):value)} createElementNS(namespace,qualifiedName){return wrap(host.createNS(namespace==null?'':String(namespace),String(qualifiedName)))} createTextNode(data){data=String(data);return freshCharacterData(host.createText(data),'text',data)} createComment(data){data=String(data);return freshCharacterData(host.createComment(data),'comment',data)} createDocumentFragment(){return new DocumentFragment(hostToken)} }
  // HTML documents have an otherwise empty interface layer above Document.
  // Keeping that layer on the canonical wrapper preserves inherited ownership.
  class HTMLDocument extends Document { constructor(token){if(token!==hostToken)throw new TypeError("Failed to construct 'HTMLDocument': Illegal constructor");super(token)} }
  Object.defineProperty(Document.prototype,'createNodeIterator',{value:function(root,whatToShow=0xffffffff,filter=null){if(!(isDOMNode(root)))throw new TypeError("Failed to execute 'createNodeIterator' on 'Document': parameter 1 is not of type 'Node'.");const nodes=[],visit=node=>{if(node instanceof Element)nodes.push(node);if(node instanceof Document){const element=node.documentElement;if(element)visit(element)}else if(node&&node.children)for(const child of Array.from(node.children))visit(child)};visit(root);const shown=Number(whatToShow)>>>0,accepted=nodes.filter(node=>{if(!(shown&1))return false;if(!filter)return true;const result=typeof filter==='function'?filter(node):filter.acceptNode(node);return Number(result)===1});let index=-1;const iterator=Object.create((globalThis.NodeIterator&&globalThis.NodeIterator.prototype)||Object.prototype);Object.defineProperties(iterator,{root:{value:root,enumerable:true},whatToShow:{value:shown,enumerable:true},filter:{value:filter,enumerable:true},referenceNode:{get:()=>accepted[Math.max(index,0)]||root,enumerable:true},pointerBeforeReferenceNode:{get:()=>index<0,enumerable:true},nextNode:{value:()=>accepted[++index]||null},previousNode:{value:()=>index>0?accepted[--index]:null},detach:{value:()=>{}}});return iterator},writable:true,enumerable:true,configurable:true});
  Object.defineProperties(Document.prototype,{all:{get:function(){return undefined},enumerable:true,configurable:true},scripts:{get:function(){return this.getElementsByTagName('script')},enumerable:true,configurable:true},styleSheets:{get:function(){const owners=[...Array.from(this.querySelectorAll('style')),...Array.from(this.querySelectorAll('link')).filter(node=>String(node.getAttribute('rel')||'').toLowerCase().split(/\s+/).includes('stylesheet')&&node.getAttribute('href')!==null)],sheets=owners.map(ownerNode=>({ownerNode,disabled:false,href:ownerNode.localName==='link'?new URL(ownerNode.getAttribute('href'),this.URL).href:null,media:{length:0,mediaText:ownerNode.getAttribute('media')||''},cssRules:[]}));return new Proxy(Object.create((globalThis.StyleSheetList&&globalThis.StyleSheetList.prototype)||Object.prototype),{get(target,key,receiver){if(key==='length')return sheets.length;if(key==='item')return index=>sheets[Number(index)]||null;if(key===Symbol.iterator)return sheets[Symbol.iterator].bind(sheets);if(typeof key==='string'&&/^\d+$/.test(key))return sheets[Number(key)];return Reflect.get(target,key,receiver)}})},enumerable:true,configurable:true},referrer:{get:()=>'',enumerable:true,configurable:true}});
  const permissionsPolicySlots=new WeakMap();
  class PermissionsPolicy { constructor(token,data){if(token!==hostToken)illegal('PermissionsPolicy');const clauses=new Map();for(const raw of String(data.header||'').split(',')){const clause=raw.trim();if(!clause)continue;const index=clause.indexOf('=');if(index<0)continue;clauses.set(clause.slice(0,index).trim(),clause.slice(index+1).trim())}permissionsPolicySlots.set(this,{clauses,origin:String(data.origin||'null')})} features(){return Array.from(permissionsPolicySlots.get(this).clauses.keys()).sort()} allowsFeature(feature,origin){if(!permissionsPolicySlots.has(this))throw new TypeError('Illegal invocation');const current=host.permissionsPolicy(),hint=current.clientHints[String(feature)];if(hint!==undefined){const target=origin===undefined?current.origin:String(origin);return Array.from(hint||[]).includes(target)||Array.from(hint||[]).includes('*')}const state=permissionsPolicySlots.get(this),value=state.clauses.get(String(feature));if(value===undefined)return true;if(value==='()')return false;if(value==='*')return true;const target=origin===undefined?state.origin:String(origin);return this.getAllowlistForFeature(feature).includes(target)||this.getAllowlistForFeature(feature).includes('*')} allowedFeatures(){return this.features().filter(feature=>this.allowsFeature(feature))} getAllowlistForFeature(feature){if(!permissionsPolicySlots.has(this))throw new TypeError('Illegal invocation');const hint=host.permissionsPolicy().clientHints[String(feature)];if(hint!==undefined)return Array.from(hint||[]);const state=permissionsPolicySlots.get(this),value=state.clauses.get(String(feature));if(value===undefined)return[state.origin];if(value==='()')return[];if(value==='*')return['*'];const body=value.replace(/^\(|\)$/g,'').trim();if(!body)return[];return body.split(/\s+/).map(item=>item.replace(/^['"]|['"]$/g,'')).map(item=>item==='self'?state.origin:item)} }
  const FeaturePolicy=PermissionsPolicy;
  class Navigator { constructor(){illegal('Navigator')} }
  class MimeType { constructor(token,type,suffixes,description){if(token!==hostToken)illegal('MimeType');this.__type=type;this.__suffixes=suffixes;this.__description=description;this.__plugin=null} get type(){return this.__type} get suffixes(){return this.__suffixes} get description(){return this.__description} get enabledPlugin(){return this.__plugin} }
  class Plugin { constructor(token,name){if(token!==hostToken)illegal('Plugin');this.__name=name;this.__mimes=[]} get name(){return this.__name} get filename(){return'internal-pdf-viewer'} get description(){return'Portable Document Format'} get length(){return this.__mimes.length} item(index){return this.__mimes[Number(index)]||null} namedItem(name){return this.__mimes.find(m=>m.type===String(name))||null} [Symbol.iterator](){return this.__mimes[Symbol.iterator]()} }
  class PluginArray { constructor(token,plugins){if(token!==hostToken)illegal('PluginArray');this.__plugins=plugins;plugins.forEach((plugin,index)=>{Object.defineProperty(this,index,{value:plugin,enumerable:true});Object.defineProperty(this,plugin.name,{value:plugin,enumerable:false})})} get length(){return this.__plugins.length} item(index){return this.__plugins[Number(index)]||null} namedItem(name){return this.__plugins.find(p=>p.name===String(name))||null} refresh(){} [Symbol.iterator](){return this.__plugins[Symbol.iterator]()} }
  class MimeTypeArray { constructor(token,mimes){if(token!==hostToken)illegal('MimeTypeArray');this.__mimes=mimes;mimes.forEach((mime,index)=>{Object.defineProperty(this,index,{value:mime,enumerable:true});Object.defineProperty(this,mime.type,{value:mime,enumerable:false})})} get length(){return this.__mimes.length} item(index){return this.__mimes[Number(index)]||null} namedItem(name){return this.__mimes.find(m=>m.type===String(name))||null} [Symbol.iterator](){return this.__mimes[Symbol.iterator]()} }
  for(const ctor of [MimeType,Plugin,PluginArray,MimeTypeArray])Object.defineProperty(ctor.prototype,Symbol.toStringTag,{value:ctor.name,configurable:true});
  const uaDataSlots=new WeakMap();
  class NavigatorUAData { constructor(token,data){if(token!==hostToken)illegal('NavigatorUAData');uaDataSlots.set(this,data)} get brands(){return uaDataSlots.get(this).uaBrands.map(x=>({brand:x.brand,version:x.version}))} get mobile(){return false} get platform(){return'Windows'} getHighEntropyValues(hints=[]){const data=uaDataSlots.get(this),out={brands:this.brands,mobile:this.mobile,platform:this.platform};for(const hint of hints.map(String)){if(hint==='architecture')out.architecture=data.architecture;if(hint==='bitness')out.bitness=data.bitness;if(hint==='model')out.model=data.model;if(hint==='platformVersion')out.platformVersion=data.platformVersion;if(hint==='uaFullVersion')out.uaFullVersion=data.uaFullVersion;if(hint==='fullVersionList')out.fullVersionList=data.uaBrands.map(x=>({brand:x.brand,version:x.fullVersion}))}return Promise.resolve(out)} toJSON(){return{brands:this.brands,mobile:this.mobile,platform:this.platform}} }
  class Screen { constructor(){illegal('Screen')} }
  class Location { constructor(){illegal('Location')} assign(value){const binding=requireRealmBinding(this,'Location');if(!arguments.length)throw new TypeError('Not enough arguments');value=bindingString(value);callRealmBinding(this,binding,'assign',[value])} replace(value){const binding=requireRealmBinding(this,'Location');if(!arguments.length)throw new TypeError('Not enough arguments');value=bindingString(value);callRealmBinding(this,binding,'replace',[value])} reload(){const binding=requireRealmBinding(this,'Location');callRealmBinding(this,binding,'reload',[])} toString(){const binding=requireRealmBinding(this,'Location');return callRealmBinding(this,binding,'get',['href'])} }
  class DOMStringList { constructor(){illegal('DOMStringList')} }
  delete DOMStringList.prototype.constructor;
  Object.defineProperties(DOMStringList.prototype,{
    length:{get(){const binding=requireRealmBinding(this,'DOMStringList');return callRealmBinding(this,binding,'length',[])},enumerable:true,configurable:true},
    contains:{value:{contains(value){const binding=requireRealmBinding(this,'DOMStringList');if(!arguments.length)throw new TypeError('Not enough arguments');value=bindingString(value);return callRealmBinding(this,binding,'contains',[value])}}.contains,writable:true,enumerable:true,configurable:true},
    item:{value:{item(index){const binding=requireRealmBinding(this,'DOMStringList');if(!arguments.length)throw new TypeError('Not enough arguments');index=(+index)>>>0;return callRealmBinding(this,binding,'item',[index])}}.item,writable:true,enumerable:true,configurable:true},
    constructor:{value:DOMStringList,writable:true,configurable:true},
    [Symbol.toStringTag]:{value:'DOMStringList',configurable:true},
    [Symbol.iterator]:{value:Array.prototype.values,writable:true,configurable:true}
  });
  const createDOMStringList=values=>{
    const target=Object.create(DOMStringList.prototype),index=key=>typeof key==='string'&&/^(0|[1-9]\d*)$/.test(key)&&Number(key)<4294967295;
    for(let i=0;i<values.length;i++)Object.defineProperty(target,String(i),{value:values[i],enumerable:true,configurable:true});
    const list=new Proxy(target,{set:(target,key,value,receiver)=>index(key)||Reflect.set(target,key,value,receiver),defineProperty:(target,key,descriptor)=>index(key)||Reflect.defineProperty(target,key,descriptor),deleteProperty:(target,key)=>index(key)?Number(key)>=values.length:Reflect.deleteProperty(target,key),preventExtensions:()=>false});
    registerRealmBinding(list,'DOMStringList',{length:()=>values.length,item:index=>values[index]??null,contains:value=>values.includes(value)},true);
    return list;
  };

  const historySlots=new WeakMap();
  // History keeps a private storage copy and a separate cached state object.
  // Native V8 cloning handles ECMAScript exotic objects without invoking proxy traps.
  const historyUncloneableHost=value=>{
    if(blobSlots.has(value)||fileSlots.has(value))throw new DOMException('History storage of Blob and File requires platform serialization support.','NotSupportedError');
    return value===globalThis||value===document||elementData.has(value)||documentWrappers.has(value)||eventSlots.has(value);
  };
  const cloneHistoryState=value=>{
    const fail=()=>{throw new DOMException('The value could not be cloned.','DataCloneError')};
    if(historyUncloneableHost(value))fail();
    if(typeof host.cloneHistoryValue==='function'){
      const reply=host.cloneHistoryValue(value,historyUncloneableHost);if(!reply[0])throw new DOMException(reply[1],'DataCloneError');return reply[1];
    }
    // Non-native engines have no serializer. This bounded graph copy supports
    // ordinary data and common builtins; unsupported brands fail explicitly.
    const seen=new Map(),copy=input=>{
      if(typeof input==='function'||typeof input==='symbol')fail();
      if(input===null||typeof input!=='object')return input;
      if(typeof host.historyCloneIsProxy==='function'&&host.historyCloneIsProxy(input))fail();
      if(seen.has(input))return seen.get(input);
      if(historyUncloneableHost(input))fail();
      let output;
      if(Array.isArray(input))output=new Array(input.length);
      else if(input instanceof Date)output=new Date(Date.prototype.getTime.call(input));
      else if(input instanceof RegExp)output=new RegExp(input.source,input.flags);
      else if(input instanceof Map)output=new Map();
      else if(input instanceof Set)output=new Set();
      else if(input instanceof ArrayBuffer)output=input.slice(0);
      else if(ArrayBuffer.isView(input)){
        const buffer=copy(input.buffer);output=input instanceof DataView?new DataView(buffer,input.byteOffset,input.byteLength):new input.constructor(buffer,input.byteOffset,input.length);
      }else if(Object.prototype.toString.call(input)==='[object Object]')output={};
      else fail();
      seen.set(input,output);
      if(input instanceof Map){Map.prototype.forEach.call(input,(v,k)=>output.set(copy(k),copy(v)));return output}
      if(input instanceof Set){Set.prototype.forEach.call(input,v=>output.add(copy(v)));return output}
      if(Array.isArray(input)||Object.prototype.toString.call(input)==='[object Object]')for(const key of Object.keys(input))Object.defineProperty(output,key,{value:copy(input[key]),writable:true,enumerable:true,configurable:true});
      return output;
    };
    return copy(value);
  };
  registerBootstrapCallback('installHistoryClone',cloneHistoryState);
  class History { constructor(){illegal('History')} pushState(s,t,u){const error=host.historyPush(u==null?'':String(u),s);if(error)throw new DOMException(error,'SecurityError')} replaceState(s,t,u){const error=host.historyReplace(u==null?'':String(u),s);if(error)throw new DOMException(error,'SecurityError')} back(){host.historyGo(-1)} forward(){host.historyGo(1)} go(n=0){host.historyGo(Number(n)|0)} get length(){return host.historyLength()} get state(){return host.historyState()} get scrollRestoration(){return historySlots.get(this).scrollRestoration} set scrollRestoration(value){value=String(value);if(value==='auto'||value==='manual')historySlots.get(this).scrollRestoration=value} }
  const storageAreas=new WeakMap();
  class Storage { constructor(){illegal('Storage')} get length(){return host.storageLength(storageAreas.get(this))} key(i){return host.storageKey(storageAreas.get(this),Number(i)|0)} getItem(k){return host.storageGet(storageAreas.get(this),String(k))} setItem(k,v){host.storageSet(storageAreas.get(this),String(k),String(v))} removeItem(k){host.storageRemove(storageAreas.get(this),String(k))} clear(){host.storageClear(storageAreas.get(this))} }
  const trustedValueSlots=new WeakMap(),trustedPolicySlots=new WeakMap(),trustedFactorySlots=new WeakMap();
  const trustedValue=(Ctor,value)=>{const result=Object.create(Ctor.prototype);trustedValueSlots.set(result,{type:Ctor,value:String(value)});return result};
  class TrustedHTML { constructor(){illegal('TrustedHTML')} toString(){return trustedValueSlots.get(this)?.value} toJSON(){return this.toString()} }
  class TrustedScript { constructor(){illegal('TrustedScript')} toString(){return trustedValueSlots.get(this)?.value} toJSON(){return this.toString()} }
  class TrustedScriptURL { constructor(){illegal('TrustedScriptURL')} toString(){return trustedValueSlots.get(this)?.value} toJSON(){return this.toString()} }
  class TrustedTypePolicy { constructor(token,name,options){if(token!==hostToken)illegal('TrustedTypePolicy');trustedPolicySlots.set(this,{name,options:options||{}})} get name(){return trustedPolicySlots.get(this).name} createHTML(input,...args){const state=trustedPolicySlots.get(this),callback=state.options.createHTML;if(typeof callback!=='function')throw new TypeError("Policy "+state.name+" disallows creating TrustedHTML");return trustedValue(TrustedHTML,callback(String(input),...args))} createScript(input,...args){const state=trustedPolicySlots.get(this),callback=state.options.createScript;if(typeof callback!=='function')throw new TypeError("Policy "+state.name+" disallows creating TrustedScript");return trustedValue(TrustedScript,callback(String(input),...args))} createScriptURL(input,...args){const state=trustedPolicySlots.get(this),callback=state.options.createScriptURL;if(typeof callback!=='function')throw new TypeError("Policy "+state.name+" disallows creating TrustedScriptURL");return trustedValue(TrustedScriptURL,callback(String(input),...args))} }
  class TrustedTypePolicyFactory { constructor(token){if(token!==hostToken)illegal('TrustedTypePolicyFactory');trustedFactorySlots.set(this,{defaultPolicy:null})} createPolicy(name,options={}){const policy=new TrustedTypePolicy(hostToken,String(name),options);if(String(name)==='default')trustedFactorySlots.get(this).defaultPolicy=policy;return policy} isHTML(value){return trustedValueSlots.get(value)?.type===TrustedHTML} isScript(value){return trustedValueSlots.get(value)?.type===TrustedScript} isScriptURL(value){return trustedValueSlots.get(value)?.type===TrustedScriptURL} get emptyHTML(){return trustedValue(TrustedHTML,'')} get emptyScript(){return trustedValue(TrustedScript,'')} get defaultPolicy(){return trustedFactorySlots.get(this).defaultPolicy} getAttributeType(tagName,attribute){const tag=String(tagName).toLowerCase(),name=String(attribute).toLowerCase();if(name.startsWith('on'))return'TrustedScript';if(tag==='script'&&name==='src')return'TrustedScriptURL';if(tag==='iframe'&&name==='srcdoc')return'TrustedHTML';return null} getPropertyType(tagName,property){const tag=String(tagName).toLowerCase(),name=String(property);if(name==='innerHTML'||name==='outerHTML'||(tag==='iframe'&&name==='srcdoc'))return'TrustedHTML';if(tag==='script'&&name==='src')return'TrustedScriptURL';if(tag==='script'&&(name==='text'||name==='textContent'||name==='innerText'))return'TrustedScript';if(name.startsWith('on'))return'TrustedScript';return null} getTypeMapping(){return{http:{script:{src:'TrustedScriptURL',text:'TrustedScript'},iframe:{srcdoc:'TrustedHTML'},'*':{innerHTML:'TrustedHTML',outerHTML:'TrustedHTML'}}}} }
  // Use the internal brand/value, never a user-defined toString or prototype.
  const trustedValueState=WeakMap.prototype.get.bind(trustedValueSlots);
  const evalSourceResolver=value=>{const state=trustedValueState(value);return state?.type===TrustedScript?state.value:undefined};
  const readableSlots=new WeakMap(),readableState=value=>readableSlots.get(value),readableControllerSlots=new WeakMap(),readerSlots=new WeakMap(),pullReadable=stream=>{const state=readableState(stream);if(state.pulling||state.state!=='readable'||typeof state.source.pull!=='function')return;state.pulling=true;Promise.resolve().then(()=>state.source.pull(state.controller)).catch(e=>state.controller.error(e)).then(()=>{state.pulling=false;if(state.reads.length&&state.state==='readable')pullReadable(stream)})};
  class ReadableStreamDefaultController { constructor(stream){readableControllerSlots.set(this,stream)} get desiredSize(){const state=readableState(readableControllerSlots.get(this));return state.state==='readable'?1-state.queue.length:null} enqueue(chunk){const state=readableState(readableControllerSlots.get(this));if(state.state!=='readable')throw new TypeError('ReadableStream is not readable');if(state.reads.length)state.reads.shift().resolve({done:false,value:chunk});else state.queue.push(chunk)} close(){const state=readableState(readableControllerSlots.get(this));if(state.state!=='readable')return;state.state='closed';if(state.closed)state.closed.resolve();while(state.reads.length)state.reads.shift().resolve({done:true,value:undefined})} error(reason){const state=readableState(readableControllerSlots.get(this));if(state.state!=='readable')return;state.state='errored';state.error=reason;if(state.closed)state.closed.reject(reason);while(state.reads.length)state.reads.shift().reject(reason)} }
  class ReadableStreamDefaultReader { constructor(stream){if(!(stream instanceof ReadableStream))throw new TypeError('ReadableStreamDefaultReader requires a ReadableStream');if(stream.locked)throw new TypeError('ReadableStream is locked');const state=readableState(stream),closed=state.state==='closed'?Promise.resolve():state.state==='errored'?Promise.reject(state.error):new Promise((resolve,reject)=>{state.closed={resolve,reject}});readerSlots.set(this,{stream,closed});state.reader=this} get closed(){return readerSlots.get(this).closed} read(){const slot=readerSlots.get(this),stream=slot.stream;if(!stream)return Promise.reject(new TypeError('Reader has been released'));const state=readableState(stream);state.disturbed=true;if(state.queue.length)return Promise.resolve({done:false,value:state.queue.shift()});if(state.state==='closed')return Promise.resolve({done:true,value:undefined});if(state.state==='errored')return Promise.reject(state.error);const result=new Promise((resolve,reject)=>state.reads.push({resolve,reject}));pullReadable(stream);return result} cancel(reason){const stream=readerSlots.get(this).stream;return stream?stream.cancel(reason):Promise.reject(new TypeError('Reader has been released'))} releaseLock(){const slot=readerSlots.get(this),stream=slot.stream;if(!stream)return;const state=readableState(stream);if(state.reads.length)throw new TypeError('Cannot release a reader with pending reads');state.reader=null;slot.stream=null} }
  class ReadableStream { constructor(source={},strategy={}){const state={source:source||{},queue:[],reads:[],reader:null,state:'readable',error:undefined,disturbed:false,pulling:false,closed:null,controller:null};readableSlots.set(this,state);state.controller=new ReadableStreamDefaultController(this);try{const started=typeof state.source.start==='function'?state.source.start(state.controller):undefined;Promise.resolve(started).catch(e=>state.controller.error(e))}catch(e){state.controller.error(e)}} get locked(){return readableState(this).reader!==null} cancel(reason){const state=readableState(this);if(this.locked)return Promise.reject(new TypeError('Cannot cancel a locked stream'));state.queue.length=0;state.controller.close();try{return Promise.resolve(typeof state.source.cancel==='function'?state.source.cancel(reason):undefined)}catch(e){return Promise.reject(e)}} getReader(){return new ReadableStreamDefaultReader(this)} pipeThrough(transform,options){this.pipeTo(transform.writable,options);return transform.readable} async pipeTo(destination){const reader=this.getReader(),writer=destination.getWriter();try{for(;;){const result=await reader.read();if(result.done)break;await writer.write(result.value)}await writer.close()}catch(e){await writer.abort(e);throw e}finally{reader.releaseLock();writer.releaseLock()}} tee(){const a=new ReadableStream(),b=new ReadableStream(),ca=readableState(a).controller,cb=readableState(b).controller,reader=this.getReader();(async()=>{try{for(;;){const r=await reader.read();if(r.done){ca.close();cb.close();break}ca.enqueue(r.value);cb.enqueue(r.value)}}catch(e){ca.error(e);cb.error(e)}})();return[a,b]} values(){const reader=this.getReader();return{next:()=>reader.read(),return:async()=>{await reader.cancel();reader.releaseLock();return{done:true}},[Symbol.asyncIterator](){return this}}} [Symbol.asyncIterator](){return this.values()} }
  const writerSlots=new WeakMap();
  class WritableStreamDefaultWriter { get ready(){return writerSlots.get(this).ready} get closed(){return writerSlots.get(this).closed} constructor(stream){if(!(stream instanceof WritableStream)||stream.locked)throw new TypeError('WritableStream is locked');this.__stream=stream;stream.__writer=this;writerSlots.set(this,{ready:Promise.resolve(),closed:stream.__state==='closed'?Promise.resolve():new Promise((resolve,reject)=>stream.__closed={resolve,reject})})} write(chunk){return this.__stream.__write(chunk)} close(){return this.__stream.close()} abort(reason){return this.__stream.abort(reason)} releaseLock(){if(this.__stream){this.__stream.__writer=null;this.__stream=null}} }
  class WritableStream { constructor(sink={},strategy={}){this.__sink=sink||{};this.__writer=null;this.__state='writable';this.__closed=null;try{Promise.resolve(typeof this.__sink.start==='function'?this.__sink.start(this):undefined).catch(e=>this.__fail(e))}catch(e){this.__fail(e)}} get locked(){return this.__writer!==null} getWriter(){return new WritableStreamDefaultWriter(this)} __write(chunk){if(this.__state!=='writable')return Promise.reject(new TypeError('WritableStream is not writable'));try{return Promise.resolve(typeof this.__sink.write==='function'?this.__sink.write(chunk,this):undefined)}catch(e){return Promise.reject(e)}} close(){if(this.__state!=='writable')return Promise.reject(new TypeError('WritableStream is not writable'));this.__state='closed';try{return Promise.resolve(typeof this.__sink.close==='function'?this.__sink.close():undefined).then(v=>{if(this.__closed)this.__closed.resolve();return v})}catch(e){return Promise.reject(e)}} abort(reason){this.__state='errored';try{return Promise.resolve(typeof this.__sink.abort==='function'?this.__sink.abort(reason):undefined).then(v=>{if(this.__closed)this.__closed.reject(reason);return v})}catch(e){return Promise.reject(e)}} __fail(e){this.__state='errored';if(this.__closed)this.__closed.reject(e)} }
  class TransformStreamDefaultController { constructor(readable){this.__readable=readable} enqueue(chunk){readableState(this.__readable).controller.enqueue(chunk)} error(reason){readableState(this.__readable).controller.error(reason)} terminate(){readableState(this.__readable).controller.close()} }
  const transformSlots=new WeakMap();
  class TransformStream { get readable(){return transformSlots.get(this).readable} get writable(){return transformSlots.get(this).writable} constructor(transformer={}){const slot={readable:new ReadableStream(),writable:null};transformSlots.set(this,slot);const controller=new TransformStreamDefaultController(slot.readable);slot.writable=new WritableStream({start:()=>typeof transformer.start==='function'?transformer.start(controller):undefined,write:chunk=>typeof transformer.transform==='function'?transformer.transform(chunk,controller):controller.enqueue(chunk),close:()=>Promise.resolve(typeof transformer.flush==='function'?transformer.flush(controller):undefined).then(()=>controller.terminate()),abort:reason=>controller.error(reason)})} }
  const workerSlots=new WeakMap();
  class Worker extends EventTarget { constructor(scriptURL,options={}){super();const state={id:0,onmessage:null,onmessageerror:null,onerror:null};workerSlots.set(this,state);const deliver=data=>dispatchTrusted(this,new MessageEvent('message',{data})),report=message=>dispatchTrusted(this,new ErrorEvent('error',{message}));state.id=host.createWorker(deliver,report,String(scriptURL),'',String(options.type||'classic'),String(options.name||''))} postMessage(message){host.workerPost(workerSlots.get(this).id,message)} terminate(){host.terminateWorker(workerSlots.get(this).id)} }
  for(const key of ['onmessage','onmessageerror','onerror'])def(Worker.prototype,key,{get(){return workerSlots.get(this)[key]},set(value){workerSlots.get(this)[key]=value}});
  /* shared_base64 */
  const cryptoKeySlot=Symbol('CryptoKey slots');
  class CryptoKey { constructor(){illegal('CryptoKey')} get type(){const slot=this&&this[cryptoKeySlot];if(!slot)throw new TypeError('Illegal invocation');return slot.type} get extractable(){const slot=this&&this[cryptoKeySlot];if(!slot)throw new TypeError('Illegal invocation');return slot.extractable} get algorithm(){const slot=this&&this[cryptoKeySlot];if(!slot)throw new TypeError('Illegal invocation');return slot.algorithm} get usages(){const slot=this&&this[cryptoKeySlot];if(!slot)throw new TypeError('Illegal invocation');return slot.usages.slice()} }
  const cryptoBytes=data=>{if(!ArrayBuffer.isView(data)&&!(data instanceof ArrayBuffer))throw new TypeError("The provided value is not of type '(ArrayBuffer or ArrayBufferView)'");return ArrayBuffer.isView(data)?Uint8Array.from(data):new Uint8Array(data)};
  const cryptoAlgorithmName=algorithm=>String(typeof algorithm==='string'?algorithm:algorithm&&algorithm.name).toUpperCase().replaceAll('_','-');
  class SubtleCrypto { constructor(){illegal('SubtleCrypto')} digest(algorithm,data){const name=typeof algorithm==='string'?algorithm:algorithm&&algorithm.name;try{const bytes=host.subtleDigest(String(name),Array.from(cryptoBytes(data))),result=new Uint8Array(bytes);return Promise.resolve(result.buffer)}catch(error){return Promise.reject(error)}} importKey(format,keyData,algorithm,extractable,keyUsages){try{if(String(format)!=='spki'||cryptoAlgorithmName(algorithm)!=='RSA-OAEP')throw new DOMException('The operation is not supported','NotSupportedError');const hashName=typeof algorithm.hash==='string'?algorithm.hash:algorithm.hash&&algorithm.hash.name,hash=String(hashName).toUpperCase().replaceAll('_','-'),der=Array.from(cryptoBytes(keyData)),usages=Array.from(keyUsages||[],String);if(usages.some(usage=>usage!=='encrypt'))throw new DOMException('Unsupported key usage for an RSA-OAEP key','SyntaxError');const metadata=host.subtleImportRSAOAEP(der),key=Object.create(CryptoKey.prototype),slot={type:'public',extractable:Boolean(extractable),algorithm:{name:'RSA-OAEP',modulusLength:metadata[0],publicExponent:new Uint8Array(metadata[1]),hash:{name:hash}},usages,der,hash};Object.defineProperty(key,cryptoKeySlot,{value:slot});return Promise.resolve(key)}catch(error){return Promise.reject(error)}} encrypt(algorithm,key,data){try{const slot=key&&key[cryptoKeySlot];if(!slot)throw new TypeError("Failed to execute 'encrypt' on 'SubtleCrypto': parameter 2 is not of type 'CryptoKey'.");if(cryptoAlgorithmName(algorithm)!=='RSA-OAEP'||slot.algorithm.name!=='RSA-OAEP'||!slot.usages.includes('encrypt'))throw new DOMException('The requested operation is not valid for the provided key','InvalidAccessError');const label=algorithm&&algorithm.label!==undefined?Array.from(cryptoBytes(algorithm.label)):[],bytes=host.subtleRSAOAEPEncrypt(slot.hash,slot.der,Array.from(cryptoBytes(data)),label),result=new Uint8Array(bytes);return Promise.resolve(result.buffer)}catch(error){return Promise.reject(error)}} decrypt(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} sign(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} verify(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} exportKey(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} generateKey(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} deriveKey(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} deriveBits(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} wrapKey(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} unwrapKey(){return Promise.reject(new DOMException('The operation is not supported','NotSupportedError'))} }
  SubtleCrypto.prototype.importKey=function(format,keyData,algorithm,extractable,keyUsages){try{if(String(format)!=='spki'||cryptoAlgorithmName(algorithm)!=='RSA-OAEP')throw new DOMException('The operation is not supported','NotSupportedError');const hashName=typeof algorithm.hash==='string'?algorithm.hash:algorithm.hash&&algorithm.hash.name,hash=String(hashName).toUpperCase().replaceAll('_','-'),der=Array.from(cryptoBytes(keyData)),usages=Array.from(keyUsages||[],String);if(usages.some(usage=>usage!=='encrypt'))throw new DOMException('Unsupported key usage for an RSA-OAEP key','SyntaxError');const metadata=String(host.subtleImportRSAOAEP(der)).split('|'),key=Object.create(CryptoKey.prototype),slot={type:'public',extractable:Boolean(extractable),algorithm:{name:'RSA-OAEP',modulusLength:Number(metadata[0]),publicExponent:new Uint8Array(metadata[1].split(',').map(Number)),hash:{name:hash}},usages,der,hash};Object.defineProperty(key,cryptoKeySlot,{value:slot});return Promise.resolve(key)}catch(error){return Promise.reject(error)}};
  const subtleCrypto=Object.create(SubtleCrypto.prototype);
  class Crypto { constructor(){illegal('Crypto')} get subtle(){return subtleCrypto} getRandomValues(view){if(!ArrayBuffer.isView(view)||view instanceof Float32Array||view instanceof Float64Array||view instanceof DataView)throw new TypeError("Failed to execute 'getRandomValues' on 'Crypto': parameter 1 is not of type 'ArrayBufferView'.");if(view.byteLength>65536)throw new DOMException('The ArrayBufferView\'s byte length exceeds the number of bytes of entropy available via this API (65536).','QuotaExceededError');const bytes=host.randomBytes(view.byteLength),raw=new Uint8Array(view.buffer,view.byteOffset,view.byteLength);for(let i=0;i<raw.length;i++)raw[i]=bytes[i];return view} randomUUID(){return host.randomUUID()} }
  const performanceEntrySlots=new WeakMap();
  class PerformanceEntry { constructor(token,data){if(token!==hostToken)illegal('PerformanceEntry');performanceEntrySlots.set(this,data)} get name(){return performanceEntrySlots.get(this).name} get entryType(){return performanceEntrySlots.get(this).entryType} get startTime(){return performanceEntrySlots.get(this).startTime} get duration(){return performanceEntrySlots.get(this).duration} get navigationId(){return performanceEntrySlots.get(this).navigationId||0} toJSON(){return Object.assign({},performanceEntrySlots.get(this))} }
  const performanceServerTimingSlots=new WeakMap();
  class PerformanceServerTiming { constructor(token,data){if(token!==hostToken)illegal('PerformanceServerTiming');performanceServerTimingSlots.set(this,data)} get name(){return performanceServerTimingSlots.get(this).name||''} get duration(){return performanceServerTimingSlots.get(this).duration||0} get description(){return performanceServerTimingSlots.get(this).description||''} toJSON(){return{name:this.name,duration:this.duration,description:this.description}} }
  const makeServerTiming=data=>new PerformanceServerTiming(hostToken,data);
  // Serialize the internal entry state. WebIDL serialization does not invoke
  // replaceable public getters, and default-valued attributes remain present.
  function resourceTimingJSON(entry){
    const data=performanceEntrySlots.get(entry);
    if(!data||(data.entryType!=='resource'&&data.entryType!=='navigation'))throw new TypeError('Illegal invocation');
    const result={},strings=new Set(['name','entryType','initiatorType','deliveryType','nextHopProtocol','contentType']);
    const fields=['name','entryType','startTime','duration','navigationId','initiatorType','deliveryType','nextHopProtocol','contentType','workerStart','redirectStart','redirectEnd','fetchStart','domainLookupStart','domainLookupEnd','connectStart','secureConnectionStart','connectEnd','requestStart','responseStart','firstInterimResponseStart','responseEnd','transferSize','encodedBodySize','decodedBodySize','responseStatus'];
    for(const name of fields)result[name]=data[name]??(strings.has(name)?'':0);
    result.serverTiming=Object.freeze((data.serverTiming||[]).map(makeServerTiming));
    // Navigation attributes share the authoritative entry state. Other resource
    // attributes remain explicit implementation boundaries, not guessed values.
    for(const name of Object.keys(data))if(!(name in result))result[name]=data[name];
    return result;
  }
  class PerformanceResourceTiming extends PerformanceEntry { constructor(token,data){super(token,data)} get initiatorType(){return performanceEntrySlots.get(this).initiatorType||''} get nextHopProtocol(){return performanceEntrySlots.get(this).nextHopProtocol||''} get workerStart(){return performanceEntrySlots.get(this).workerStart||0} get redirectStart(){return performanceEntrySlots.get(this).redirectStart||0} get redirectEnd(){return performanceEntrySlots.get(this).redirectEnd||0} get fetchStart(){return performanceEntrySlots.get(this).fetchStart||0} get domainLookupStart(){return performanceEntrySlots.get(this).domainLookupStart||0} get domainLookupEnd(){return performanceEntrySlots.get(this).domainLookupEnd||0} get connectStart(){return performanceEntrySlots.get(this).connectStart||0} get secureConnectionStart(){return performanceEntrySlots.get(this).secureConnectionStart||0} get connectEnd(){return performanceEntrySlots.get(this).connectEnd||0} get requestStart(){return performanceEntrySlots.get(this).requestStart||0} get responseStart(){return performanceEntrySlots.get(this).responseStart||0} get firstInterimResponseStart(){return performanceEntrySlots.get(this).firstInterimResponseStart||0} get responseEnd(){return performanceEntrySlots.get(this).responseEnd||0} get transferSize(){return performanceEntrySlots.get(this).transferSize||0} get encodedBodySize(){return performanceEntrySlots.get(this).encodedBodySize||0} get decodedBodySize(){return performanceEntrySlots.get(this).decodedBodySize||0} get responseStatus(){return performanceEntrySlots.get(this).responseStatus||0} get serverTiming(){return Object.freeze((performanceEntrySlots.get(this).serverTiming||[]).map(makeServerTiming))} get contentType(){return performanceEntrySlots.get(this).contentType||''} get deliveryType(){return performanceEntrySlots.get(this).deliveryType||''} toJSON(){return resourceTimingJSON(this)} }
  class PerformanceNavigationTiming extends PerformanceResourceTiming { constructor(token,data){super(token,data)} get type(){return performanceEntrySlots.get(this).type||'navigate'} get redirectCount(){return performanceEntrySlots.get(this).redirectCount||0} get activationStart(){return performanceEntrySlots.get(this).activationStart||0} }
  class PerformanceMark extends PerformanceEntry { constructor(token,data){super(token,data)} get detail(){return performanceEntrySlots.get(this).detail??null} }
  class PerformanceTiming { constructor(){illegal('PerformanceTiming')} }
  const makePerformanceEntry=data=>data.entryType==='resource'?new PerformanceResourceTiming(hostToken,data):data.entryType==='navigation'?new PerformanceNavigationTiming(hostToken,data):new PerformanceEntry(hostToken,data);
  const performanceObserverListSlots=new WeakMap(),performanceObserverSlots=new WeakMap(),performanceObservers=new Set();
  class PerformanceObserverEntryList { constructor(entries){performanceObserverListSlots.set(this,entries.map(makePerformanceEntry))} getEntries(){return performanceObserverListSlots.get(this).slice()} getEntriesByType(type){return performanceObserverListSlots.get(this).filter(x=>x.entryType===String(type))} getEntriesByName(name,type){return performanceObserverListSlots.get(this).filter(x=>x.name===String(name)&&(type===undefined||x.entryType===String(type)))} }
  let navigationFinalized=false;
  const performanceEntryKey=entry=>entry.entryType+'\n'+entry.name+'\n'+entry.startTime+'\n'+entry.duration+(entry.entryType==='navigation'?'\n'+navigationFinalized:'');
  const queuePerformanceDelivery=observer=>{const state=performanceObserverSlots.get(observer);if(!state||!state.active||state.queued||!state.records.length)return;state.queued=true;host.queuePerformanceObserver(()=>{state.queued=false;if(!state.active)return;const records=state.records.splice(0);if(records.length)state.callback(new PerformanceObserverEntryList(records),observer)})};
  class PerformanceObserver { constructor(callback){if(typeof callback!=='function')throw new TypeError("Failed to construct 'PerformanceObserver': parameter 1 is not of type 'PerformanceObserverCallback'.");performanceObserverSlots.set(this,{callback,records:[],active:true,types:[],seen:new Set(),queued:false})} observe(options={}){const state=performanceObserverSlots.get(this);state.active=true;const types=Array.isArray(options.entryTypes)?options.entryTypes.map(String):(options.type!==undefined?[String(options.type)]:[]);if(!types.length)throw new TypeError("Failed to execute 'observe' on 'PerformanceObserver': An observe() call must include either entryTypes or type arguments.");state.types=types;const current=host.performanceEntries(types,false);for(const entry of current)state.seen.add(performanceEntryKey(entry));if(options.buffered&&current.length)state.records.push(...current);performanceObservers.add(this);queuePerformanceDelivery(this)} disconnect(){const state=performanceObserverSlots.get(this);state.active=false;state.records.length=0;state.queued=false;performanceObservers.delete(this)} takeRecords(){return performanceObserverSlots.get(this).records.splice(0)} static get supportedEntryTypes(){return ['element','event','first-input','largest-contentful-paint','layout-shift','long-animation-frame','longtask','mark','measure','navigation','paint','resource','visibility-state']} }
  const intersectionObserverSlots=new WeakMap();
  class IntersectionObserver { constructor(callback,options={}){if(typeof callback!=='function')throw new TypeError("Failed to construct 'IntersectionObserver': parameter 1 is not of type 'IntersectionObserverCallback'.");const thresholds=(options.threshold===undefined?[0]:(Array.isArray(options.threshold)?options.threshold:[options.threshold])).map(Number).sort((a,b)=>a-b);if(thresholds.some(value=>!Number.isFinite(value)||value<0||value>1))throw new RangeError('Threshold values must be numbers between 0 and 1');intersectionObserverSlots.set(this,{callback,root:options.root||null,rootMargin:String(options.rootMargin||'0px 0px 0px 0px'),scrollMargin:String(options.scrollMargin||'0px 0px 0px 0px'),thresholds:[...new Set(thresholds)],records:[],targets:new Set()})} get root(){return intersectionObserverSlots.get(this).root} get rootMargin(){return intersectionObserverSlots.get(this).rootMargin} get scrollMargin(){return intersectionObserverSlots.get(this).scrollMargin} get thresholds(){return intersectionObserverSlots.get(this).thresholds.slice()} observe(target){if(!(target instanceof Element))throw new TypeError("Failed to execute 'observe' on 'IntersectionObserver': parameter 1 is not of type 'Element'.");intersectionObserverSlots.get(this).targets.add(target)} unobserve(target){intersectionObserverSlots.get(this).targets.delete(target)} disconnect(){const state=intersectionObserverSlots.get(this);state.targets.clear();state.records.length=0} takeRecords(){return intersectionObserverSlots.get(this).records.splice(0)} }
  Object.defineProperty(globalThis,'__mimicNotifyPerformanceObservers',{value:finalized=>{navigationFinalized=!!finalized;for(const observer of performanceObservers){const state=performanceObserverSlots.get(observer);if(!state||!state.active)continue;for(const entry of host.performanceEntries(state.types,false)){const id=performanceEntryKey(entry);if(!state.seen.has(id)){state.seen.add(id);state.records.push(entry)}}queuePerformanceDelivery(observer)}},configurable:true});
  const performanceMarks=[];
  class Performance { constructor(){illegal('Performance')} get timeOrigin(){const binding=requireRealmBinding(this,'Performance');return callRealmBinding(this,binding,'timeOrigin',[])} get timing(){const value=Object.create(PerformanceTiming.prototype),origin=Math.trunc(this.timeOrigin);for(const name of ['navigationStart','unloadEventStart','unloadEventEnd','redirectStart','redirectEnd','fetchStart','domainLookupStart','domainLookupEnd','connectStart','connectEnd','secureConnectionStart','requestStart','responseStart','responseEnd','domLoading','domInteractive','domContentLoadedEventStart','domContentLoadedEventEnd','domComplete','loadEventStart','loadEventEnd'])Object.defineProperty(value,name,{value:name==='navigationStart'||name==='fetchStart'?origin:0,enumerable:true});return value} now(){const binding=requireRealmBinding(this,'Performance');return callRealmBinding(this,binding,'now',[])} mark(name,options={}){const mark=new PerformanceMark(hostToken,{name:String(name),entryType:'mark',startTime:options.startTime===undefined?this.now():Number(options.startTime),duration:0,detail:options.detail??null});performanceMarks.push(mark);return mark} getEntries(){const binding=requireRealmBinding(this,'Performance');return callRealmBinding(this,binding,'entries',[])} getEntriesByType(type){const binding=requireRealmBinding(this,'Performance');if(!arguments.length)throw new TypeError('Not enough arguments');type=bindingString(type);return callRealmBinding(this,binding,'entriesByType',[type])} getEntriesByName(name,type=undefined){const binding=requireRealmBinding(this,'Performance');if(!arguments.length)throw new TypeError('Not enough arguments');name=bindingString(name);if(type!==undefined)type=bindingString(type);return callRealmBinding(this,binding,'entriesByName',[name,type])} }
  class GPUAdapterInfo { constructor(){illegal("GPUAdapterInfo")}}
  class GPUSupportedFeatures { constructor(){illegal("GPUSupportedFeatures")}}
  class GPUSupportedLimits { constructor(){illegal("GPUSupportedLimits")}}
  class GPUAdapter { constructor(){illegal("GPUAdapter")}}
  class GPUDevice extends EventTarget { constructor(){super();illegal("GPUDevice")}}
  class GPU { constructor(){illegal("GPU")}}
  const rtcDescriptionSlots=new WeakMap();
  const rtcDescriptionState=value=>{const state=rtcDescriptionSlots.get(value);if(!state)throw new TypeError('Illegal invocation');return state};
  class RTCSessionDescription { constructor(init={}){init=init==null?{}:Object(init);rtcDescriptionSlots.set(this,{type:String(init.type||''),sdp:String(init.sdp||'')})} get type(){return rtcDescriptionState(this).type} get sdp(){return rtcDescriptionState(this).sdp} toJSON(){const state=rtcDescriptionState(this);return{type:state.type,sdp:state.sdp}} }
  const rtcCandidateSlots=new WeakMap(),rtcIceEventSlots=new WeakMap();
  class RTCIceCandidate { constructor(init={}){if(init.sdpMid==null&&init.sdpMLineIndex==null&&String(init.candidate||''))throw new TypeError("Failed to construct 'RTCIceCandidate': sdpMid and sdpMLineIndex are both null.");rtcCandidateSlots.set(this,{candidate:String(init.candidate||''),sdpMid:init.sdpMid==null?null:String(init.sdpMid),sdpMLineIndex:init.sdpMLineIndex==null?null:Number(init.sdpMLineIndex),usernameFragment:init.usernameFragment==null?null:String(init.usernameFragment)})} get candidate(){return rtcCandidateSlots.get(this).candidate} get sdpMid(){return rtcCandidateSlots.get(this).sdpMid} get sdpMLineIndex(){return rtcCandidateSlots.get(this).sdpMLineIndex} get usernameFragment(){return rtcCandidateSlots.get(this).usernameFragment} toJSON(){return{candidate:this.candidate,sdpMid:this.sdpMid,sdpMLineIndex:this.sdpMLineIndex,usernameFragment:this.usernameFragment}} }
  class RTCPeerConnectionIceEvent extends Event { constructor(type,init={}){super(type,init);rtcIceEventSlots.set(this,{candidate:init.candidate??null,url:String(init.url||'')})} get candidate(){return rtcIceEventSlots.get(this).candidate} get url(){return rtcIceEventSlots.get(this).url} }
  const rtcDataStates=new WeakMap(),rtcPeerStates=new WeakMap(),rtcPeerPrivate=new WeakMap();
  class RTCDataChannel extends EventTarget { constructor(token,label,options={}){super();if(token!==hostToken)illegal('RTCDataChannel');rtcDataStates.set(this,{label:String(label),ordered:options.ordered===undefined?true:Boolean(options.ordered),maxPacketLifeTime:options.maxPacketLifeTime==null?null:Number(options.maxPacketLifeTime),maxRetransmits:options.maxRetransmits==null?null:Number(options.maxRetransmits),protocol:String(options.protocol||''),negotiated:Boolean(options.negotiated),id:options.id==null?null:Number(options.id),readyState:'connecting',bufferedAmount:0,bufferedAmountLowThreshold:0,binaryType:'arraybuffer',onopen:null,onbufferedamountlow:null,onerror:null,onclosing:null,onclose:null,onmessage:null})} get reliable(){const state=rtcDataStates.get(this);return state.ordered&&state.maxPacketLifeTime===null&&state.maxRetransmits===null} send(){if(rtcDataStates.get(this).readyState!=='open')throw new DOMException('RTCDataChannel.readyState is not open','InvalidStateError')} close(){const state=rtcDataStates.get(this);if(state.readyState==='closed'||state.readyState==='closing')return;state.readyState='closing'} }
  for(const key of ['label','ordered','maxPacketLifeTime','maxRetransmits','protocol','negotiated','id','readyState','bufferedAmount'])def(RTCDataChannel.prototype,key,{get(){return rtcDataStates.get(this)?.[key]}});
  for(const key of ['bufferedAmountLowThreshold','binaryType','onopen','onbufferedamountlow','onerror','onclosing','onclose','onmessage'])def(RTCDataChannel.prototype,key,{get(){return rtcDataStates.get(this)?.[key]},set(value){rtcDataStates.get(this)[key]=value}});
  const normalizeRTCConfiguration=configuration=>{configuration=configuration||{};return{iceServers:Array.from(configuration.iceServers||[],server=>({urls:Array.isArray(server.urls)?server.urls.map(String):[String(server.urls||'')],username:String(server.username||''),credential:String(server.credential||'')})),iceTransportPolicy:String(configuration.iceTransportPolicy||'all'),bundlePolicy:String(configuration.bundlePolicy||'balanced'),rtcpMuxPolicy:'require',iceCandidatePoolSize:Number(configuration.iceCandidatePoolSize||0),certificates:Array.from(configuration.certificates||[]),encodedInsertableStreams:Boolean(configuration.encodedInsertableStreams),alwaysNegotiateDataChannels:Boolean(configuration.alwaysNegotiateDataChannels)}};
  class RTCPeerConnection extends EventTarget { constructor(configuration={}){super();rtcPeerStates.set(this,{localDescription:null,currentLocalDescription:null,pendingLocalDescription:null,remoteDescription:null,currentRemoteDescription:null,pendingRemoteDescription:null,signalingState:'stable',iceGatheringState:'new',iceConnectionState:'new',connectionState:'new',canTrickleIceCandidates:null,onicecandidate:null,onicegatheringstatechange:null,oniceconnectionstatechange:null,onsignalingstatechange:null,onconnectionstatechange:null,ondatachannel:null});rtcPeerPrivate.set(this,{configuration:normalizeRTCConfiguration(configuration),closed:false,hasDataChannel:false,iceUfrag:''});return observe('RTCPeerConnection',this)} getConfiguration(){const configuration=rtcPeerPrivate.get(this).configuration;return{...configuration,iceServers:configuration.iceServers.map(server=>({...server,urls:server.urls.slice()})),certificates:configuration.certificates.slice()}} setConfiguration(v){rtcPeerPrivate.get(this).configuration=normalizeRTCConfiguration(v)} createOffer(){if(rtcPeerPrivate.get(this).closed)return Promise.reject(new DOMException('The RTCPeerConnection is closed.','InvalidStateError'));return Promise.resolve(new RTCSessionDescription({type:'offer',sdp:'v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n'}))} createAnswer(){return Promise.resolve(new RTCSessionDescription({type:'answer',sdp:'v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n'}))} setLocalDescription(description){const state=rtcPeerStates.get(this),value=description instanceof RTCSessionDescription?description:new RTCSessionDescription(description||{type:'offer'});state.localDescription=value;state.currentLocalDescription=value;state.iceGatheringState='complete';return Promise.resolve()} setRemoteDescription(description){const state=rtcPeerStates.get(this),value=description instanceof RTCSessionDescription?description:new RTCSessionDescription(description);state.remoteDescription=value;state.currentRemoteDescription=value;return Promise.resolve()} addIceCandidate(){return Promise.resolve()} getSenders(){return[]} getReceivers(){return[]} getTransceivers(){return[]} close(){rtcPeerPrivate.get(this).closed=true;const state=rtcPeerStates.get(this);state.signalingState='closed';state.iceConnectionState='closed';state.connectionState='closed'} }
  for(const key of ['localDescription','currentLocalDescription','pendingLocalDescription','remoteDescription','currentRemoteDescription','pendingRemoteDescription','signalingState','iceGatheringState','iceConnectionState','connectionState','canTrickleIceCandidates'])def(RTCPeerConnection.prototype,key,{get(){return rtcPeerStates.get(this)?.[key]}});
  for(const key of ['onicecandidate','onicegatheringstatechange','oniceconnectionstatechange','onsignalingstatechange','onconnectionstatechange','ondatachannel'])def(RTCPeerConnection.prototype,key,{get(){return rtcPeerStates.get(this)?.[key]},set(value){rtcPeerStates.get(this)[key]=value}});
  Object.defineProperty(RTCPeerConnection.prototype,'createDataChannel',{value:function(label,options={}){const state=rtcPeerPrivate.get(this);if(state.closed)throw new DOMException('The RTCPeerConnection is closed.','InvalidStateError');state.hasDataChannel=true;return new RTCDataChannel(hostToken,label,options)},writable:true,configurable:true});
  const rtcRandomBytes=n=>Uint8Array.from(host.internalRandomBytes(n));
  const rtcHex=(bytes,separator='')=>Array.from(bytes,x=>x.toString(16).padStart(2,'0').toUpperCase()).join(separator);
  const rtcBase64=bytes=>btoa(String.fromCharCode(...bytes));
  const rtcOfferSDP=peer=>{const state=rtcPeerPrivate.get(peer),random=rtcRandomBytes(32),ufrag=rtcBase64(random.slice(0,3)),pwd=rtcBase64(random.slice(3,21)),fingerprint=rtcHex(rtcRandomBytes(32),':'),session=String((BigInt('0x'+rtcHex(random.slice(21,29)))%9223372036854775807n)||1n),data=state.hasDataChannel?'m=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\nc=IN IP4 0.0.0.0\r\n':'';state.iceUfrag=ufrag;return 'v=0\r\no=- '+session+' 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE 0\r\na=extmap-allow-mixed\r\na=msid-semantic: WMS\r\n'+data+'a=ice-ufrag:'+ufrag+'\r\na=ice-pwd:'+pwd+'\r\na=ice-options:trickle\r\na=fingerprint:sha-256 '+fingerprint+'\r\na=setup:actpass\r\na=mid:0\r\na=sctp-port:5000\r\na=max-message-size:262144\r\n'};
  Object.defineProperty(RTCPeerConnection.prototype,'createOffer',{value:function(){if(rtcPeerPrivate.get(this).closed)return Promise.reject(new DOMException('The RTCPeerConnection is closed.','InvalidStateError'));return Promise.resolve(new RTCSessionDescription({type:'offer',sdp:rtcOfferSDP(this)}))},writable:true,configurable:true});
  Object.defineProperty(RTCPeerConnection.prototype,'setLocalDescription',{value:function(description){
    const state=rtcPeerPrivate.get(this);
    if(state.closed)return Promise.reject(new DOMException('The RTCPeerConnection is closed.','InvalidStateError'));
    const value=description instanceof RTCSessionDescription?description:new RTCSessionDescription(description||{type:'offer',sdp:rtcOfferSDP(this)});
    const observable=rtcPeerStates.get(this);observable.localDescription=value;observable.pendingLocalDescription=value;
    if(value.type==='offer')observable.signalingState='have-local-offer';else if(value.type==='answer')observable.signalingState='stable';
    setTimeout(()=>{
      if(state.closed)return;
      dispatchTrusted(this,new Event('signalingstatechange'));
      observable.iceGatheringState='gathering';dispatchTrusted(this,new Event('icegatheringstatechange'));
	      const environment=host.rtcEnvironment(),hasSTUN=state.configuration.iceServers.some(server=>server.urls.some(url=>/^stuns?:/i.test(url))),count=hasSTUN?Math.max(1,Number(environment.hostCandidateCount)||1):1,reflexiveCount=hasSTUN?Math.max(0,Number(environment.reflexiveCandidateCount)||0):0,networkCost=Math.max(0,Number(environment.networkCost)||0),id=host.internalRandomUUID(),hostFoundation=String(parseInt(rtcHex(rtcRandomBytes(4)),16)>>>0),reflexiveFoundation=String(parseInt(rtcHex(rtcRandomBytes(4)),16)>>>0),priority=2113937151,reflexivePriority=1677729535,basePort=49152+(parseInt(rtcHex(rtcRandomBytes(2)),16)%16380),hostCandidates=[],reflexiveCandidates=[];
      for(let index=0;index<count;index++){
        const port=basePort+Number((environment.portOffsets||[])[index]??index*2),suffix=' generation 0 ufrag '+String(state.iceUfrag||'')+' network-cost '+networkCost;
        hostCandidates.push(new RTCIceCandidate({candidate:'candidate:'+hostFoundation+' 1 udp '+priority+' '+id+'.local '+port+' typ host'+suffix,sdpMid:'0',sdpMLineIndex:0,usernameFragment:state.iceUfrag||null}));
      }
	      if(hasSTUN&&environment.publicAddress)for(let index=0;index<reflexiveCount;index++){
        const port=basePort+Number((environment.reflexivePortOffsets||environment.portOffsets||[])[index]??index*2),suffix=' generation 0 ufrag '+String(state.iceUfrag||'')+' network-cost '+networkCost;
        reflexiveCandidates.push(new RTCIceCandidate({candidate:'candidate:'+reflexiveFoundation+' 1 udp '+reflexivePriority+' '+String(environment.publicAddress)+' '+port+' typ srflx raddr 0.0.0.0 rport 0'+suffix,sdpMid:'0',sdpMLineIndex:0,usernameFragment:state.iceUfrag||null}));
      }
      const withCandidates=candidates=>{const candidateLines=candidates.map(candidate=>'a='+candidate.candidate.replace(/ ufrag [^ ]+(?= network-cost)/,'')+'\r\n').join(''),withAddress=hasSTUN?value.sdp.replace('m=application 9 UDP/DTLS/SCTP','m=application '+basePort+' UDP/DTLS/SCTP').replace('c=IN IP4 0.0.0.0','c=IN IP4 '+String(environment.publicAddress||'0.0.0.0')):value.sdp,candidateIndex=withAddress.indexOf('a=ice-ufrag:'),sdp=candidateIndex<0?withAddress+candidateLines:withAddress.slice(0,candidateIndex)+candidateLines+withAddress.slice(candidateIndex);return new RTCSessionDescription({type:value.type,sdp})};
      const hostDescription=withCandidates(hostCandidates);observable.localDescription=hostDescription;observable.pendingLocalDescription=hostDescription;
      for(const candidate of hostCandidates)dispatchTrusted(this,new RTCPeerConnectionIceEvent('icecandidate',{candidate}));
      setTimeout(()=>{if(state.closed)return;const allCandidates=hostCandidates.concat(reflexiveCandidates),reflexiveDescription=withCandidates(allCandidates);observable.localDescription=reflexiveDescription;observable.pendingLocalDescription=reflexiveDescription;for(const candidate of reflexiveCandidates)dispatchTrusted(this,new RTCPeerConnectionIceEvent('icecandidate',{candidate}));setTimeout(()=>{if(state.closed)return;observable.iceGatheringState='complete';dispatchTrusted(this,new Event('icegatheringstatechange'));dispatchTrusted(this,new RTCPeerConnectionIceEvent('icecandidate',{candidate:null}))},Math.max(0,Number(environment.endDelayMillis)-Number(environment.reflexiveDelayMillis)))},Math.max(0,Number(environment.reflexiveDelayMillis)-Number(environment.hostDelayMillis)));
    },Math.max(0,Number(host.rtcEnvironment().hostDelayMillis)||0));
    return Promise.resolve()
  },writable:true,configurable:true});
  globalThis.RTCDataChannel=RTCDataChannel;
  Object.defineProperty(RTCDataChannel.prototype,Symbol.toStringTag,{value:'RTCDataChannel',configurable:true});
  const targetAbsentProperties=new Set(['Document.namespaces','Element<script>.crossorigin']);
  // Traps depend on the interface label, never on one particular wrapper.
  // Share their code and qualified property names; still check support on every
  // access so own properties and prototype changes retain their tracing semantics.
  const observationHandlers=new Map();
  const observationHandler=name=>{
    let handler=observationHandlers.get(name);if(handler)return handler;
    const names=new Map(),qualified=p=>{let value=names.get(p);if(value===undefined){value=name+'.'+p;if(names.size<128)names.set(p,value)}return value};
    handler={
      get(t,p,r){if(typeof p==='string'&&!p.startsWith('_')&&(p!=='then'||Reflect.has(t,p))){const key=qualified(p);recordAPIAccess(key,Reflect.has(t,p)||targetAbsentProperties.has(key))}return Reflect.get(t,p,r)},
      set(t,p,v,r){if(typeof p==='string'&&!p.startsWith('_')){const key=qualified(p);recordAPIAccess(key,Reflect.has(t,p)||targetAbsentProperties.has(key))}return Reflect.set(t,p,v,r)}
    };
    observationHandlers.set(name,handler);return handler;
  };
  const observe=(name,target)=>{const proxy=new Proxy(target,observationHandler(name));if(elementData.has(target))elementData.set(proxy,elementData.get(target));if(scriptStates.has(target))scriptStates.set(proxy,scriptStates.get(target));if(elementHandlers.has(target))elementHandlers.set(proxy,elementHandlers.get(target));if(storageAreas.has(target))storageAreas.set(proxy,storageAreas.get(target));if(rtcPeerStates.has(target))rtcPeerStates.set(proxy,rtcPeerStates.get(target));if(rtcPeerPrivate.has(target))rtcPeerPrivate.set(proxy,rtcPeerPrivate.get(target));if(rtcDataStates.has(target))rtcDataStates.set(proxy,rtcDataStates.get(target));if(permissionsPolicySlots.has(target))permissionsPolicySlots.set(proxy,permissionsPolicySlots.get(target));if(trustedFactorySlots.has(target))trustedFactorySlots.set(proxy,trustedFactorySlots.get(target));return proxy};
  const storageTarget=Object.create(Storage.prototype),sessionStorageTarget=Object.create(Storage.prototype);storageAreas.set(storageTarget,'local');storageAreas.set(sessionStorageTarget,'session');
  const nav=observe('Navigator',Object.create(Navigator.prototype)),scr=observe('Screen',Object.create(Screen.prototype)),loc=observe('Location',Object.create(Location.prototype)),histTarget=Object.create(History.prototype),hist=observe('History',histTarget),storage=observe('Storage',storageTarget),sessionStorage=observe('Storage',sessionStorageTarget),crypto=observe('Crypto',Object.create(Crypto.prototype)),gpu=observe('GPU',Object.create(GPU.prototype)),trustedTypes=observe('TrustedTypePolicyFactory',new TrustedTypePolicyFactory(hostToken)),documentPolicy=observe('PermissionsPolicy',new PermissionsPolicy(hostToken,host.permissionsPolicy()));
  historySlots.set(histTarget,{state:null,scrollRestoration:'auto'});historySlots.set(hist,historySlots.get(histTarget));
  for(const k of ['userAgent','appVersion','platform','languages','language','hardwareConcurrency','deviceMemory','onLine','cookieEnabled','vendor','product','appName','maxTouchPoints','webdriver','pdfViewerEnabled'])def(Navigator.prototype,k,{get:()=>host.navigator()[k]});
  let uaData;def(Navigator.prototype,'userAgentData',{get:()=>uaData||(uaData=new NavigatorUAData(hostToken,host.navigator()))});
  const pdfPluginNames=['PDF Viewer','Chrome PDF Viewer','Chromium PDF Viewer','Microsoft Edge PDF Viewer','WebKit built-in PDF'];
  const pdfPlugins=pdfPluginNames.map(name=>new Plugin(hostToken,name));
  const pdfMimeTypes=[new MimeType(hostToken,'application/pdf','pdf','Portable Document Format'),new MimeType(hostToken,'text/pdf','pdf','Portable Document Format')];
  for(const plugin of pdfPlugins){plugin.__mimes=pdfMimeTypes;for(const [index,mime] of pdfMimeTypes.entries()){Object.defineProperty(plugin,index,{value:mime,enumerable:true});Object.defineProperty(plugin,mime.type,{value:mime,enumerable:false})}}
  for(const mime of pdfMimeTypes)mime.__plugin=pdfPlugins[0];
  const pluginArray=new PluginArray(hostToken,pdfPlugins),mimeTypeArray=new MimeTypeArray(hostToken,pdfMimeTypes);
  def(Navigator.prototype,'plugins',{get:()=>pluginArray});def(Navigator.prototype,'mimeTypes',{get:()=>mimeTypeArray});
  def(Navigator.prototype,'gpu',{get:()=>gpu});
  for(const k of ['width','height','availWidth','availHeight','colorDepth','pixelDepth'])def(Screen.prototype,k,{get:()=>host.screen()[k]});
  let locationAncestors;
  bootstrapRestoreHooks.push(()=>{locationAncestors=undefined});
  registerRealmBinding(loc,'Location',{
    get:key=>key==='ancestorOrigins'?(locationAncestors??=createDOMStringList(host.locationAncestorOrigins())):host.locationPart(key),
    set:(key,value)=>host.setLocationPart(key,value),assign:value=>host.navigate(value),replace:value=>host.navigate(value,true),reload:()=>host.navigate(host.location(),true,true)
  });
  Object.defineProperty(loc,'valueOf',{value:Object.prototype.valueOf});
  for(const key of ['ancestorOrigins','href','origin','protocol','host','hostname','port','pathname','search','hash']){
    const get=Object.getOwnPropertyDescriptor({get [key](){const binding=requireRealmBinding(this,'Location');return callRealmBinding(this,binding,'get',[key])}},key).get;
    const set=['origin','ancestorOrigins'].includes(key)?undefined:Object.getOwnPropertyDescriptor({set [key](value){const binding=requireRealmBinding(this,'Location');value=bindingString(value);callRealmBinding(this,binding,'set',[key,value])}},key).set;
    markNative(get,key,'get ');markNative(set,key,'set ');Object.defineProperty(loc,key,{get,set,enumerable:true});
  }
  for(const key of ['assign','reload','replace','toString']){
    const value=Location.prototype[key];markNative(value,key);Object.defineProperty(loc,key,{value,enumerable:true});delete Location.prototype[key];
  }
  Object.defineProperty(loc,Symbol.toPrimitive,{value:undefined});

  const consoleString=String,consoleParseInt=parseInt,consoleParseFloat=parseFloat;
  const consoleArguments=args=>{
    if(args.length>1&&typeof args[0]==='string'){
      const format=args[0];let argument=1;
      for(let index=0;index<format.length-1&&argument<args.length;index++){
        if(format[index]!=='%')continue;
        const specifier=format[++index];
        if(specifier!=='s'&&specifier!=='d'&&specifier!=='i'&&specifier!=='f'&&specifier!=='o'&&specifier!=='O'&&specifier!=='c')continue;
        const value=args[argument];
        if(specifier==='s')args[argument]=consoleString(value);
        else if(specifier==='d'||specifier==='i'||specifier==='f')args[argument]=typeof value==='symbol'?NaN:(specifier==='f'?consoleParseFloat(value):consoleParseInt(value));
        argument++;
      }
    }
    // Trace diagnostics are strings, not remote-object inspection handles.
    // Never traverse or stringify application objects merely to log them.
    for(let index=0;index<args.length;index++){const value=args[index];args[index]=value!==null&&typeof value==='object'?'[object]':typeof value==='function'?'[function]':consoleString(value)}
    return args;
  };
  class Console { group(...a){host.console('startGroup',a.length?consoleArguments(a):['console.group'])} groupCollapsed(...a){host.console('startGroupCollapsed',a.length?consoleArguments(a):['console.groupCollapsed'])} groupEnd(...a){host.console('endGroup',a.length?consoleArguments(a):['console.groupEnd'])} debug(...a){host.console('debug',consoleArguments(a))} log(...a){host.console('log',consoleArguments(a))} info(...a){host.console('info',consoleArguments(a))} warn(...a){host.console('warn',consoleArguments(a))} error(...a){host.console('error',consoleArguments(a))} }
  const consoleCounts=new Map();
  const consoleCountGet=Function.prototype.call.bind(Map.prototype.get),consoleCountSet=Function.prototype.call.bind(Map.prototype.set),consoleCountDelete=Function.prototype.call.bind(Map.prototype.delete);
  const consoleCount=(label,reset)=>{
    let key='default',failure,failed=false;
    try{if(label!==undefined){if(typeof label==='symbol')throw new TypeError('Cannot convert a Symbol value to a string');key=consoleString(label)}}catch(error){failure=error;failed=true}
    // Chrome applies the operation to the default label even if conversion
    // fails, then propagates the original exception.
    if(reset){if(!consoleCountDelete(consoleCounts,key))host.console('warning',["Count for '"+key+"' does not exist"])}
    else{const count=(consoleCountGet(consoleCounts,key)||0)+1;consoleCountSet(consoleCounts,key,count);host.console('count',[key+': '+count])}
    if(failed)throw failure;
  };
  Object.defineProperties(Console.prototype,{
    count:{value:function count(label=undefined){consoleCount(label,false)},writable:true,configurable:true},
    countReset:{value:function countReset(label=undefined){consoleCount(label,true)},writable:true,configurable:true}
  });
  const document=observe('Document',new HTMLDocument(hostToken));
  Object.defineProperty(document,'location',{get:()=>loc,enumerable:true,configurable:false});
  let frameElementCache;
  const frameElement=()=>{const result=host.frameElement();return result?unwrapCrossRealm(result.frame,result):null};
  const remoteWindowCache=new Map(),remoteDocumentCache=new Map(),crossRealmCache=new Map(),crossRealmReferences=new WeakMap();
  let bridgeRealmID=host.selfRealmID();
  const bridgeApply=Reflect.apply;
  const bridgeHasOwn=Object.prototype.hasOwnProperty;
  const bridgeWeakGet=WeakMap.prototype.get,bridgeWeakSet=WeakMap.prototype.set,bridgeWeakHas=WeakMap.prototype.has;
  const referenceGet=value=>bridgeApply(bridgeWeakGet,crossRealmReferences,[value]),referenceSet=(value,record)=>bridgeApply(bridgeWeakSet,crossRealmReferences,[value,record]),referenceHas=value=>bridgeApply(bridgeWeakHas,crossRealmReferences,[value]);
  // Only native ArrayIterator.next creates a result with no preexisting user
  // aliases. Its primitive own fields remain valid until mutation or escape.
  // Keep identity/prototypes on the ordinary reference bridge; never prefetch
  // array elements or cache user iterator results.
  const freshIteratorFields=new WeakMap();
  const invalidateIteratorFields=value=>{const fields=bridgeApply(bridgeWeakGet,freshIteratorFields,[value]);if(fields)fields.valid=false};
  // Some engines reject even identical nonconfigurable accessor descriptors
  // returned by a Proxy. Detect the invariant once using bootstrap intrinsics.
  const crossRealmAccessorDescriptors=(()=>{
    const describe=Reflect.getOwnPropertyDescriptor,define=Object.defineProperty,getter=()=>0,target={};
    define(target,'value',{get:getter,configurable:false});
    try{const descriptor=describe(new Proxy(target,{getOwnPropertyDescriptor:describe}),'value');return descriptor.get===getter&&!descriptor.configurable}catch{return false}
  })();
  const crossRealmSymbols=new Map(),crossRealmSymbolReferences=new Map();
  const localCrossRealmSymbols=new Map();let nextCrossRealmSymbolID=0;
  const wellKnownSymbols=new Map(Reflect.ownKeys(Symbol).filter(name=>typeof name==='string'&&typeof Symbol[name]==='symbol').map(name=>[Symbol[name],name]));
  const encodeCrossRealmKey=key=>{
    if(typeof key!=='symbol')return{kind:'string',value:String(key)};
    const wellKnown=wellKnownSymbols.get(key);if(wellKnown)return{kind:'symbol',wellKnown};
    const global=Symbol.keyFor(key);if(global!==undefined)return{kind:'symbol',global};
    if(!crossRealmSymbolReferences.has(key)){
      const localID=++nextCrossRealmSymbolID;localCrossRealmSymbols.set(localID,key);
      crossRealmSymbolReferences.set(key,{kind:'symbol',sourceRealm:hostToken,localID,description:key.description});
    }
    return crossRealmSymbolReferences.get(key);
  };
  const decodeCrossRealmKey=key=>{
    if(key.kind==='string')return key.value;
    if(key.realm===host.selfRealmID())return host.frameResolve(key.handle,key.realm);
    if(key.wellKnown!==undefined)return Symbol[key.wellKnown];
    if(key.global!==undefined)return Symbol.for(key.global);
    if(key.sourceRealm===hostToken&&localCrossRealmSymbols.has(key.localID))return localCrossRealmSymbols.get(key.localID);
    const identity=key.localID===undefined?key.realm+':'+key.handle:key.sourceRealm+':local:'+key.localID;
    if(!crossRealmSymbols.has(identity)){const value=Symbol(key.description);crossRealmSymbols.set(identity,value);crossRealmSymbolReferences.set(value,key)}
    return crossRealmSymbols.get(identity);
  };
  const encodeCrossRealmArgument=value=>{
    if(value===undefined)return{kind:'undefined'};
    if(typeof value==='symbol')return{kind:'symbol',key:encodeCrossRealmKey(value)};
    if(typeof value==='bigint')return{kind:'bigint',value:String(value)};
    if((typeof value==='object'&&value!==null)||typeof value==='function'||typeof value==='undefined'){
      const reference=referenceGet(value);if(reference){invalidateIteratorFields(value);return{kind:'reference',...reference}}
      const local=host.frameReference(value);
      return{kind:'reference',frame:local.frame,realm:local.realm,handle:local.handle,type:local.__mimicCrossRealm,array:local.array,constructable:local.constructable};
    }
    return{kind:'value',value};
  };
  // Documents are canonical realm-owned objects. Only WindowProxy follows the
  // current document after navigation; saved Document references keep their owner.
  const bridgeAccess=(id,realm)=>{if(!host.frameCanAccess(id,realm))throw new DOMException('Blocked cross-origin frame access','SecurityError')};
  const remoteDocument=(id,nullable=false)=>{if(nullable&&!host.frameCanAccess(id))return null;bridgeAccess(id);return unwrapCrossRealm(id,host.frameGlobalGet(id,{kind:'string',value:'document'}))};
  const unwrapCrossRealm=(id,result)=>{
    if(result&&result.frame!==undefined)id=result.frame;
    const kind=result&&result.__mimicCrossRealm;
    if(kind==='undefined')return undefined;if(kind==='null')return null;if(kind==='value')return result.value;
    if(kind==='symbol')return decodeCrossRealmKey(result);
    if(kind==='bigint')return BigInt(result.value);
    if(kind==='special-number')return result.value==='NaN'?NaN:result.value==='-0'?-0:result.value==='Infinity'?Infinity:-Infinity;
    if(kind==='window')return remoteWindow(result.frame);
    if(kind!=='object'&&kind!=='function'&&kind!=='undetectable')return result;
    if(result.realm===bridgeRealmID)return host.frameResolve(result.handle,result.realm);
    const key=id+':'+String(result.realm||'')+':'+result.handle;if(crossRealmCache.has(key))return crossRealmCache.get(key);
    const iteratorFields=result.iteratorResult?{valid:true,fields:result.iteratorResult}:null;
    // Bound callables have no nonconfigurable prototype/caller/arguments of
    // their own. Remote descriptors supply those properties when they exist.
    const target=kind==='function'?(result.constructable?(function(){}).bind(null):()=>{}):result.array?[]:Object.create(null);
    for(const name of Reflect.ownKeys(target))if(Object.getOwnPropertyDescriptor(target,name).configurable)delete target[name];
    const unsupported=operation=>{host.semanticMissingAt('surface.js:513','CrossRealm.'+operation);throw new DOMException('Cross-realm '+operation+' is not implemented','NotSupportedError')};
    const traps={
      get(_target,property){bridgeAccess(id,result.realm);if(iteratorFields?.valid&&(property==='done'||property==='value')&&bridgeApply(bridgeHasOwn,iteratorFields.fields,[property]))return unwrapCrossRealm(id,iteratorFields.fields[property]);return unwrapCrossRealm(id,host.frameGet(id,result.handle,encodeCrossRealmKey(property),result.realm))},
      set(_target,property,value){if(iteratorFields)iteratorFields.valid=false;bridgeAccess(id,result.realm);return host.frameSet(id,result.handle,encodeCrossRealmKey(property),encodeCrossRealmArgument(value),result.realm)},
      has(_target,property){bridgeAccess(id,result.realm);return host.frameHas(id,result.handle,encodeCrossRealmKey(property),result.realm)},
      apply(_target,receiver,args){
        bridgeAccess(id,result.realm);
        if(result.eval){if(!host.frameEvalAllowed(id))return undefined;const value=args[0],source=typeof value==='string'?value:evalSourceResolver(value);if(source===undefined){host.frameEval(id,undefined,result.realm);return value}return unwrapCrossRealm(id,host.frameEval(id,source,result.realm))}
        return unwrapCrossRealm(id,host.frameCall(id,result.handle,args.map(encodeCrossRealmArgument),encodeCrossRealmArgument(receiver),result.realm,!!result.iteratorNext));
      },
      construct(_target,args,newTarget){
        bridgeAccess(id,result.realm);
        const reference=referenceGet(newTarget);
        if(!reference||reference.frame!==id||reference.realm!==result.realm)return unsupported('constructNewTarget');
        const encoded=args.map(encodeCrossRealmArgument);
        const outcome=host.frameConstruct(id,result.handle,encoded,encodeCrossRealmArgument(newTarget),result.realm),value=unwrapCrossRealm(id,outcome.value);
        if(outcome.threw)throw value;
        return value;
      },
      defineProperty(_target,property,descriptor){
        if(iteratorFields)iteratorFields.valid=false;bridgeAccess(id,result.realm);
        const outcome=host.frameMutateProperty(id,result.handle,'mutateDefine',encodeCrossRealmKey(property),encodeCrossRealmArgument(descriptor),result.realm),value=unwrapCrossRealm(id,outcome.value);
        if(outcome.threw)throw value;
        // Materialize only the descriptor required by the local Proxy invariant.
        // The owner remains authoritative for configurable properties.
        if(value&&(descriptor.configurable===false||descriptor.writable===false))traps.getOwnPropertyDescriptor(target,property);
        return value;
      },
      deleteProperty(_target,property){
        if(iteratorFields)iteratorFields.valid=false;bridgeAccess(id,result.realm);
        const outcome=host.frameMutateProperty(id,result.handle,'mutateDelete',encodeCrossRealmKey(property),encodeCrossRealmArgument(undefined),result.realm),value=unwrapCrossRealm(id,outcome.value);
        if(outcome.threw)throw value;return value;
      },
      preventExtensions(){if(iteratorFields)iteratorFields.valid=false;if(result.binding?.unpreventable){bridgeAccess(id,result.realm);return false}return unsupported('preventExtensions')},
      setPrototypeOf(){if(iteratorFields)iteratorFields.valid=false;return unsupported('setPrototypeOf')},
      getPrototypeOf(){bridgeAccess(id,result.realm);return unwrapCrossRealm(id,host.framePrototype(id,result.handle,result.realm))},
      ownKeys(){bridgeAccess(id,result.realm);return host.frameOwnKeys(id,result.handle,result.realm).map(decodeCrossRealmKey)},
      getOwnPropertyDescriptor(_target,property){
        bridgeAccess(id,result.realm);
        const raw=host.frameDescriptor(id,result.handle,encodeCrossRealmKey(property),result.realm);
        if(!raw||!raw.exists)return undefined;
        if(raw.accessor&&!raw.configurable&&!crossRealmAccessorDescriptors)return unsupported('nonconfigurableAccessorDescriptor');
        const descriptor={enumerable:!!raw.enumerable,configurable:!!raw.configurable};
        if(raw.accessor){descriptor.get=unwrapCrossRealm(id,raw.get);descriptor.set=unwrapCrossRealm(id,raw.set)}
        else{descriptor.value=unwrapCrossRealm(id,raw.value);descriptor.writable=!!raw.writable}
        // Reporting a remote nonconfigurable property requires the same
        // descriptor on the local proxy target (ECMAScript proxy invariants).
        if(!descriptor.configurable)Object.defineProperty(target,property,descriptor);
        return descriptor;
      }
    };
    // A Proxy cannot preserve [[IsHTMLDDA]]. Import an engine-native
    // undetectable object while retaining the original realm's identity.
    const proxy=kind==='undetectable'?host.createUndetectable({
      __proto__:null,
      call(args,construct,receiver){if(construct)throw new TypeError('Illegal constructor');return traps.apply(target,receiver,args)},
      get(property){return traps.has(target,property)?[true,traps.get(target,property)]:[false]},
      set(property,value){return[true,traps.set(target,property,value)]},
      deleteProperty(property){return[true,traps.deleteProperty(target,property)]},
      defineProperty(property,descriptor){return[true,traps.defineProperty(target,property,descriptor)]},
      getOwnPropertyDescriptor(property){const descriptor=traps.getOwnPropertyDescriptor(target,property);return descriptor===undefined?[false]:[true,descriptor]},
      ownKeys(){return traps.ownKeys()}
    }):new Proxy(target,traps);
    if(iteratorFields)bridgeApply(bridgeWeakSet,freshIteratorFields,[proxy,iteratorFields]);
    if(result.eval)markNative(proxy,'eval');
    if(result.nodeId){const data=host.nodeData(result.nodeId);if(data){elementData.set(proxy,data);elementWrappers.set(String(result.nodeId),proxy)}}
    referenceSet(proxy,{frame:id,realm:result.realm,handle:result.handle,type:kind,array:result.array,constructable:result.constructable,nodeId:result.nodeId,document:result.document,eval:result.eval,binding:result.binding});crossRealmCache.set(key,proxy);
    if(result.document){remoteDocumentCache.set(result.realm,proxy);if(result.nodeId)documentWrappers.set(result.nodeId,proxy)}
    // Native undetectable objects cannot intercept [[GetPrototypeOf]]. This
    // preserves the initial remote prototype identity, but later replacement
    // of the owner's prototype is not reflected by this bridge wrapper.
    if(kind==='undetectable')Object.setPrototypeOf(proxy,traps.getPrototypeOf());
    return proxy;
  };
  const bridgeOriginalEval=eval;let bridgeOriginalPostMessage;
  registerBootstrapCallback('installFrameReferenceBridge',
    encoded=>unwrapCrossRealm(encoded.frame,encoded),
    value=>{
      if(value===null||value===undefined||(typeof value!=='object'&&typeof value!=='function'&&typeof value!=='undefined'))return null;
      const reference=referenceGet(value);if(reference){invalidateIteratorFields(value);return{...reference,__mimicCrossRealm:reference.type}}
      return null;
    },(value,importNode=false)=>importNode?wrap(host.nodeData(value)):value===document?host.documentRootID():bridgeApply(bridgeWeakGet,elementData,[value])?.nodeId||0,
    key=>{const value=Reflect.get(globalThis,key);return{value,intrinsic:key==='eval'&&value===bridgeOriginalEval||key==='postMessage'&&value===bridgeOriginalPostMessage}},
    value=>{const binding=bindingGet(value);return binding?{kind:binding.kind,invoke:binding.invoke,unpreventable:binding.unpreventable}:null});
  const remoteWindow=id=>{
    if(id===host.selfFrameID())return globalThis;
    if(remoteWindowCache.has(id))return remoteWindowCache.get(id);
    host.retainWindowReference(id);
    let proxy;
    const framePost=new Proxy(function postMessage(){},{apply(_target,_this,args){return host.framePost(id,args[0],args[1]===undefined?'/':String(args[1]),takeMessagePorts(args[2]))}});markNative(framePost,'postMessage');
    const target={postMessage:framePost};Object.defineProperty(target,Symbol.toStringTag,{value:'Window',configurable:true});
    const crossKeys=['window','self','location','closed','frames','length','top','opener','parent','blur','close','focus','postMessage','then',Symbol.toStringTag,Symbol.hasInstance,Symbol.isConcatSpreadable];
    const read=p=>{
      if(p==='window'||p==='self')return proxy;
      if(p==='document')return remoteDocument(id);
      if(p==='parent'||p==='top')return remoteWindow(host.frameRelation(id,p));
      const accessible=host.frameCanAccess(id);
      if(!accessible){
        if(p==='then'||p===Symbol.toStringTag||p===Symbol.hasInstance||p===Symbol.isConcatSpreadable)return undefined;
        if(p==='location')return Object.freeze({get href(){bridgeAccess(id);return host.frameLocation(id)},toString(){return this.href}});
      }
      if(p!=='postMessage'&&!accessible)throw new DOMException('Blocked cross-origin frame access','SecurityError');
      const encoded=host.frameGlobalGet(id,encodeCrossRealmKey(p));
      if(encoded.intrinsic&&p==='postMessage')return framePost;
      const value=unwrapCrossRealm(id,encoded);
      if(typeof p==='string')recordAPIAccess('WindowProxy.'+p,value!==undefined);
      return value;
    };
    const describe=p=>{
      if(!host.frameCanAccess(id)){
        if(p==='then'||p===Symbol.toStringTag||p===Symbol.hasInstance||p===Symbol.isConcatSpreadable)return{value:undefined,writable:false,enumerable:false,configurable:true};
        bridgeAccess(id);
      }
      const raw=host.frameGlobalReflect(id,'descriptor',encodeCrossRealmKey(p));
      if(!raw||!raw.exists)return undefined;
      const descriptor={enumerable:!!raw.enumerable,configurable:!!raw.configurable};
      if(raw.accessor){descriptor.get=unwrapCrossRealm(id,raw.get);descriptor.set=unwrapCrossRealm(id,raw.set)}
      else{descriptor.value=unwrapCrossRealm(id,raw.value);descriptor.writable=!!raw.writable}
      return descriptor;
    };
    const keys=()=>host.frameCanAccess(id)?host.frameGlobalReflect(id,'keys').map(decodeCrossRealmKey):crossKeys.slice();
    const define=(p,d)=>{bridgeAccess(id);const raw={};for(const k of ['enumerable','configurable','writable'])if(k in d)raw[k]=d[k];for(const k of ['get','set','value'])if(k in d)raw[k]=encodeCrossRealmArgument(d[k]);return host.frameGlobalReflect(id,'define',encodeCrossRealmKey(p),raw)};
    const remove=p=>{bridgeAccess(id);return host.frameGlobalReflect(id,'delete',encodeCrossRealmKey(p))};
    const prototype=()=>host.frameCanAccess(id)?unwrapCrossRealm(id,host.frameGlobalReflect(id,'prototype')):null;
    const write=(p,v)=>{bridgeAccess(id);return unwrapCrossRealm(id,host.frameGlobalSet(id,encodeCrossRealmKey(p),encodeCrossRealmArgument(v)))};
    if(host.createWindowObject){
      // A native exotic object can replace nonconfigurable descriptors after
      // navigation. A JS Proxy target cannot do this without invariant errors.
      const native=host.createWindowObject({__proto__:null,get:p=>[true,read(p)],set:(p,v)=>[true,write(p,v)],getOwnPropertyDescriptor:p=>{const d=describe(p);return d===undefined?[false]:[true,d]},ownKeys:keys,defineProperty:(p,d)=>[true,define(p,d)],deleteProperty:p=>[true,remove(p)]});
      proxy=new Proxy(native,{getPrototypeOf:prototype,preventExtensions:()=>false,setPrototypeOf:(_t,next)=>next===prototype(),has:(_t,p)=>{if(!host.frameCanAccess(id)){if(crossKeys.includes(p))return true;bridgeAccess(id)}return unwrapCrossRealm(id,host.frameGlobalHas(id,encodeCrossRealmKey(p)))}});
    }else{
      proxy=new Proxy(target,{get:(_t,p)=>read(p),set:(_t,p,v)=>write(p,v),has:(_t,p)=>{bridgeAccess(id);return unwrapCrossRealm(id,host.frameGlobalHas(id,encodeCrossRealmKey(p)))},ownKeys:keys,getPrototypeOf:prototype,preventExtensions:()=>false,setPrototypeOf:(_t,next)=>next===prototype(),getOwnPropertyDescriptor:(_t,p)=>{const d=describe(p);if(d&&!d.configurable)Object.defineProperty(target,p,d);return d},defineProperty:(_t,p,d)=>define(p,d),deleteProperty:(_t,p)=>remove(p)});
    }
    referenceSet(proxy,{frame:id,type:'window'});remoteWindowCache.set(id,proxy);return proxy;
  };
  const xhrSlots=new WeakMap(),xhrState=xhr=>xhrSlots.get(xhr);
  const fireXHREvent=(xhr,type)=>dispatchTrusted(xhr,new Event(type)),setXHRState=(xhr,value)=>{xhrState(xhr).readyState=value;fireXHREvent(xhr,'readystatechange')};
  // Keep the WebIDL intermediate interface in the implementation's constructor
  // chain too. Its public constructor stays illegal; only XHR initializes it.
  const xhrConstructionToken = {};
  class XMLHttpRequestEventTarget extends EventTarget { constructor(token){if(token!==xhrConstructionToken)throw new TypeError('Illegal constructor');super()} }
  class XMLHttpRequest extends XMLHttpRequestEventTarget { constructor(){super(xhrConstructionToken);xhrSlots.set(this,{readyState:0,status:0,statusText:'',responseText:'',responseURL:'',responseType:'',timeout:0,withCredentials:false,onreadystatechange:null,onload:null,onerror:null,ontimeout:null,onloadend:null,requestHeaders:{},requestHeaderOrder:[],responseHeaders:{},sent:false,method:'GET',url:'',async:true})} open(method,url,async=true,user,password){method=String(method).toUpperCase();if(!method||/[^A-Z-]/.test(method))throw new DOMException('Invalid HTTP method','SyntaxError');const state=xhrState(this);state.method=method;state.url=String(url);state.async=Boolean(async);state.sent=false;state.requestHeaders={};state.requestHeaderOrder=[];state.status=0;state.responseText='';state.responseURL='';setXHRState(this,1)} setRequestHeader(name,value){const state=xhrState(this);if(state.readyState!==1||state.sent)throw new DOMException("The object's state must be OPENED.",'InvalidStateError');name=String(name).trim().toLowerCase();value=String(value).trim();if(!name||/[^!#$%&'*+.^_`|~0-9a-z-]/i.test(name)||/[\0\r\n]/.test(value))throw new DOMException('Invalid HTTP header','SyntaxError');if(state.requestHeaders[name]===undefined)state.requestHeaderOrder.push(name);state.requestHeaders[name]=state.requestHeaders[name]?state.requestHeaders[name]+', '+value:value} getResponseHeader(name){const state=xhrState(this);if(state.readyState<2)return null;const value=state.responseHeaders[String(name).toLowerCase()];return value===undefined?null:value} getAllResponseHeaders(){const state=xhrState(this);if(state.readyState<2)return'';return Object.keys(state.responseHeaders).sort().map(k=>k+': '+state.responseHeaders[k]+'\r\n').join('')} send(body=null){const state=xhrState(this);if(state.readyState!==1||state.sent)throw new DOMException("The object's state must be OPENED.",'InvalidStateError');state.sent=true;host.xhr(result=>{if(result.error){state.status=0;setXHRState(this,4);fireXHREvent(this,result.error);fireXHREvent(this,'loadend');return}state.status=result.status;state.statusText=result.statusText;state.responseURL=result.responseURL;state.responseHeaders=result.responseHeaders||{};setXHRState(this,2);setXHRState(this,3);state.responseText=result.responseText||'';setXHRState(this,4);fireXHREvent(this,'load');fireXHREvent(this,'loadend')},state.method,state.url,state.requestHeaders,body==null?'':String(body),Number(state.timeout)||0,state.requestHeaderOrder,Boolean(state.withCredentials))} abort(){const state=xhrState(this);state.sent=false;state.status=0;state.responseText='';setXHRState(this,0)} }
  for(const key of ['readyState','status','statusText','responseText','responseURL','responseType','timeout','withCredentials','onreadystatechange','onload','onerror','ontimeout','onloadend'])def(XMLHttpRequest.prototype,key,{get(){return xhrState(this)[key]},set(value){xhrState(this)[key]=value}});
  const requestSlots=new WeakMap(),responseSlots=new WeakMap();
  class Request { constructor(input,init={}){const previous=input instanceof Request?requestSlots.get(input):null,method=String(init.method||previous?.method||'GET').toUpperCase(),raw=input instanceof Request?previous.url:String(input),url=host.urlParts(raw).href;requestSlots.set(this,{url,method,headers:new Headers(init.headers||previous?.headers),mode:String(init.mode||previous?.mode||'cors'),credentials:String(init.credentials||previous?.credentials||'same-origin'),cache:String(init.cache||previous?.cache||'default'),redirect:String(init.redirect||previous?.redirect||'follow'),referrer:init.referrer===undefined?(previous?.referrer||'about:client'):String(init.referrer),body:init.body??null,bodyUsed:false})} }
  for(const key of ['url','method','headers','mode','credentials','cache','redirect','referrer','body','bodyUsed'])def(Request.prototype,key,{get(){const state=requestSlots.get(this);if(!state)throw new TypeError('Illegal invocation');return state[key]}});
  Object.defineProperty(Request.prototype,Symbol.toStringTag,{value:'Request',configurable:true});
  class Response { constructor(body=null,init={}){const status=init.status===undefined?200:Number(init.status);if(status!==0&&(status<200||status>599))throw new RangeError('Invalid status code');responseSlots.set(this,{body,headers:new Headers(init.headers),status,statusText:String(init.statusText||''),type:'default',url:'',redirected:false,bodyUsed:false})} get ok(){const status=responseSlots.get(this).status;return status>=200&&status<=299} text(){const state=responseSlots.get(this);state.bodyUsed=true;return Promise.resolve(state.body==null?'':String(state.body))} json(){return this.text().then(JSON.parse)} }
  for(const key of ['body','headers','status','statusText','type','url','redirected','bodyUsed'])def(Response.prototype,key,{get(){const state=responseSlots.get(this);if(!state)throw new TypeError('Illegal invocation');return state[key]}});
  Object.defineProperty(Response.prototype,Symbol.toStringTag,{value:'Response',configurable:true});
  // Legacy named constructors are distinct functions with the element
  // interface prototype, rather than aliases whose function name mutates the
  // corresponding HTML*Element constructor.
  globalThis.Image=function Image(width,height){const element=document.createElement('img');if(width!==undefined)element.width=Number(width);if(height!==undefined)element.height=Number(height);return element};
  globalThis.Audio=function Audio(src){const element=document.createElement('audio');if(src!==undefined)element.src=String(src);return element};
  globalThis.Option=function Option(text='',value,defaultSelected=false,selected=false){const element=document.createElement('option');element.text=String(text);if(value!==undefined)element.value=String(value);element.defaultSelected=Boolean(defaultSelected);element.selected=Boolean(selected);return element};
  const window=globalThis;listenersFor(window);window.addEventListener=(...a)=>EventTarget.prototype.addEventListener.apply(window,a);window.removeEventListener=(...a)=>EventTarget.prototype.removeEventListener.apply(window,a);window.dispatchEvent=(...a)=>EventTarget.prototype.dispatchEvent.apply(window,a);window.onmessage=null;window.onerror=null;const NodeFilter=Object.freeze({FILTER_ACCEPT:1,FILTER_REJECT:2,FILTER_SKIP:3,SHOW_ALL:0xffffffff,SHOW_ELEMENT:1,SHOW_ATTRIBUTE:2,SHOW_TEXT:4,SHOW_CDATA_SECTION:8,SHOW_ENTITY_REFERENCE:16,SHOW_ENTITY:32,SHOW_PROCESSING_INSTRUCTION:64,SHOW_COMMENT:128,SHOW_DOCUMENT:256,SHOW_DOCUMENT_TYPE:512,SHOW_DOCUMENT_FRAGMENT:1024,SHOW_NOTATION:2048});Object.assign(window,{window:null,self:null,top:null,parent:null,document,navigator:nav,screen:scr,location:loc,history:hist,localStorage:storage,sessionStorage,crypto,trustedTypes,console:new Console(),atob,btoa,TextEncoder,Headers,Request,Response,Event,MessageEvent,ErrorEvent,EventTarget,Node,DocumentFragment,ShadowRoot,Element,HTMLElement,SVGElement,HTMLScriptElement,HTMLImageElement,HTMLIFrameElement,HTMLAnchorElement,HTMLCollection,NodeList,DOMTokenList,CSSStyleDeclaration,Crypto,DOMException,PermissionsPolicy,FeaturePolicy,TrustedHTML,TrustedScript,TrustedScriptURL,TrustedTypePolicy,TrustedTypePolicyFactory,URL,URLSearchParams,ReadableStream,ReadableStreamDefaultReader,ReadableStreamDefaultController,WritableStream,WritableStreamDefaultWriter,TransformStream,TransformStreamDefaultController,Blob,File,Worker,Document,HTMLDocument,Navigator,NavigatorUAData,Screen,Location,History,Storage,XMLHttpRequestEventTarget,XMLHttpRequest,GPU,GPUAdapter,GPUAdapterInfo,GPUSupportedFeatures,GPUSupportedLimits,GPUDevice,RTCPeerConnection,RTCSessionDescription,RTCIceCandidate,PerformanceEntry,PerformanceServerTiming,PerformanceResourceTiming,PerformanceNavigationTiming,NodeFilter});let security=host.documentSecurity();for(const [name,key] of Object.entries({isSecureContext:'secureContext',crossOriginIsolated:'crossOriginIsolated',credentialless:'credentialless',originAgentCluster:'originAgentCluster'}))Object.defineProperty(window,name,{get:()=>security[key],enumerable:true,configurable:true});const relations=host.windowRelations();window.window=window;window.self=window;let windowTop=relations.top===relations.self?window:remoteWindow(relations.top),windowParent=relations.parent===relations.self?window:remoteWindow(relations.parent);
  // Top stays an unforgeable accessor after exposure normalization. Parent is
  // replaceable: assignment creates an own data property, as in Chrome.
  const getWindowTop=()=>windowTop,getWindowParent=()=>windowParent;
  const setWindowParent=({set(value){Object.defineProperty(this,'parent',{value,writable:true,enumerable:true,configurable:true})}}).set;
  for(const [fn,name,prefix] of [[getWindowTop,'top','get '],[getWindowParent,'parent','get '],[setWindowParent,'parent','set ']]){
    Object.defineProperty(fn,'name',{value:prefix+name,configurable:true});markNative(fn,name,prefix);
  }
  Object.defineProperties(window,{top:{get:getWindowTop,enumerable:true,configurable:true},parent:{get:getWindowParent,set:setWindowParent,enumerable:true,configurable:true}});
  const originWindowID=value=>{
    if(value===window)return '';
    for(const [id,proxy] of remoteWindowCache)if(value===proxy){bridgeAccess(id);return id}
    throw new TypeError('Illegal invocation');
  };
  const originDescriptor=Object.getOwnPropertyDescriptor({
    get origin(){return host.windowOrigin(originWindowID(this))},
    set origin(value){originWindowID(this);Object.defineProperty(this,'origin',{value,writable:true,enumerable:true,configurable:true})}
  },'origin');
  markNative(originDescriptor.get,'origin','get ');markNative(originDescriptor.set,'origin','set ');
  Object.defineProperty(window,'origin',{...originDescriptor,enumerable:true,configurable:true});
  window.__receiveFrameMessage=(data,origin,sourceId)=>dispatchTrusted(window,new MessageEvent('message',{data,origin,source:remoteWindow(sourceId)}));globalThis.__receiveFrameMessage=window.__receiveFrameMessage;
  Object.assign(window,{CharacterData,Text,Comment,SVGSVGElement,SVGRect,HTMLLinkElement,HTMLMetaElement,IntersectionObserver,BroadcastChannel,CryptoKey,SubtleCrypto,PerformanceMark,PerformanceTiming});
  window.__receiveFrameMessage=(data,origin,sourceId,portIds=[])=>dispatchTrusted(window,new MessageEvent('message',{data,origin,source:remoteWindow(sourceId),ports:Array.from(portIds,wrapBrowserMessagePort)}));globalThis.__receiveFrameMessage=window.__receiveFrameMessage;
  Object.assign(window,{MessagePort,MessageChannel,DOMRectReadOnly,DOMRect,DOMMatrix});
  Object.assign(window,{MessagePort:BrowserMessagePort,MessageChannel:BrowserMessageChannel});
  const chromeLoadTimes=function loadTimes(){return{requestTime:performance.timeOrigin/1000,startLoadTime:performance.timeOrigin/1000,commitLoadTime:performance.timeOrigin/1000,finishDocumentLoadTime:0,finishLoadTime:0,firstPaintTime:0,firstPaintAfterLoadTime:0,navigationType:'Other',wasFetchedViaSpdy:true,wasNpnNegotiated:true,npnNegotiatedProtocol:'h2',wasAlternateProtocolAvailable:false,connectionInfo:'h2'}};
  const chromeCSI=function csi(){return{startE:performance.timeOrigin,onloadT:0,pageT:performance.now(),tran:15}};
  const chromeApp={isInstalled:false,getDetails:function getDetails(){return null},getIsInstalled:function getIsInstalled(){return false},installState:function installState(callback){if(typeof callback==='function')callback('not_installed')},runningState:function runningState(){return'cannot_run'},InstallState:{DISABLED:'disabled',INSTALLED:'installed',NOT_INSTALLED:'not_installed'},RunningState:{CANNOT_RUN:'cannot_run',READY_TO_RUN:'ready_to_run',RUNNING:'running'}};
  window.chrome={loadTimes:chromeLoadTimes,csi:chromeCSI,app:chromeApp};
  def(window,'innerWidth',{get:()=>host.viewport().width});def(window,'innerHeight',{get:()=>host.viewport().height});def(window,'outerWidth',{get:()=>host.viewport().outerWidth});def(window,'outerHeight',{get:()=>host.viewport().outerHeight});def(window,'devicePixelRatio',{get:()=>host.screen().devicePixelRatio});
  const timerHandler=(handler,args)=>typeof handler==='function'?()=>handler(...args):(()=>{const source=String(handler);return()=>eval(source)})();window.setTimeout=function setTimeout(handler,timeout=0,...args){return host.setTimer(timerHandler(handler,args),Number(timeout),false)};window.setInterval=function setInterval(handler,timeout=0,...args){return host.setTimer(timerHandler(handler,args),Number(timeout),true)};window.clearTimeout=function clearTimeout(id){return host.clearTimer(Number(id))};window.clearInterval=function clearInterval(id){return host.clearTimer(Number(id))};window.requestAnimationFrame=function requestAnimationFrame(callback){if(typeof callback!=='function')throw new TypeError('callback is not a function');return host.setTimer(()=>callback(performance.now()),16,false)};window.cancelAnimationFrame=function cancelAnimationFrame(id){return host.clearTimer(Number(id))};
  window.postMessage=bridgeOriginalPostMessage=function postMessage(message,targetOrigin='/',transfer=[]){if(targetOrigin&&typeof targetOrigin==='object'){transfer=targetOrigin.transfer||[];targetOrigin=targetOrigin.targetOrigin===undefined?'/':targetOrigin.targetOrigin}return host.framePost(host.selfFrameID(),message,String(targetOrigin),takeMessagePorts(transfer))};
  window.queueMicrotask=function queueMicrotask(callback){if(typeof callback!=='function')throw new TypeError('callback is not a function');Promise.resolve().then(callback)};
  window.fetch=function fetch(input,init={}){const request=input instanceof Request?new Request(input,init):new Request(input,init);return host.fetch(request.url,request.method,Object.fromEntries(request.headers),request.body==null?'':String(request.body)).then(r=>{const response=new Response(r.body,{status:r.status,headers:r.headers});const state=responseSlots.get(response);state.url=r.url;return response})};
  const performanceEntries=()=>host.performanceEntries([],true).map(makePerformanceEntry).concat(performanceMarks);
  const performanceOperations={
    timeOrigin:()=>host.performanceTimeOrigin(),now:()=>host.performanceNow(),entries:performanceEntries,
    entriesByType:type=>host.performanceEntries([type],true).map(makePerformanceEntry).concat(type==='mark'?performanceMarks:[]),
    entriesByName:(name,type)=>performanceEntries().filter(entry=>entry.name===name&&(type===undefined||entry.entryType===type))
  };
  window.performance=Object.create(Performance.prototype);registerRealmBinding(window.performance,'Performance',performanceOperations);window.DOMStringList=DOMStringList;window.Performance=Performance;window.PerformanceEntry=PerformanceEntry;window.PerformanceServerTiming=PerformanceServerTiming;window.PerformanceResourceTiming=PerformanceResourceTiming;window.PerformanceNavigationTiming=PerformanceNavigationTiming;window.PerformanceObserverEntryList=PerformanceObserverEntryList;window.PerformanceObserver=PerformanceObserver;
  window.getComputedStyle=e=>cssDeclaration(e,true);window.matchMedia=q=>({matches:host.media(String(q)),media:String(q),onchange:null,addEventListener(){},removeEventListener(){}});
  const handwrittenInterfaceConstructors=new Set(Object.getOwnPropertyNames(globalThis).map(name=>globalThis[name]).filter(value=>typeof value==='function'));
  const attributeUnsafeInterfaces=globalThis.__mimicAttributeUnsafeInterfaces;
  delete globalThis.__mimicAttributeUnsafeInterfaces;
  let known;
  const applyTargetExposure=exposure=>{
    const properties=exposure.properties||[];
    const publicationMissing=new Set();
    // Publish the frozen profile's static globals in their observed order once,
    // before unforgeable descriptors are installed. Later user additions and
    // delete/reinsert operations retain the engine's ordinary key ordering.
    if(exposure.propertyOrder&&exposure.propertyOrder.length){
      const descriptors=new Map(),define=Object.defineProperty;
      let republish=false;
      for(const name of exposure.propertyOrder){
        if(!engineGlobals.has(name))republish=true;
        if(!republish)continue;
        const descriptor=Object.getOwnPropertyDescriptor(globalThis,name)||(name in globalThis?{value:globalThis[name],writable:true,configurable:true}:undefined);
        if(descriptor&&!descriptor.configurable)throw new Error('non-configurable global publication: '+name);
        descriptors.set(name,descriptor);delete globalThis[name];
      }
      for(const [name,descriptor]of descriptors){
        if(snapshotPublication&&snapshotPublication.lateEngineKeys.has(name))continue;
        if(!descriptor)publicationMissing.add(name);
        define(globalThis,name,descriptor||{value:undefined,writable:true,configurable:true});
      }
    }
    const expected=new Map(properties.map(property=>[property.name,property]));
    for(const name of Object.getOwnPropertyNames(globalThis)){
      if(name.startsWith('__mimic')||name==='__receiveFrameMessage'||name==='__receiveMessagePort')continue;
      if(!expected.has(name)){const descriptor=Object.getOwnPropertyDescriptor(globalThis,name);if(descriptor&&descriptor.configurable)delete globalThis[name]}
    }
    for(const property of properties){
      if(snapshotPublication&&snapshotPublication.lateEngineKeys.has(property.name))continue;
      const propertyName=property.name;
      const descriptor={enumerable:property.enumerable,configurable:property.configurable};
      const current=publicationMissing.has(property.name)?undefined:Object.getOwnPropertyDescriptor(globalThis,property.name);
      if(property.valueType==='accessor'){
        let stored=current&&'value' in current?current.value:undefined;
        descriptor.get=current&&current.get?current.get:function(){return stored};
        if(property.setter)descriptor.set=current&&current.set?current.set:function(value){stored=value};
      }else{
        let value;
        if(current&&'value' in current)value=current.value;
        else if(!publicationMissing.has(property.name)&&property.name in globalThis)value=globalThis[property.name];
        else if(property.valueType==='function'){
          const functionName=property.functionName||property.name;
          value={[functionName]:function(){return host.semanticMissingAt('surface.js:615','Window.'+propertyName)}}[functionName];
        }else if(property.valueType==='boolean')value=false;
        else if(property.valueType==='number')value=0;
        else if(property.valueType==='string')value='';
        else value={};
        if(typeof value==='function'){
          if(property.functionName&&value.name!==property.functionName)Object.defineProperty(value,'name',{value:property.functionName,configurable:true});
          if(property.functionLength!==null&&value.length!==property.functionLength)Object.defineProperty(value,'length',{value:property.functionLength,configurable:true});
        }
        descriptor.value=value;descriptor.writable=property.writable!==false;
      }
      if(!current||current.configurable){
        if(snapshotPublication&&!descriptor.configurable){
          snapshotPublication.descriptors.set(property.name,descriptor);
          Object.defineProperty(globalThis,property.name,Object.assign({},descriptor,{configurable:true}));
        }else Object.defineProperty(globalThis,property.name,descriptor);
      }
    }
    for(const [interfaceName,members] of Object.entries(exposure.prototypes||{})){
      // ECMAScript intrinsics already own their engine-defined descriptors.
      // Reapplying the WebIDL prototype pass invalidates V8 species/prototype
      // guards even where the captured descriptor has the same visible shape.
      if(engineGlobals.has(interfaceName))continue;
      const ctor=globalThis[interfaceName],prototype=ctor&&ctor.prototype;
      if(!prototype)continue;
      for(const property of members){
        const propertyName=property.name;
        const current=Object.getOwnPropertyDescriptor(prototype,property.name);
        if(current){
          if(current.configurable){
            const normalized=Object.assign({},current,{enumerable:property.enumerable,configurable:property.configurable});
            if('value' in normalized){
              normalized.writable=property.writable!==false;
              if(typeof normalized.value==='function'){
                if(property.functionName&&normalized.value.name!==property.functionName)Object.defineProperty(normalized.value,'name',{value:property.functionName,configurable:true});
                if(property.functionLength!==null&&property.functionLength!==undefined&&normalized.value.length!==property.functionLength)Object.defineProperty(normalized.value,'length',{value:property.functionLength,configurable:true});
              }
            }
            Object.defineProperty(prototype,property.name,normalized);
          }
          continue;
        }
        // Handwritten classes still use JS assignment for native internal-slot
        // initialization. Installing a readonly WebIDL accessor here would
        // intercept that assignment; those attributes are added as each class
        // migrates to generated slot-backed semantics.
        if(property.valueType==='accessor'&&handwrittenInterfaceConstructors.has(ctor)&&attributeUnsafeInterfaces.has(interfaceName))continue;
        const descriptor={enumerable:property.enumerable,configurable:property.configurable};
        if(property.valueType==='accessor'){
          descriptor.get=property.getter?function(){return host.semanticMissingAt('surface.js:659',interfaceName+'.'+propertyName)}:undefined;
          descriptor.set=property.setter?function(){return host.semanticMissingAt('surface.js:660',interfaceName+'.'+propertyName+' setter')}:undefined;
        }else{
          let value;
          if(property.name==='constructor')value=ctor;
          else if(property.valueType==='function'){
            const functionName=property.functionName||property.name;
            value={[functionName]:function(){return host.semanticMissingAt('surface.js:666',interfaceName+'.'+propertyName)}}[functionName];
            if(property.functionLength!==null&&property.functionLength!==undefined)Object.defineProperty(value,'length',{value:property.functionLength,configurable:true});
          }else if(property.valueType==='boolean')value=false;
          else if(property.valueType==='number')value=0;
          else if(property.valueType==='string')value='';
          else value=null;
          descriptor.value=value;descriptor.writable=property.writable!==false;
        }
        Object.defineProperty(prototype,property.name,descriptor);
      }
    }
    // The captured target exposure, not handwritten class placement, owns the
    // observable prototype shape. Move implemented descriptors to the exact
    // ancestor that owns them in the pinned Chrome revision, then remove IDL
    // members that are not exposed by this target/profile. Private helpers on
    // still-unmigrated implementations remain temporarily hidden by naming;
    // migrated interfaces use WeakMap-backed internal slots and expose none.
    const prototypeTargets=new Map();
    for(const [interfaceName,members] of Object.entries(exposure.prototypes||{})){
      if(engineGlobals.has(interfaceName))continue;
      const ctor=globalThis[interfaceName];
      if(ctor&&ctor.prototype){
        prototypeTargets.set(ctor.prototype,new Set(members.map(member=>member.name)));
        markNative(ctor,ctor.name||interfaceName);
        const prototypeDescriptor=Object.getOwnPropertyDescriptor(ctor,'prototype');
        if(prototypeDescriptor&&prototypeDescriptor.writable)Object.defineProperty(ctor,'prototype',{writable:false});
        if(!Object.prototype.hasOwnProperty.call(ctor.prototype,Symbol.toStringTag))Object.defineProperty(ctor.prototype,Symbol.toStringTag,{value:interfaceName,configurable:true});
        for(const name of Object.getOwnPropertyNames(ctor.prototype)){
          const d=Object.getOwnPropertyDescriptor(ctor.prototype,name);
          if(name!=='constructor')markNative(d.value,name);
          markNative(d.get,name,'get ');markNative(d.set,name,'set ');
        }
      }
    }
    for(const [prototype,expectedMembers] of prototypeTargets){
      for(const name of Object.getOwnPropertyNames(prototype)){
        if(expectedMembers.has(name))continue;
        const source=Object.getOwnPropertyDescriptor(prototype,name);
        if(!source||!source.configurable)continue;
        let owner=Object.getPrototypeOf(prototype);
        while(owner&&owner!==Object.prototype){
          const ownerMembers=prototypeTargets.get(owner);
          if(ownerMembers&&ownerMembers.has(name))break;
          owner=Object.getPrototypeOf(owner);
        }
        if(owner&&owner!==Object.prototype){
          Object.defineProperty(owner,name,source);
          delete prototype[name];
        }else if(!name.startsWith('__')){
          delete prototype[name];
        }
      }
    }
  };
  const finalizeNativeBindings=()=>{
  // WebIDL bindings are native functions from JavaScript's point of view even
  // when their semantics are implemented in JavaScript underneath. Derive this
  // representation from the installed surface so generated and handwritten
  // interfaces cannot drift apart.
  const markedPrototypes=new Set([Object.prototype,Function.prototype]);
  const markMembers=owner=>{
    for(const member of Reflect.ownKeys(owner)){
      if(member==='constructor'||member==='prototype')continue;
      const descriptor=Object.getOwnPropertyDescriptor(owner,member);
      const method=descriptor&&descriptor.value;
      // Iteration aliases can reuse intrinsic functions such as Array#values.
      // Their identity and original function name survive the alias.
      markNative(method,typeof method==='function'&&method.name||String(member));
      markNative(descriptor&&descriptor.get,String(member),'get ');
      markNative(descriptor&&descriptor.set,String(member),'set ');
    }
  };
  for(const key of Reflect.ownKeys(globalThis)){
    if(engineGlobals.has(key))continue;
    const globalDescriptor=Object.getOwnPropertyDescriptor(globalThis,key);
    const value=globalDescriptor&&globalDescriptor.value;
    if(typeof value!=='function')continue;
    // A LegacyWindowAlias is another property pointing at the same interface
    // object; it must not rename that function in Function#toString.
    markNative(value,value.name||String(key));
    markMembers(value);
    let prototype=value.prototype;
    while(prototype&&prototype!==Object.prototype&&!markedPrototypes.has(prototype)){
      markedPrototypes.add(prototype);
      markMembers(prototype);
      prototype=Object.getPrototypeOf(prototype);
    }
  }
  };
  const finalizeBindings=()=>{
  const reflectString=(interfaceName,property,attribute=property,defaultValue='')=>{const ctor=globalThis[interfaceName];if(!ctor||!ctor.prototype)return;Object.defineProperty(ctor.prototype,property,{get(){const value=this.getAttribute(attribute);return value===null?defaultValue:value},set(value){this.setAttribute(attribute,String(value))},enumerable:true,configurable:true})};
  Object.defineProperty(CharacterData.prototype,'nodeName',{get(){const slot=elementSlot(this);return slot&&slot.type==='comment'?'#comment':'#text'},enumerable:true,configurable:true});
  Object.defineProperty(DocumentFragment.prototype,'nodeName',{get(){return'#document-fragment'},enumerable:true,configurable:true});
  reflectString('HTMLScriptElement','type');
  reflectString('HTMLInputElement','name');
  if(globalThis.HTMLInputElement&&globalThis.HTMLInputElement.prototype)Object.defineProperty(globalThis.HTMLInputElement.prototype,'type',{get(){const value=(this.getAttribute('type')||'text').toLowerCase();return new Set(['hidden','text','search','tel','url','email','password','date','month','week','time','datetime-local','number','range','color','checkbox','radio','file','submit','image','reset','button']).has(value)?value:'text'},set(value){this.setAttribute('type',String(value))},enumerable:true,configurable:true});
  if(globalThis.HTMLImageElement){globalThis.Image.prototype=globalThis.HTMLImageElement.prototype;Object.defineProperty(globalThis.HTMLImageElement.prototype,Symbol.toStringTag,{value:'HTMLImageElement',configurable:true})}
  if(globalThis.HTMLAudioElement)globalThis.Audio.prototype=globalThis.HTMLAudioElement.prototype;
  if(globalThis.HTMLOptionElement)globalThis.Option.prototype=globalThis.HTMLOptionElement.prototype;
  globalThis.globalThis=globalThis;
  for(const name of Object.getOwnPropertyNames(globalThis)){if(engineGlobals.has(name))continue;const ctor=globalThis[name];if(typeof ctor==='function'&&ctor.prototype&&!Object.prototype.hasOwnProperty.call(ctor.prototype,Symbol.toStringTag))Object.defineProperty(ctor.prototype,Symbol.toStringTag,{value:name,configurable:true})}
  for(const [name,fn] of [['setTimeout',setTimeout],['setInterval',setInterval],['clearTimeout',clearTimeout],['clearInterval',clearInterval],['fetch',fetch],['atob',atob],['btoa',btoa],['getComputedStyle',getComputedStyle],['matchMedia',matchMedia]])markNative(fn,name);
  // Window-exposed interface objects are non-enumerable data properties;
  // singleton browser objects are enumerable readonly accessors. Object.assign
  // above is only a convenient installation mechanism, not their final binding
  // descriptor.
  for(const ctor of [Event,MessageEvent,ErrorEvent,EventTarget,Node,DocumentFragment,ShadowRoot,Element,HTMLElement,SVGElement,HTMLScriptElement,HTMLImageElement,HTMLIFrameElement,HTMLAnchorElement,HTMLCollection,NodeList,DOMTokenList,CSSStyleDeclaration,Crypto,DOMException,PermissionsPolicy,TrustedHTML,TrustedScript,TrustedScriptURL,TrustedTypePolicy,TrustedTypePolicyFactory,URL,URLSearchParams,ReadableStream,ReadableStreamDefaultReader,ReadableStreamDefaultController,WritableStream,WritableStreamDefaultWriter,TransformStream,TransformStreamDefaultController,Blob,File,Worker,Document,HTMLDocument,Navigator,NavigatorUAData,Plugin,PluginArray,MimeType,MimeTypeArray,Screen,Location,History,Storage,XMLHttpRequestEventTarget,XMLHttpRequest,GPU,GPUAdapter,GPUAdapterInfo,GPUSupportedFeatures,GPUSupportedLimits,GPUDevice,RTCPeerConnection,RTCSessionDescription,RTCIceCandidate,RTCPeerConnectionIceEvent,RTCDataChannel,Performance,PerformanceEntry,PerformanceServerTiming,PerformanceResourceTiming,PerformanceNavigationTiming,PerformanceObserverEntryList,PerformanceObserver])Object.defineProperty(globalThis,ctor.name,{value:ctor,writable:true,enumerable:false,configurable:true});
  for(const ctor of [CharacterData,Text,Comment,SVGSVGElement,SVGRect,HTMLLinkElement,HTMLMetaElement,IntersectionObserver,BroadcastChannel,CryptoKey,SubtleCrypto,PerformanceMark,PerformanceTiming])Object.defineProperty(globalThis,ctor.name,{value:ctor,writable:true,enumerable:false,configurable:true});
  for(const [name,getter] of [['document',()=>document],['navigator',()=>nav],['screen',()=>scr],['location',()=>loc],['history',()=>hist],['localStorage',()=>storage],['sessionStorage',()=>sessionStorage],['crypto',()=>crypto],['trustedTypes',()=>trustedTypes],['performance',()=>window.performance]]){
    const value=getter();
    Object.defineProperty(globalThis,name,{get:()=>value,enumerable:true,configurable:true});
    if(globalThis.Window&&globalThis.Window.prototype)delete globalThis.Window.prototype[name];
  }
  if(globalThis.Window&&globalThis.Window.prototype){
    // Blink's WindowProperties is an unexposed intermediate interface. The IDL
    // generator must not publish its constructor, but Window still inherits
    // EventTarget behavior through it.
    if(!('addEventListener' in globalThis.Window.prototype))Object.setPrototypeOf(globalThis.Window.prototype,EventTarget.prototype);
    Object.defineProperty(globalThis.Window.prototype,Symbol.toStringTag,{value:'Window',configurable:true});
    Object.setPrototypeOf(globalThis,globalThis.Window.prototype);
    // QuickJS gives its realm global an internal "global" brand which wins
    // over an inherited tag. Blink exposes the local WindowProxy as Window.
    Object.defineProperty(globalThis,Symbol.toStringTag,{value:'Window',configurable:true});
  }
  markNative(frameElement,'frameElement','get ');Object.defineProperty(globalThis,'frameElement',{get:frameElement,enumerable:true,configurable:true});
  if(globalThis.Window&&globalThis.Window.prototype)delete globalThis.Window.prototype.frameElement;
  if(globalThis.Window&&globalThis.Window.prototype){
    const target=window;
    // The realm global is the local WindowProxy identity.  Wrapping it in a
    // second ECMAScript Proxy makes `window === globalThis` appear correct but
    // breaks dynamic code: a sloppy Function receives the engine realm global
    // as `this`, not that wrapper.  Keep one identity at the engine boundary;
    // cross-frame WindowProxy objects remain explicit remoteWindow proxies.
    for(const name of ['window','self','globalThis'])Object.defineProperty(target,name,{value:target,writable:true,enumerable:true,configurable:true});

  }
  };
  // Called only on a fresh deserialized, pre-script realm. Seed DOM must be
  // empty: preserving user-created wrappers across a host change is invalid.
  globalThis.__mimicRestoreBootstrap=freshHost=>{
    host=freshHost;hostToken=host.token();bridgeRealmID=host.selfRealmID();intlEnvironment=host.intlEnvironment();
    security=host.documentSecurity();
    const policy=permissionsPolicySlots.get(new PermissionsPolicy(hostToken,host.permissionsPolicy()));
    Object.assign(permissionsPolicySlots.get(documentPolicy),policy);
    remoteWindowCache.clear();remoteDocumentCache.clear();crossRealmCache.clear();
    crossRealmSymbols.clear();crossRealmSymbolReferences.clear();localCrossRealmSymbols.clear();nextCrossRealmSymbolID=0;
    elementWrappers.clear();documentWrappers.clear();frameElementCache=undefined;uaData=undefined;
    const relations=host.windowRelations();
    windowTop=relations.top===relations.self?window:remoteWindow(relations.top);
    windowParent=relations.parent===relations.self?window:remoteWindow(relations.parent);
    for(const restore of bootstrapRestoreHooks)restore();
    tracedAccesses.clear();
    // Conditional V8 intrinsics were absent in the serializing context and
    // are now installed alongside the published surface.
    known=new Set(Reflect.ownKeys(globalThis));
    for(const [name,callbacks] of bootstrapCallbacks)host[name](...callbacks);
    host.ready();
  };
  globalThis.__mimicEvalSourceResolver=evalSourceResolver;known=new Set(Reflect.ownKeys(globalThis));host.ready();globalThis.__mimicUnsupportedProbe=n=>{if(!known.has(n))host.unsupported(String(n))};
})(__mimic);
