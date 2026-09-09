// Stateful HTML form values. DOM attributes remain in the canonical node store;
// dirty value/checked/selected state belongs to the realm's element identity.
{
  const inputs=new WeakMap(),textareas=new WeakMap(),options=new WeakMap(),selects=new WeakMap();
  const inputState=e=>{let s=inputs.get(e);if(!s){s={dirty:false,value:'',dirtyChecked:false,checked:false,indeterminate:false,start:0,end:0,direction:'none'};inputs.set(e,s)}return s};
  const textareaState=e=>{let s=textareas.get(e);if(!s){s={dirty:false,value:'',start:0,end:0,direction:'none'};textareas.set(e,s)}return s};
  let nextOptionID=0;
  const optionState=e=>{let s=options.get(e);if(!s){s={dirty:false,selected:false,id:++nextOptionID};options.set(e,s)}return s};
  const selectState=e=>{let s=selects.get(e);if(!s){s={noSelection:false,signature:''};selects.set(e,s)}return s};
  const define=(name,key,descriptor)=>{const p=globalThis[name]?.prototype;if(p)Object.defineProperty(p,key,{...descriptor,enumerable:true,configurable:true})};
  const string=(name,key,attribute=key.toLowerCase())=>define(name,key,{get(){return this.getAttribute(attribute)||''},set(value){this.setAttribute(attribute,String(value))}});
  const boolean=(name,key,attribute=key.toLowerCase())=>define(name,key,{get(){return this.hasAttribute(attribute)},set(value){if(value)this.setAttribute(attribute,'');else this.removeAttribute(attribute)}});
  const lf=value=>String(value).replace(/\r\n?/g,'\n');
  const childText=e=>Array.from(e.childNodes).filter(node=>node.nodeType===3).map(node=>node.data).join('');
  const valueTypes=new Set(['text','search','tel','url','email','password','date','month','week','time','datetime-local','number','range','color']);
  const selectionTypes=new Set(['text','search','tel','url','password']);
  const types=new Set([...valueTypes,'hidden','checkbox','radio','file','submit','image','reset','button']);
  const typeOf=e=>{const value=(e.getAttribute('type')||'text').toLowerCase();return types.has(value)?value:'text'};
  const mode=e=>{const type=typeOf(e);return valueTypes.has(type)?'value':type==='file'?'filename':['checkbox','radio'].includes(type)?'default-on':'default'};
  function sanitize(e,value){
    const type=typeOf(e);value=String(value);
    if(['text','search','tel','password'].includes(type))return value.replace(/[\r\n]/g,'');
    if(['url','email'].includes(type)){value=value.replace(/[\r\n]/g,'').replace(/^[\t\n\f\r ]+|[\t\n\f\r ]+$/g,'');if(type==='email'&&e.multiple)value=value.split(',').map(x=>x.trim()).join(',');return value}
    if(type==='number')return /^-?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(value)&&Number.isFinite(Number(value))?value:'';
    if(type==='color')return /^#[\da-f]{6}$/i.test(value)?value.toLowerCase():'#000000';
    if(type==='range'){const min=Number(e.getAttribute('min')??0),rawMax=Number(e.getAttribute('max')??100),max=Math.max(min,rawMax);let number=value!==''&&Number.isFinite(Number(value))?Number(value):(min+max)/2;number=Math.max(min,Math.min(max,number));return String(number)}
    if(['date','time','datetime-local'].includes(type)){
      const validDate=text=>{const match=/^(\d{4,})-(\d{2})-(\d{2})$/.exec(text);if(!match||+match[1]===0)return false;const date=new Date(0);date.setUTCFullYear(+match[1],+match[2]-1,+match[3]);return date.getUTCFullYear()===+match[1]&&date.getUTCMonth()===+match[2]-1&&date.getUTCDate()===+match[3]};
      const validTime=text=>{const match=/^(\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,3}))?)?$/.exec(text);return match&&+match[1]<24&&+match[2]<60&&(match[3]===undefined||+match[3]<60)};
      if(type==='date')return validDate(value)?value:'';if(type==='time')return validTime(value)?value:'';
      const parts=value.split(/[T ]/);if(parts.length!==2||!validDate(parts[0])||!validTime(parts[1]))return '';let time=parts[1].replace(/(\.\d*?)0+$/,'$1').replace(/\.$/,'').replace(/^(\d{2}:\d{2}):00$/,'$1');return parts[0]+'T'+time;
    }
    // Month/week and stepping remain a separately measured value domain.
    return value;
  }
  const formOwner=e=>{const id=e.getAttribute('form');if(id!==null){const owner=document.getElementById(id);return owner?.localName==='form'?owner:null}for(let p=e.parentElement;p;p=p.parentElement)if(p.localName==='form')return p;return null};
  const inputValue=e=>{const s=inputState(e),m=mode(e);if(m==='filename')return '';if(m==='default-on')return e.getAttribute('value')??'on';if(m==='default')return e.getAttribute('value')??'';return sanitize(e,s.dirty?s.value:(e.getAttribute('value')??''))};
  define('HTMLInputElement','type',{get(){return typeOf(this)},set(value){const oldMode=mode(this),oldValue=inputValue(this);this.setAttribute('type',String(value));const next=mode(this),s=inputState(this);if(oldMode==='value'&&next!=='value'&&oldValue!=='')this.setAttribute('value',oldValue);if(oldMode!=='value'&&next==='value'){s.dirty=false;s.value=''}if(next==='filename'){s.dirty=false;s.value=''}}});
  define('HTMLInputElement','value',{get(){return inputValue(this)},set(value){value=value===null?'':String(value);const m=mode(this),s=inputState(this);if(m==='filename'){if(value!=='')throw new DOMException('File input value can only be cleared','InvalidStateError');s.value='';return}if(m!=='value'){this.setAttribute('value',value);return}s.dirty=true;s.value=sanitize(this,value);s.start=s.end=s.value.length;s.direction='none'}});
  string('HTMLInputElement','defaultValue','value');
  define('HTMLInputElement','checked',{get(){const s=inputState(this);return s.dirtyChecked?s.checked:this.hasAttribute('checked')},set(value){const s=inputState(this);s.dirtyChecked=true;s.checked=Boolean(value);if(s.checked&&typeOf(this)==='radio'&&this.name){const root=this.getRootNode();for(const other of root.querySelectorAll('input'))if(other!==this&&typeOf(other)==='radio'&&other.name===this.name&&formOwner(other)===formOwner(this)){const state=inputState(other);state.dirtyChecked=true;state.checked=false}}}});
  boolean('HTMLInputElement','defaultChecked','checked');
  define('HTMLInputElement','indeterminate',{get(){return inputState(this).indeterminate},set(value){inputState(this).indeterminate=Boolean(value)}});
  define('HTMLTextAreaElement','value',{get(){const s=textareaState(this);return s.dirty?s.value:lf(childText(this))},set(value){const s=textareaState(this);s.dirty=true;s.value=lf(value===null?'':value);s.start=s.end=s.value.length;s.direction='none'}});
  define('HTMLTextAreaElement','defaultValue',{get(){return childText(this)},set(value){this.textContent=String(value)}});
  define('HTMLTextAreaElement','type',{get(){return 'textarea'}});
  for(const name of ['HTMLInputElement','HTMLTextAreaElement']){
    define(name,'textLength',{get(){return this.value.length}});
    for(const property of ['selectionStart','selectionEnd','selectionDirection'])define(name,property,{get(){if(name==='HTMLInputElement'&&!selectionTypes.has(typeOf(this)))return null;const s=name==='HTMLInputElement'?inputState(this):textareaState(this);return s[property==='selectionStart'?'start':property==='selectionEnd'?'end':'direction']},set(value){const s=name==='HTMLInputElement'?inputState(this):textareaState(this);this.setSelectionRange(property==='selectionStart'?value:s.start,property==='selectionEnd'?value:s.end,property==='selectionDirection'?value:s.direction)}});
    define(name,'setSelectionRange',{value:function(start,end,direction='none'){if(name==='HTMLInputElement'&&!selectionTypes.has(typeOf(this)))throw new DOMException('Input does not support selection','InvalidStateError');const s=name==='HTMLInputElement'?inputState(this):textareaState(this);s.end=Math.min(Number(end)>>>0,this.value.length);s.start=Math.min(Number(start)>>>0,s.end);s.direction=['forward','backward'].includes(String(direction))?String(direction):'none'},writable:true});
    define(name,'select',{value:function(){if(name==='HTMLInputElement'&&!selectionTypes.has(typeOf(this)))return;this.setSelectionRange(0,this.value.length);this.dispatchEvent(new Event('select',{bubbles:true}))},writable:true});
  }
  const optionList=select=>Array.from(select.querySelectorAll('option')).filter(option=>{for(let p=option.parentElement;p;p=p.parentElement){if(p.localName==='select')return p===select}return false});
  const selectOwner=option=>{for(let p=option.parentElement;p;p=p.parentElement){if(p.localName==='select')return p}return null};
  const optionDisabled=option=>option.disabled||(option.parentElement?.localName==='optgroup'&&option.parentElement.hasAttribute('disabled'));
  function selection(select){
    const list=optionList(select),state=selectState(select),signature=list.map(option=>String(optionState(option).id)+':'+option.hasAttribute('selected')).join(',');
    if(signature!==state.signature){state.noSelection=false;state.signature=signature}
    let selected=list.filter(option=>{const s=optionState(option);return s.dirty?s.selected:option.hasAttribute('selected')});
    if(!select.multiple){if(selected.length>1)selected=selected.slice(-1);if(!selected.length&&!state.noSelection&&Number(select.size)<=1){const first=list.find(option=>!optionDisabled(option));if(first)selected=[first]}}
    return {list,selected};
  }
  function assignSelection(select,chosen){const {list}=selection(select);for(const option of list){const s=optionState(option);s.dirty=true;s.selected=chosen.includes(option)}selectState(select).noSelection=chosen.length===0}
  const collection=(get,prototype)=>new Proxy(Object.create(prototype),{get(target,key,receiver){const values=get();if(key==='length')return values.length;if(key==='item')return index=>values[Number(index)]??null;if(key==='namedItem')return name=>values.find(e=>e.id===String(name)||e.getAttribute('name')===String(name))??null;if(key===Symbol.iterator)return values[Symbol.iterator].bind(values);if(typeof key==='string'&&/^\d+$/.test(key))return values[Number(key)];return Reflect.get(target,key,receiver)}});
  define('HTMLOptionElement','text',{get(){return this.textContent.replace(/[\t\n\f\r ]+/g,' ').trim()},set(value){this.textContent=String(value)}});
  define('HTMLOptionElement','value',{get(){return this.getAttribute('value')??this.text},set(value){this.setAttribute('value',String(value))}});
  define('HTMLOptionElement','label',{get(){return this.getAttribute('label')??this.text},set(value){this.setAttribute('label',String(value))}});
  boolean('HTMLOptionElement','defaultSelected','selected');boolean('HTMLOptionElement','disabled');
  define('HTMLOptionElement','selected',{get(){const owner=selectOwner(this);if(owner)return selection(owner).selected.includes(this);const s=optionState(this);return s.dirty?s.selected:this.hasAttribute('selected')},set(value){const owner=selectOwner(this),s=optionState(this);s.dirty=true;s.selected=Boolean(value);if(owner){if(s.selected&&!owner.multiple)assignSelection(owner,[this]);else selectState(owner).noSelection=false}}});
  define('HTMLOptionElement','index',{get(){const owner=selectOwner(this);return owner?optionList(owner).indexOf(this):0}});
  define('HTMLSelectElement','type',{get(){return this.multiple?'select-multiple':'select-one'}});
  define('HTMLSelectElement','size',{get(){const value=Number(this.getAttribute('size'));return Number.isInteger(value)&&value>=0?value:0},set(value){this.setAttribute('size',String(Number(value)>>>0))}});
  define('HTMLSelectElement','options',{get(){const owner=this,base=collection(()=>optionList(owner),globalThis.HTMLOptionsCollection?.prototype||globalThis.HTMLCollection.prototype);return new Proxy(base,{get(target,key,receiver){if(key==='selectedIndex')return owner.selectedIndex;if(key==='add')return (...args)=>owner.add(...args);if(key==='remove')return index=>owner.remove(index);return Reflect.get(target,key,receiver)},set(target,key,value){if(key==='selectedIndex'){owner.selectedIndex=value;return true}if(key==='length'){owner.length=value;return true}return Reflect.set(target,key,value)}})}});
  define('HTMLSelectElement','selectedOptions',{get(){return collection(()=>selection(this).selected,globalThis.HTMLCollection.prototype)}});
  define('HTMLSelectElement','length',{get(){return optionList(this).length},set(value){value=Number(value)>>>0;if(value>100000)throw new DOMException('Too many options','IndexSizeError');const list=optionList(this);while(list.length>value)list.pop().remove();while(list.length<value){const option=document.createElement('option');this.appendChild(option);list.push(option)}}});
  define('HTMLSelectElement','selectedIndex',{get(){const s=selection(this);return s.selected.length?s.list.indexOf(s.selected[0]):-1},set(value){const list=optionList(this),index=Number(value)|0;assignSelection(this,index>=0&&index<list.length?[list[index]]:[])}});
  define('HTMLSelectElement','value',{get(){return selection(this).selected[0]?.value??''},set(value){const found=optionList(this).find(option=>option.value===String(value));assignSelection(this,found?[found]:[])}});
  define('HTMLSelectElement','item',{value:function(index){return optionList(this)[Number(index)]??null},writable:true});
  define('HTMLSelectElement','add',{value:function(element,before){if(!['option','optgroup'].includes(element?.localName))throw new TypeError('Expected option or optgroup');if(typeof before==='number')before=optionList(this)[before]??null;if(before)before.parentNode.insertBefore(element,before);else this.appendChild(element)},writable:true});
  const removeElement=globalThis.Element.prototype.remove;
  define('HTMLSelectElement','remove',{value:function(index){if(arguments.length===0)return removeElement.call(this);const option=optionList(this)[Number(index)|0];if(option)option.remove()},writable:true});
  for(const name of ['HTMLInputElement','HTMLTextAreaElement','HTMLSelectElement','HTMLButtonElement']){
    string(name,'name');boolean(name,'disabled');boolean(name,'required');define(name,'form',{get(){return formOwner(this)}});
  }
  for(const name of ['HTMLInputElement','HTMLTextAreaElement']){string(name,'placeholder');boolean(name,'readOnly','readonly')}
  for(const name of ['HTMLInputElement','HTMLSelectElement'])boolean(name,'multiple');
  const associated=form=>Array.from(document.querySelectorAll('input,textarea,select,button')).filter(e=>formOwner(e)===form);
  define('HTMLFormElement','elements',{get(){return collection(()=>associated(this),globalThis.HTMLFormControlsCollection?.prototype||globalThis.HTMLCollection.prototype)}});
  define('HTMLFormElement','length',{get(){return associated(this).length}});
  define('HTMLFormElement','reset',{value:function(){if(!this.dispatchEvent(new Event('reset',{bubbles:true,cancelable:true})))return;for(const control of associated(this)){
    if(control.localName==='input'){const s=inputState(control);s.dirty=false;s.value='';s.dirtyChecked=false;s.checked=false}
    if(control.localName==='textarea'){const s=textareaState(control);s.dirty=false;s.value=''}
    if(control.localName==='select'){for(const option of optionList(control)){const s=optionState(option);s.dirty=false;s.selected=false}selectState(control).noSelection=false}
  }},writable:true});
}
