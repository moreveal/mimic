// Trusted browser input shares the canonical form/focus slots. This closure is
// captured by the host before author scripts run and is removed from Window.
{
  const parse=JSON.parse,stringify=JSON.stringify;
  const keyboardSlots=new WeakMap(),inputSlots=new WeakMap(),focusSlots=new WeakMap(),submitSlots=new WeakMap();
  const accessor=(prototype,key,get)=>Object.defineProperty(prototype,key,{get,enumerable:true,configurable:true});
  class KeyboardEvent extends UIEvent {
    constructor(type,init={}){
      super(type,init);const state={};
      for(const key of ['key','code'])state[key]=String(init[key]??'');
      for(const key of ['location','keyCode','charCode'])state[key]=Number(init[key])||0;
      for(const key of ['ctrlKey','shiftKey','altKey','metaKey','repeat','isComposing'])state[key]=!!init[key];
      keyboardSlots.set(this,state);
    }
    getModifierState(key){return !!keyboardSlots.get(this)?.[{Alt:'altKey',Control:'ctrlKey',Meta:'metaKey',Shift:'shiftKey'}[String(key)]]}
  }
  for(const key of ['key','code','location','keyCode','charCode','ctrlKey','shiftKey','altKey','metaKey','repeat','isComposing'])accessor(KeyboardEvent.prototype,key,function(){const state=keyboardSlots.get(this);if(!state)throw new TypeError('Illegal invocation');return state[key]});
  accessor(KeyboardEvent.prototype,'which',function(){return this.type==='keypress'?this.charCode:this.keyCode});
  for(const [name,value]of [['DOM_KEY_LOCATION_STANDARD',0],['DOM_KEY_LOCATION_LEFT',1],['DOM_KEY_LOCATION_RIGHT',2],['DOM_KEY_LOCATION_NUMPAD',3]])for(const owner of [KeyboardEvent,KeyboardEvent.prototype])Object.defineProperty(owner,name,{value,enumerable:true});
  class InputEvent extends UIEvent {
    constructor(type,init={}){super(type,init);inputSlots.set(this,{data:init.data==null?null:String(init.data),inputType:String(init.inputType||''),isComposing:!!init.isComposing,dataTransfer:init.dataTransfer??null})}
    getTargetRanges(){if(!inputSlots.has(this))throw new TypeError('Illegal invocation');return []}
  }
  for(const key of ['data','inputType','isComposing','dataTransfer'])accessor(InputEvent.prototype,key,function(){const state=inputSlots.get(this);if(!state)throw new TypeError('Illegal invocation');return state[key]});
  class FocusEvent extends UIEvent {constructor(type,init={}){super(type,init);focusSlots.set(this,init.relatedTarget??null)}}
  accessor(FocusEvent.prototype,'relatedTarget',function(){if(!focusSlots.has(this))throw new TypeError('Illegal invocation');return focusSlots.get(this)});
  class SubmitEvent extends Event {constructor(type,init={}){super(type,init);submitSlots.set(this,init.submitter??null)}}
  accessor(SubmitEvent.prototype,'submitter',function(){if(!submitSlots.has(this))throw new TypeError('Illegal invocation');return submitSlots.get(this)});
  for(const [name,value]of Object.entries({KeyboardEvent,InputEvent,FocusEvent,SubmitEvent})){
    markNative(value,name);Object.defineProperty(globalThis,name,{value,writable:true,configurable:true});Object.defineProperty(value.prototype,Symbol.toStringTag,{value:name,configurable:true});
  }
  const constructors={Event,KeyboardEvent,InputEvent,FocusEvent,SubmitEvent,MouseEvent,PointerEvent};
  let activateCommand=()=>{};
  if(globalThis.HTMLButtonElement&&globalThis.HTMLDialogElement){
  const commandSlots=new WeakMap(),commandTargets=new WeakMap();
  class CommandEvent extends Event {
    constructor(type,init={}){super(type,init);const source=init.source??null;if(source!==null&&!(source instanceof Element))throw new TypeError('source must be an Element');commandSlots.set(this,{command:String(init.command??''),source})}
  }
  for(const key of ['command','source'])accessor(CommandEvent.prototype,key,function(){const state=commandSlots.get(this);if(!state)throw new TypeError('Illegal invocation');return state[key]});
  markNative(CommandEvent,'CommandEvent');Object.defineProperty(globalThis,'CommandEvent',{value:CommandEvent,writable:true,configurable:true});Object.defineProperty(CommandEvent.prototype,Symbol.toStringTag,{value:'CommandEvent',configurable:true});
  const commandFor=button=>{
    const explicit=commandTargets.get(button);if(explicit)return explicit.getRootNode()===button.getRootNode()?explicit:null;
    const id=button.getAttribute('commandfor');if(id===null)return null;
    return button.getRootNode().getElementById(id);
  };
  const commandOf=button=>{const raw=button.getAttribute('command')||'',value=raw.toLowerCase();return raw.startsWith('--')?raw:['show-modal','close','request-close','show-popover','hide-popover','toggle-popover'].includes(value)?value:''};
  const buttonReceiver=value=>{if(!(value instanceof HTMLButtonElement))throw new TypeError('Illegal invocation');return value};
  Object.defineProperties(HTMLButtonElement.prototype,{
    command:{get(){return commandOf(buttonReceiver(this))},set(value){buttonReceiver(this).setAttribute('command',String(value))},enumerable:true,configurable:true},
    commandForElement:{get(){return commandFor(buttonReceiver(this))},set(value){buttonReceiver(this);if(value!==null&&!(value instanceof Element))throw new TypeError('commandForElement must be an Element');commandTargets.delete(this);if(value===null)this.removeAttribute('commandfor');else{this.setAttribute('commandfor','');commandTargets.set(this,value)}},enumerable:true,configurable:true}
  });
  // Default activation uses private dialog operations shared with public methods;
  // author replacements cannot intercept UA work or lose the command source.
  activateCommand=button=>{
    const target=commandFor(button),command=commandOf(button);
    if(!target||!command||button.disabled)return;
    if(command.endsWith('popover')){host.semanticMissingAt('input.js/activateCommand','HTMLButtonElement.popoverCommand');return}
    if(!command.startsWith('--')&&!(target instanceof HTMLDialogElement))return;
    if(!dispatchTrusted(target,new CommandEvent('command',{command,source:button,cancelable:true,composed:true})))return;
    if(command==='show-modal'&&!target.open&&target.isConnected)compatibilityElementState.showDialog(target,button);
    else if(command==='close')compatibilityElementState.closeDialog(target,undefined,button);
    else if(command==='request-close'&&target.open&&dispatchTrusted(target,new Event('cancel',{cancelable:true})))compatibilityElementState.closeDialog(target,undefined,button);
  };
  }
  compatibilityElementState.controlGeometry=(element,entries)=>{
    const data=elementSlot(element),tag=data?.tagName;
    if(!['INPUT','BUTTON','TEXTAREA','SELECT'].includes(tag))return null;
    const attribute=name=>host.getAttribute(data.nodeId,name),property=(node,key)=>compatibilityElementState.formOperation(node,'get',key);
    const type=tag==='INPUT'?String(attribute('type')||'text').toLowerCase():'',own=key=>entries.find(e=>e.name===key)?.value;
    if(tag==='INPUT'&&type==='hidden')return {width:0,height:0};
    if(tag==='INPUT'&&['checkbox','radio'].includes(type))return {width:13,height:13};
    const size=own('font-size')?cssComputedFontSize(element):40/3,scale=size/(40/3);
    if(size===null)throw new Error('Control font metrics unavailable');
    const family=own('font-family')||(tag==='TEXTAREA'?'monospace':'Arial');
    const measure=text=>{if(size===0)return 0;const shaped=parse(host.shapeText(text,family,size,400,0,0,0));if(shaped.error)throw new Error('Control font metrics unavailable: '+shaped.error);return Math.ceil(shaped.glyphs.reduce((sum,glyph)=>sum+glyph.advance,0)*64)/64};
    // Frozen Windows UA metrics: author dimensions still take precedence in
    // layoutRectFor. Text-bearing buttons use the existing font shaper.
    if(tag==='BUTTON'||tag==='INPUT'&&['button','submit','reset'].includes(type)){
      const text=tag==='BUTTON'?host.textContent(data.nodeId):property(element,'value')||(type==='submit'?'Submit':type==='reset'?'Reset':'');
      return {width:measure(text)+16*scale,height:21*scale};
    }
    if(tag==='TEXTAREA')return {width:(Math.max(1,Number(attribute('cols'))||20)*8+8)*scale,height:(Math.max(1,Number(attribute('rows'))||2)*15+6)*scale};
    if(tag==='SELECT'){
      const labels=Array.from(compatibilitySelectors.query(element,'option'),option=>measure(property(option,'label')||property(option,'text')));
      return {width:Math.ceil(Math.max(0,...labels))+22*scale,height:19*scale};
    }
    return {width:(Math.max(1,Number(attribute('size'))||20)*7+37)*scale,height:21*scale};
  };
  const nodeID=node=>elementSlot(node)?.nodeId||0;
  const eventFor=(kind,type,init)=>new constructors[kind](type,{...init,view:window,relatedTarget:wrap(init.relatedNode||0)||null,submitter:wrap(init.submitterNode||0)||null});
  const emit=(target,kind,type,init={},trusted=true,native=true)=>{
    const event=eventFor(kind,type,init),stamp=eventSlots.get(event),timing=trusted&&native&&!isolated?host.performanceEventStart(type,nodeID(target),stamp.timeStamp,stamp.cancelable):0;
    const allowed=dispatchEventCore(target,event,trusted,native);
    const others=host.broadcastInputEvent(nodeID(target),stringify({kind,type,init,canceled:!allowed,trusted,native}));
    if(timing)host.performanceEventEnd(timing);
    return allowed&&others;
  };
  const originalFocus=HTMLElement.prototype.focus,originalBlur=HTMLElement.prototype.blur;
  const active=Object.getOwnPropertyDescriptor(Document.prototype,'activeElement').get,originalFocused=compatibilityElementState.focused;
  let isolated=host.isIsolatedInputWorld();
  bootstrapRestoreHooks.push(()=>{isolated=host.isIsolatedInputWorld()});
  const main=(element,operation,params={})=>parse(host.mainWorldInput(nodeID(element),operation,stringify(params)));
  const control=(element,operation,key,args=[])=>compatibilityElementState.formOperation(element,operation,key,args);
  const valueOf=element=>String(control(element,'get','value'));
  const editable=element=>element&&!element.disabled&&!element.readOnly&&(element.localName==='textarea'||element.localName==='input'&&['text','search','tel','url','email','password'].includes(element.type));
  const focus=element=>{if(element&&element.isConnected&&!element.disabled&&!(element.localName==='input'&&element.type==='hidden'))originalFocus.call(element)};
  const changedValues=new WeakMap();
  compatibilityElementState.dispatchFocus=(target,type,related,bubbles)=>{
    if(type==='blur'&&changedValues.has(target)){
      const previous=changedValues.get(target);changedValues.delete(target);
      if(previous!==valueOf(target))emit(target,'Event','change',{bubbles:true});
    }
    return emit(target,'FocusEvent',type,{bubbles,composed:true,relatedNode:nodeID(related)});
  };
  Object.defineProperty(HTMLElement.prototype,'focus',{value:function(){if(isolated){main(this,'focus');return}focus(this)},writable:true,enumerable:true,configurable:true});
  Object.defineProperty(HTMLElement.prototype,'blur',{value:function(){if(isolated){main(this,'blur');return}originalBlur.call(this)},writable:true,enumerable:true,configurable:true});
  Object.defineProperty(Document.prototype,'activeElement',{get(){return isolated?(wrap(main(null,'active').nodeID)||document.body||document.documentElement):active.call(document)},enumerable:true,configurable:true});
  compatibilityElementState.focused=()=>isolated?(wrap(main(null,'focused').nodeID)||null):originalFocused();
  const selection=element=>{
    const value=valueOf(element),start=control(element,'get','selectionStart'),end=control(element,'get','selectionEnd');
    return {value,start:start??value.length,end:end??value.length};
  };
  const select=(element,start,end)=>{if(control(element,'get','selectionStart')!==null)control(element,'call','setSelectionRange',[start,end])};
  const edit=(element,text,inputType='insertText')=>{
    if(!editable(element))return;
    const original=selection(element);
    const data=inputType.startsWith('delete')?null:text;
    if(!emit(element,'InputEvent','beforeinput',{bubbles:true,cancelable:true,composed:true,inputType,data}))return;
    if(!element.isConnected)return;
    const before=selection(element);let start=before.start,end=before.end;
    if(inputType==='deleteContentBackward'&&start===end&&start>0){start--;if(start>0&&/[\uDC00-\uDFFF]/.test(before.value[start])&&/[\uD800-\uDBFF]/.test(before.value[start-1]))start--}
    if(inputType==='deleteContentForward'&&start===end&&end<before.value.length){end++;if(end<before.value.length&&/[\uD800-\uDBFF]/.test(before.value[end-1])&&/[\uDC00-\uDFFF]/.test(before.value[end]))end++}
    const inserted=text??'',maximum=Number(element.getAttribute('maxlength'));
    let addition=element.hasAttribute('maxlength')&&maximum>=0?inserted.slice(0,Math.max(0,maximum-before.value.length+end-start)):inserted;
    if(addition.length<inserted.length&&/[\uD800-\uDBFF]/.test(addition.at(-1)||'')&&/[\uDC00-\uDFFF]/.test(inserted[addition.length]))addition=addition.slice(0,-1);
    const next=before.value.slice(0,start)+addition+before.value.slice(end);
    if(next===before.value&&start===end&&addition==='')return;
    if(!changedValues.has(element))changedValues.set(element,before.value);
    control(element,'set','value',[next]);
    // Chrome's insertion uses the live range after beforeinput but places the
    // caret relative to the original composition range for Input.insertText.
    const caret=inputType==='insertText'?original.start+addition.length:start+addition.length;
    select(element,caret,caret);
    emit(element,'InputEvent','input',{bubbles:true,composed:true,inputType,data});
  };
  const modifiers=value=>({altKey:!!(value&1),ctrlKey:!!(value&2),metaKey:!!(value&4),shiftKey:!!(value&8)});
  const tab=(element,backwards)=>{
    const list=compatibilitySelectors.query(document,'input,textarea,button,select,a[href],[tabindex]').filter(e=>e.tabIndex>=0&&!e.disabled&&!(e.localName==='input'&&e.type==='hidden'));
    if(!list.length)return;let at=list.indexOf(element);at=(at+(backwards?-1:1)+list.length)%list.length;focus(list[at]);
  };
  const keyCommand=params=>{
    let target=active.call(document)||document.body||document.documentElement;
    const kind=params.type,flags=modifiers(params.modifiers||0),text=params.text||'';
    const init={bubbles:true,cancelable:true,composed:true,key:params.key||'',code:params.code||'',keyCode:params.windowsVirtualKeyCode||params.nativeVirtualKeyCode||0,charCode:0,repeat:!!params.autoRepeat,location:params.location||(params.isKeypad?3:0),...flags};
    if(kind==='keyUp'){emit(target,'KeyboardEvent','keyup',init);return}
    if(kind!=='char'&&!emit(target,'KeyboardEvent','keydown',init))return;
    target=active.call(document)||target;
    if(kind!=='char'){
      if(init.key==='Tab'){tab(target,flags.shiftKey);return}
      if(editable(target)){
        const state=selection(target),key=init.key;
        if((flags.ctrlKey||flags.metaKey)&&key.toLowerCase()==='a'||params.commands?.includes('selectAll')){select(target,0,state.value.length);return}
        if(key==='Backspace'||key==='Delete'){edit(target,'',key==='Backspace'?'deleteContentBackward':'deleteContentForward');return}
        if(['ArrowLeft','ArrowRight','Home','End'].includes(key)){
          const at=key==='Home'?0:key==='End'?state.value.length:key==='ArrowLeft'?Math.max(0,state.start-Number(state.start===state.end)):Math.min(state.value.length,state.end+Number(state.start===state.end));
          select(target,flags.shiftKey?Math.min(state.start,at):at,flags.shiftKey?Math.max(state.end,at):at);return;
        }
      }
    }
    if((kind==='keyDown'||kind==='char')&&text){
      const code=text.charCodeAt(0),press={...init,keyCode:code,charCode:code};
      if(!emit(target,'KeyboardEvent','keypress',press))return;
      if(text==='\r'&&target.localName==='textarea')edit(target,'\n','insertLineBreak');
      else if(text!=='\r')edit(target,text);
    }
  };
  let pointTargetVersion=null;
  const pointObservationVersion=()=>host.observationVersion()+':'+(constructedStyleSheets.revision?.()||0)+':'+compatibilityElementState.observationVersion();
  const pointTargets=(x,y)=>withStyleReadCache(()=>{
    const viewport=host.viewport();
    if(!Number.isFinite(x)||!Number.isFinite(y)||x<0||y<0||x>=viewport.width||y>=viewport.height)return [];
    const elements=compatibilitySelectors.query(document,'*'),orders=new WeakMap(elements.map((e,i)=>[e,i])),scopes=new WeakMap();
    // z-index belongs to a stacking context, not an isolated element. Children
    // paint above their context's background; a high-z descendant cannot escape
    // a lower-z context. Positioned auto-z groups do not create that boundary.
    const scope=element=>{
      if(!element)return {path:[],group:null};if(scopes.has(element))return scopes.get(element);
      const parent=geometryParent(element),above=scope(parent),s=cssBoxModel.state(element),position=s.get('position')||'static',z=s.get('z-index'),order=orders.get(element)??-1;
      const parentDisplay=parent?cssBoxModel.state(parent).display:'',positioned=position!=='static';
      const context=!parent||['fixed','sticky'].includes(position)||z&&z!=='auto'&&(positioned||/flex|grid/.test(parentDisplay))||Number(s.get('opacity')??1)<1||s.get('transform')&&s.get('transform')!=='none'||s.get('isolation')==='isolate';
      const path=context&&parent?above.path.concat([[Number(z)||0,1,order,order]]):above.path,group=context?null:positioned?order:above.group;
      const rank=context?path:path.concat([[0,group===null?0:1,group??order,order]]),result={path,group,rank};scopes.set(element,result);return result;
    };
    const above=(a,b)=>{if(!b)return true;for(let i=0;i<Math.min(a.length,b.length);i++)for(let j=0;j<4;j++)if(a[i][j]!==b[i][j])return a[i][j]>b[i][j];return a.length>=b.length};
    const hits=[];
    for(const element of elements){
      const entries=computedCSSDeclarations(element),get=name=>entries.find(e=>e.name===name)?.value;
      if(get('visibility')==='hidden'||get('pointer-events')==='none')continue;
      const box=clientRectFor(element);if(box.width<=0||box.height<=0||x<box.x||x>=box.x+box.width||y<box.y||y>=box.y+box.height)continue;
      hits.push(element);
    }
    hits.sort((a,b)=>a===b?0:above(scope(a).rank,scope(b).rank)?-1:1);
    const root=document.documentElement;if(root&&!hits.includes(root))hits.push(root);
    return hits;
  });
  const pointTarget=(x,y)=>{
    const version=pointObservationVersion();
    if(pointerTarget?.isConnected&&x===pointerX&&y===pointerY&&version===pointTargetVersion)return pointerTarget;
    const target=pointTargets(x,y)[0];pointTargetVersion=version;
    return target||document.body||document.documentElement;
  };
  for(const [name,all] of [['elementFromPoint',false],['elementsFromPoint',true]]){
    Object.defineProperty(Document.prototype,name,{value:function(x,y){
      if(this!==document)throw new TypeError('Illegal invocation');
      if(arguments.length<2)throw new TypeError('Not enough arguments');
      // Hit testing belongs to the document owner. Borrowing each element's
      // geometry separately from an isolated world rebuilds the box graph for
      // every candidate and can disagree with the world's synthetic tree.
      x=Number(x);y=Number(y);
      const hits=isolated?main(null,'points',{x,y}).nodes.map(wrap):pointTargets(x,y);return all?hits:hits[0]||null;
    },writable:true,enumerable:true,configurable:true});
  }
  let pointerTarget=null,pointerX=0,pointerY=0,mouseButtons=0;
  const pressed=new Map(),buttons={none:-1,left:0,middle:1,right:2,back:3,forward:4},buttonMasks={left:1,middle:4,right:2,back:8,forward:16};
  const click=(target,init,trusted=true)=>{
    const checkable=target.localName==='input'&&['checkbox','radio'].includes(target.type),previous=checkable?control(target,'get','checked'):false;
    if(checkable)control(target,'set','checked',[target.type==='radio'?true:!previous]);
    if(!emit(target,'PointerEvent','click',init,trusted,trusted)){if(checkable)control(target,'set','checked',[previous]);return}
    let activator=target;while(activator&&activator.localName!=='button'&&activator.localName!=='a')activator=activator.parentElement;
    if(activator?.localName==='button')activateCommand(activator);
    if(checkable&&previous!==control(target,'get','checked')){emit(target,'Event','input',{bubbles:true,composed:true},true,trusted);emit(target,'Event','change',{bubbles:true},true,trusted)}
    if(activator?.localName==='a'&&activator.hasAttribute('href'))host.navigate(activator.href);
  };
  const clicking=new WeakSet();
  const syntheticClick=target=>{
    if(!target||clicking.has(target)||target.disabled)return;clicking.add(target);
    try{click(target,{bubbles:true,cancelable:true,composed:true,pointerId:-1,pointerType:'',isPrimary:false,button:0,buttons:0,detail:0},false)}finally{clicking.delete(target)}
  };
  Object.defineProperty(HTMLElement.prototype,'click',{value:function(){if(isolated){main(this,'click');return}syntheticClick(this)},writable:true,enumerable:true,configurable:true});
  const mouseCommand=(params,hintedTarget)=>{
    const x=Number(params.x),y=Number(params.y),target=hintedTarget?.isConnected?hintedTarget:pointTarget(x,y),buttonName=params.button||'none',button=buttons[buttonName]??-1,mask=buttonMasks[buttonName]||0;
    if(params.type==='mousePressed')mouseButtons|=mask;else if(params.type==='mouseReleased')mouseButtons&=~mask;
    if(params.buttons!==undefined)mouseButtons=params.buttons;
    const init={bubbles:true,cancelable:true,composed:true,clientX:x,clientY:y,screenX:x+(window.screenX||0),screenY:y+(window.screenY||0),button,buttons:mouseButtons,detail:params.clickCount||0,movementX:x-pointerX,movementY:y-pointerY,...modifiers(params.modifiers||0),pointerId:1,pointerType:'mouse',isPrimary:true,pressure:mouseButtons ? .5 : 0};
    if(target!==pointerTarget){
      if(pointerTarget){emit(pointerTarget,'PointerEvent','pointerout',{...init,relatedNode:nodeID(target)});emit(pointerTarget,'MouseEvent','mouseout',{...init,relatedNode:nodeID(target)})}
      emit(target,'PointerEvent','pointerover',{...init,relatedNode:nodeID(pointerTarget)});emit(target,'MouseEvent','mouseover',{...init,relatedNode:nodeID(pointerTarget)});pointerTarget=target;
    }
    pointerX=x;pointerY=y;pointTargetVersion=pointObservationVersion();
    const disabled=target.disabled&&['input','button','select','textarea','option'].includes(target.localName);
    if(params.type==='mouseMoved'){emit(target,'PointerEvent','pointermove',{...init,button:-1});if(!disabled)emit(target,'MouseEvent','mousemove',{...init,button:0});return}
    if(params.type==='mousePressed'){
      pressed.set(button,target);const accepted=emit(target,'PointerEvent','pointerdown',init);
      if(accepted&&!disabled&&emit(target,'MouseEvent','mousedown',init))focus(target);
      return;
    }
    emit(target,'PointerEvent','pointerup',init);if(!disabled)emit(target,'MouseEvent','mouseup',init);
    if(!disabled&&button===0&&pressed.get(button)===target)click(target,init);
    pressed.delete(button);
  };
  const legacy=globalThis.__mimicDispatchInput;
  globalThis.__mimicDispatchInput=(id,operation,raw)=>{
    if(raw===undefined)return legacy(id,operation);
    const params=parse(raw||'{}'),element=wrap(id);
    if(operation==='form')return stringify({value:control(element,params.operation,params.key,params.args)});
    if(operation==='active')return stringify({nodeID:nodeID(active.call(document))});
    if(operation==='focused')return stringify({nodeID:nodeID(originalFocused())});
    if(operation==='points')return stringify({nodes:pointTargets(params.x,params.y).map(nodeID)});
    if(operation==='rect'){
      const rect=layoutRectFor(element);
      return stringify({x:rect.x,y:rect.y,width:rect.width,height:rect.height});
    }
    if(operation==='focus'){focus(element);return '{}'}
    if(operation==='blur'){originalBlur.call(element);return '{}'}
    if(operation==='click'){syntheticClick(element);return '{}'}
    if(operation==='event'){
      const event=eventFor(params.kind,params.type,params.init);if(params.canceled)event.preventDefault();
      return stringify({allowed:dispatchEventCore(element||document.body||document.documentElement,event,params.trusted!==false,params.native!==false)});
    }
    if(operation==='key'&&['keyDown','rawKeyDown'].includes(params.type)||operation==='mouse'&&params.type==='mousePressed')host.activateProtocolInput();
    if(operation==='key')keyCommand(params);
    else if(operation==='text')edit(active.call(document),params.text);
    else if(operation==='mouse')mouseCommand(params,element);
    else throw new Error('Unsupported trusted input operation '+operation);
    return '{}';
  };
  // One box per element in the current geometry model; callers need the same
  // geometry from DOM APIs and CDP. Multi-fragment inline layout is unsupported.
  if(typeof DOMRectList==='function'){
  const rectLists=new WeakMap();
  accessor(DOMRectList.prototype,'length',function(){const list=rectLists.get(this);if(!list)throw new TypeError('Illegal invocation');return list.length});
  Object.defineProperty(DOMRectList.prototype,'item',{value:function(index){if(!arguments.length)throw new TypeError('Expected index');const list=rectLists.get(this);if(!list)throw new TypeError('Illegal invocation');return list[Number(index)>>>0]??null},writable:true,enumerable:true,configurable:true});
  Object.defineProperty(DOMRectList.prototype,Symbol.iterator,{value:function(){const list=rectLists.get(this);if(!list)throw new TypeError('Illegal invocation');return list[Symbol.iterator]()},writable:true,configurable:true});
  makeElementClientRects=element=>{
    const values=cssBoxModel.hasBox(element)?[makeDOMRect(null,element)]:[],list=Object.create(DOMRectList.prototype);
    rectLists.set(list,values);for(let i=0;i<values.length;i++)Object.defineProperty(list,i,{value:values[i],enumerable:true,configurable:true});return list;
  };
  Object.defineProperty(Element.prototype,'getClientRects',{value:function(){
    return callRealmBinding(this,requireRealmBinding(this,'ElementGeometry'),'rects',[]);
  },writable:true,enumerable:true,configurable:true});
  }
}
