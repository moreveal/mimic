// A single dispatch path serves script-created events and host lifecycle
// events. Listener records belong to their EventTarget, never a global Page.
{
  const member=(prototype,name,value)=>{Object.defineProperty(value,'name',{value:name,configurable:true});markNative(value,name);Object.defineProperty(prototype,name,{value,writable:true,enumerable:true,configurable:true})};
  const accessor=(prototype,name,get,set)=>{markNative(get,name,'get ');markNative(set,name,'set ');Object.defineProperty(prototype,name,{get,set,enumerable:true,configurable:true})};
  const stateOf=event=>{const state=eventSlots.get(event);if(!state)throw new TypeError('Illegal invocation');return state};
  const optionCapture=options=>typeof options==='boolean'?options:!!options?.capture;
  member(EventTarget.prototype,'addEventListener',function(type,callback,options){
    const target=this==null?window:this;type=String(type);
    if(callback==null)return;if(typeof callback!=='function'&&typeof callback!=='object')throw new TypeError('Invalid event listener');
    const capture=optionCapture(options),signal=typeof options==='object'?options?.signal:null;
    if(signal?.aborted)return;
    const listeners=listenersFor(target),list=listeners.get(type)||[];
    if(list.some(record=>record.callback===callback&&record.capture===capture&&!record.removed))return;
    const record={callback,capture,once:!!options?.once,passive:!!options?.passive,removed:false};
    list.push(record);listeners.set(type,list);
    if(signal)signal.addEventListener('abort',()=>{record.removed=true;const index=list.indexOf(record);if(index>=0)list.splice(index,1)},{once:true});
  });
  member(EventTarget.prototype,'removeEventListener',function(type,callback,options){const list=listenersFor(this==null?window:this).get(String(type))||[],capture=optionCapture(options),index=list.findIndex(record=>record.callback===callback&&record.capture===capture);if(index>=0){list[index].removed=true;list.splice(index,1)}});
  for(const [name,key,fallback] of [['target','target',null],['srcElement','target',null],['currentTarget','currentTarget',null],['eventPhase','phase',0],['composed','composed',false]])accessor(Event.prototype,name,function(){return stateOf(this)[key]??fallback});
  accessor(Event.prototype,'cancelBubble',function(){return !!stateOf(this).stopped},function(value){if(value)stateOf(this).stopped=true});
  accessor(Event.prototype,'returnValue',function(){return !stateOf(this).defaultPrevented},function(value){if(!value)this.preventDefault()});
  member(Event.prototype,'stopPropagation',function(){stateOf(this).stopped=true});
  member(Event.prototype,'stopImmediatePropagation',function(){const state=stateOf(this);state.stopped=true;state.immediate=true});
  member(Event.prototype,'preventDefault',function(){const state=stateOf(this);if(state.cancelable&&!state.passive)state.defaultPrevented=true});
  const insideRoot=(node,root)=>{for(let current=node;current instanceof Node;current=current.parentNode||(current instanceof ShadowRoot?current.host:null))if(current===root)return true;return false};
  member(Event.prototype,'composedPath',function(){const state=stateOf(this);return (state.path||[]).filter(node=>{for(let current=node;current instanceof Node;){const root=current.getRootNode();if(!(root instanceof ShadowRoot))break;if(root.mode==='closed'&&!insideRoot(state.currentTarget,root))return false;current=root.host}return true})});
  const retarget=(target,current)=>{let node=target;while(node instanceof Node){const root=node.getRootNode();if(!(root instanceof ShadowRoot)||current instanceof Node&&current.getRootNode()===root)return node;node=root.host}return node};
  dispatchEventCore=(target,event,trusted)=>{
    const state=stateOf(event);if(state.dispatching||!state.type)throw new DOMException('Event is already being dispatched or uninitialized','InvalidStateError');
    state.trusted=!!trusted;state.dispatching=true;state.stopped=false;state.immediate=false;
    const path=[target];let current=target;
    while(current instanceof Node){let parent=current.parentNode;if(!parent&&current instanceof ShadowRoot&&state.composed)parent=current.host;if(!parent&&current instanceof Document&&state.type!=='load')parent=current.defaultView;if(!parent)break;path.push(parent);current=parent}
    state.path=path;
    const report=error=>{try{console.error(error?.stack||String(error))}catch{}};
    const invoke=(current,capture,phase)=>{
      state.currentTarget=current;state.target=retarget(target,current);state.phase=phase;
      const list=listenersFor(current).get(state.type)||[];
      for(const record of list.slice()){
        if(state.immediate)break;
        if(typeof record==='function'){if(capture)continue;try{record.call(current,event)}catch(error){report(error)}continue}
        if(record.removed||record.capture!==capture)continue;
        if(record.once){record.removed=true;const index=list.indexOf(record);if(index>=0)list.splice(index,1)}
        state.passive=record.passive;
        try{if(typeof record.callback==='function')record.callback.call(current,event);else if(typeof record.callback.handleEvent==='function')record.callback.handleEvent(event)}catch(error){report(error)}
        state.passive=false;
      }
      if(!capture&&!state.immediate){const handler=internalEventHandler(current,state.type);if(typeof handler==='function')try{if(handler.call(current,event)===false)event.preventDefault()}catch(error){report(error)}}
    };
    try{
      for(let i=path.length-1;i>0&&!state.stopped;i--)invoke(path[i],true,retarget(target,path[i])===path[i]?2:1);
      if(!state.stopped){invoke(target,true,2);if(!state.immediate)invoke(target,false,2)}
      for(let i=1;i<path.length&&!state.stopped;i++){const atTarget=retarget(target,path[i])===path[i];if(state.bubbles||atTarget)invoke(path[i],false,atTarget?2:3)}
    }finally{state.currentTarget=null;state.phase=0;state.path=[];state.target=retarget(target,path[path.length-1]);state.dispatching=false;state.passive=false;state.stopped=false;state.immediate=false}
    return !state.defaultPrevented;
  };
  const uiSlots=new WeakMap(),mouseSlots=new WeakMap(),pointerSlots=new WeakMap();
  class UIEvent extends Event {constructor(type,init={}){super(type,init);uiSlots.set(this,{view:init.view??null,detail:Number(init.detail)||0,which:0})}}
  for(const name of ['view','detail','which'])accessor(UIEvent.prototype,name,function(){return uiSlots.get(this)?.[name]});
  class MouseEvent extends UIEvent {constructor(type,init={}){super(type,init);const data={};for(const name of ['screenX','screenY','clientX','clientY','button','buttons','movementX','movementY'])data[name]=Number(init[name])||0;for(const name of ['ctrlKey','shiftKey','altKey','metaKey'])data[name]=!!init[name];data.relatedTarget=init.relatedTarget??null;mouseSlots.set(this,data)}}
  for(const name of ['screenX','screenY','clientX','clientY','button','buttons','movementX','movementY','ctrlKey','shiftKey','altKey','metaKey','relatedTarget'])accessor(MouseEvent.prototype,name,function(){return mouseSlots.get(this)?.[name]});
  accessor(MouseEvent.prototype,'which',function(){return this.button+1});
  for(const [name,source] of [['x','clientX'],['y','clientY'],['pageX','clientX'],['pageY','clientY'],['offsetX','clientX'],['offsetY','clientY']])accessor(MouseEvent.prototype,name,function(){return this[source]});
  member(MouseEvent.prototype,'getModifierState',function(key){return !!mouseSlots.get(this)?.[{Alt:'altKey',Control:'ctrlKey',Meta:'metaKey',Shift:'shiftKey'}[String(key)]]});
  class PointerEvent extends MouseEvent {constructor(type,init={}){super(type,init);const data={};for(const [name,fallback] of [['pointerId',0],['width',1],['height',1],['pressure',0],['tangentialPressure',0],['tiltX',0],['tiltY',0],['twist',0],['altitudeAngle',Math.PI/2],['azimuthAngle',0],['persistentDeviceId',0]])data[name]=init[name]===undefined?fallback:Number(init[name]);data.pointerType=String(init.pointerType||'');data.isPrimary=!!init.isPrimary;pointerSlots.set(this,data)}}
  for(const name of ['pointerId','width','height','pressure','tangentialPressure','tiltX','tiltY','twist','altitudeAngle','azimuthAngle','persistentDeviceId','pointerType','isPrimary'])accessor(PointerEvent.prototype,name,function(){return pointerSlots.get(this)?.[name]});
  for(const name of ['getCoalescedEvents','getPredictedEvents'])member(PointerEvent.prototype,name,function(){return []});
  for(const [name,value] of Object.entries({UIEvent,MouseEvent,PointerEvent})){markNative(value,name);Object.defineProperty(globalThis,name,{value,writable:true,configurable:true});Object.defineProperty(value.prototype,Symbol.toStringTag,{value:name,configurable:true})}
  const clicking=new WeakSet();
  member(HTMLElement.prototype,'click',function(){
    if(!(this instanceof HTMLElement))throw new TypeError('Illegal invocation');
    if(clicking.has(this)||['BUTTON','INPUT','SELECT','TEXTAREA','OPTION'].includes(this.tagName)&&this.disabled)return;
    clicking.add(this);
    try{
      const event=new PointerEvent('click',{bubbles:true,cancelable:true,composed:true,view:this.ownerDocument?.defaultView??null,pointerId:-1,pointerType:'',isPrimary:false});
      const checkable=this.tagName==='INPUT'&&['checkbox','radio'].includes(this.type),previous=checkable?this.checked:null;
      if(checkable)this.checked=this.type==='radio'?true:!previous;
      if(!dispatchEventCore(this,event,false)){if(checkable)this.checked=previous;return}
      if(checkable){dispatchEventCore(this,new Event('input',{bubbles:true,composed:true}),true);dispatchEventCore(this,new Event('change',{bubbles:true}),true)}
      if(this.tagName==='A'&&this.hasAttribute('href'))host.navigate(this.href);
    }finally{clicking.delete(this)}
  });
}
