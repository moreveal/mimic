// Only WebIDL conversion, brands, callback references, and immutable event
// snapshots live here. Utterance properties, voice data and synthesis queues
// are projections of the document-owned, platform-neutral Go service.
if(typeof SpeechSynthesis==='function'&&typeof SpeechSynthesisUtterance==='function'){
  const synthesis=new EventTarget();Object.setPrototypeOf(synthesis,SpeechSynthesis.prototype);
  Object.defineProperty(globalThis,'speechSynthesis',{get(){return synthesis},enumerable:true,configurable:true});
  const utteranceIDs=new WeakMap(),utterances=new Map(),voiceIDs=new WeakMap(),voices=new Map(),speechEvents=new WeakMap();
  const data=(...args)=>host.speechState(...args);
  const nativeMethod=(prototype,name,fn)=>{markNative(fn,name);Object.defineProperty(prototype,name,{value:fn,writable:true,enumerable:true,configurable:true})};
  const getter=(prototype,name,get,set)=>{markNative(get,name,'get ');if(set)markNative(set,name,'set ');Object.defineProperty(prototype,name,{get,set,enumerable:true,configurable:true})};
  const bindCall=(receiver,kind,operation,args)=>{const binding=requireRealmBinding(receiver,kind);return callRealmBinding(receiver,binding,operation,args)};
  const convertFloat=value=>{const result=Math.fround(+value);if(!Number.isFinite(result))throw new TypeError('The provided float value is non-finite.');return result};
  const replaceConstructor=(name,ctor)=>{
    const previous=globalThis[name];Object.defineProperty(ctor,'name',{value:name,configurable:true});Object.defineProperty(ctor,'length',{value:previous.length,configurable:true});
    ctor.prototype=previous.prototype;Object.defineProperty(ctor.prototype,'constructor',{value:ctor,writable:true,configurable:true});
    markNative(ctor,name);Object.defineProperty(globalThis,name,{value:ctor,writable:true,configurable:true});
  };
  const createVoice=id=>{
    if(!voices.has(id)){const voice=Object.create(SpeechSynthesisVoice.prototype);voiceIDs.set(voice,id);registerRealmBinding(voice,'SpeechSynthesisVoice',{id:()=>id,read:name=>{const row=data('voiceRead',id);return name==='voiceURI'?row.name:row[name]}});voices.set(id,voice)}
    return voices.get(id);
  };
  for(const name of ['voiceURI','name','lang','localService','default'])getter(SpeechSynthesisVoice.prototype,name,function(){return bindCall(this,'SpeechSynthesisVoice','read',[name])});
  function Utterance(text=''){
    if(!new.target)throw new TypeError("Please use the 'new' operator.");
    text=bindingString(text);const object=Reflect.construct(EventTarget,[],new.target),id=data('create',text);
    utteranceIDs.set(object,id);utterances.set(id,object);
    registerRealmBinding(object,'SpeechSynthesisUtterance',{
      identity:()=>({id,owner:data('owner')}),
      read:name=>name==='voice'?data('voiceReference',id):data('read',id)[name],
      set:(name,value,reference)=>data('set',id,name,value,reference),handlerGet:name=>eventHandlerRecord(object,name).value,handlerSet:(name,value)=>setEventHandlerValue(object,name,value)
    });return object;
  }
  replaceConstructor('SpeechSynthesisUtterance',Utterance);
  for(const name of ['text','lang','voice','volume','rate','pitch'])getter(Utterance.prototype,name,function(){return bindCall(this,'SpeechSynthesisUtterance','read',[name])},function(value){
    if(name==='voice'){const id=value==null?'':bindCall(value,'SpeechSynthesisVoice','id',[]);bindCall(this,'SpeechSynthesisUtterance','set',[name,id,value??null]);return}
    value=name==='text'||name==='lang'?bindingString(value):convertFloat(value);
    bindCall(this,'SpeechSynthesisUtterance','set',[name,value]);
  });
  const handlers=(prototype,names,kind)=>{for(const name of names)getter(prototype,'on'+name,function(){return bindCall(this,kind,'handlerGet',[name])},function(value){bindCall(this,kind,'handlerSet',[name,value])})};
  handlers(Utterance.prototype,['start','end','error','pause','resume','mark','boundary'],'SpeechSynthesisUtterance');
  const eventData=(object,type,init)=>{
    if(!init||init.utterance==null)throw new TypeError("Required member 'utterance' is undefined.");
    requireRealmBinding(init.utterance,'SpeechSynthesisUtterance');
    const row={utterance:init.utterance,charIndex:(+init.charIndex)>>>0,charLength:(+init.charLength)>>>0,elapsedTime:init.elapsedTime===undefined?0:convertFloat(init.elapsedTime),name:bindingString(init.name??'')};
    if(type==='SpeechSynthesisErrorEvent'){
      const allowed=['canceled','interrupted','audio-busy','audio-hardware','network','synthesis-unavailable','synthesis-failed','language-unavailable','voice-unavailable','text-too-long','invalid-argument','not-allowed'];
      const error=bindingString(init.error);if(!allowed.includes(error))throw new TypeError('Invalid SpeechSynthesisErrorCode');row.error=error;
    }
    speechEvents.set(object,row);
  };
  function SynthesisEvent(type,init){if(!new.target)throw new TypeError("Please use the 'new' operator.");const event=Reflect.construct(Event,[bindingString(type),init],new.target);eventData(event,'SpeechSynthesisEvent',init);return event}
  replaceConstructor('SpeechSynthesisEvent',SynthesisEvent);
  function SynthesisErrorEvent(type,init){if(!new.target)throw new TypeError("Please use the 'new' operator.");const event=Reflect.construct(Event,[bindingString(type),init],new.target);eventData(event,'SpeechSynthesisErrorEvent',init);return event}
  replaceConstructor('SpeechSynthesisErrorEvent',SynthesisErrorEvent);
  for(const name of ['utterance','charIndex','charLength','elapsedTime','name'])getter(SynthesisEvent.prototype,name,function(){const row=speechEvents.get(this);if(!row)throw new TypeError('Illegal invocation');return row[name]});
  getter(SynthesisErrorEvent.prototype,'error',function(){const row=speechEvents.get(this);if(!row||row.error===undefined)throw new TypeError('Illegal invocation');return row.error});
  registerRealmBinding(synthesis,'SpeechSynthesis',{
    handlerGet:name=>eventHandlerRecord(synthesis,name).value,handlerSet:(name,value)=>setEventHandlerValue(synthesis,name,value),read:name=>data('status')[name],voices:()=>data('voices').map(row=>createVoice(row.id)),
    speak:identity=>data('speak',identity.id,identity.owner),cancel:()=>data('cancel'),pause:()=>data('pause'),resume:()=>data('resume')
  });
  for(const name of ['pending','speaking','paused'])getter(SpeechSynthesis.prototype,name,function(){return bindCall(this,'SpeechSynthesis','read',[name])});
  nativeMethod(SpeechSynthesis.prototype,'getVoices',function getVoices(){return bindCall(this,'SpeechSynthesis','voices',[])});
  nativeMethod(SpeechSynthesis.prototype,'speak',function speak(utterance){if(!arguments.length)throw new TypeError("Failed to execute 'speak' on 'SpeechSynthesis': 1 argument required, but only 0 present.");const identity=bindCall(utterance,'SpeechSynthesisUtterance','identity',[]);return bindCall(this,'SpeechSynthesis','speak',[identity])});
  for(const name of ['cancel','pause','resume'])nativeMethod(SpeechSynthesis.prototype,name,{[name](){return bindCall(this,'SpeechSynthesis',name,[])}}[name]);
  handlers(SpeechSynthesis.prototype,['voiceschanged'],'SpeechSynthesis');
  registerBootstrapCallback('installSpeechNotifier',row=>{
    if(row.kind==='voiceschanged'){dispatchNative(synthesis,new Event('voiceschanged'));return}
    const utterance=utterances.get(row.id);if(!utterance)return;
    const Constructor=row.kind==='error'?SynthesisErrorEvent:SynthesisEvent;
    const event=new Constructor(row.kind,{...row,utterance});eventSlots.get(event).timeStamp=row.timeStamp;dispatchNative(utterance,event);
  });
}
