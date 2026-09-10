// Font collection ownership and descriptor state; no native font backend.
// Loading a usable font requires a font resource model shared with text metrics.
// Until that exists, reject explicitly rather than claiming successful loading.
(()=>{
 if(typeof globalThis.FontFace!=='function'||typeof globalThis.FontFaceSet!=='function')return;
 const faces=new WeakMap(),sets=new WeakMap(),owners=new WeakMap();
 const syntax=()=>new DOMException('Invalid font descriptor','SyntaxError');
 const unsupported=()=>{host.semanticMissingAt('font_faces.js:8','FontFace.fontResourceLoading');return new DOMException('Font resource loading is not supported','NotSupportedError')};
 const requireFace=value=>{const s=faces.get(value);if(!s)throw new TypeError('Illegal invocation');return s};
 const requireSet=value=>{const s=sets.get(value);if(!s)throw new TypeError('Illegal invocation');return s};
 const defaults={style:'normal',weight:'normal',stretch:'normal',unicodeRange:'U+0-10FFFF',variant:'normal',featureSettings:'normal',variationSettings:'normal',display:'auto',ascentOverride:'normal',descentOverride:'normal',lineGapOverride:'normal',sizeAdjust:'100%'};
 const percent=v=>/^\d+(?:\.\d+)?%$/.test(v);
 function descriptor(key,value){
  value=String(value).trim().replace(/\s+/g,' ');let valid=false;
  if(key==='style')valid=/^(normal|italic|oblique(?: -?\d+(?:\.\d+)?deg){0,2})$/.test(value);
  else if(key==='weight')valid=/^(normal|bold)$/.test(value)||value.split(' ').length<=2&&value.split(' ').every(v=>/^\d+(?:\.\d+)?$/.test(v)&&Number(v)>=1&&Number(v)<=1000);
  else if(key==='stretch')valid=/^(normal|(?:ultra-|extra-|semi-)?(?:condensed|expanded))$/.test(value)||value.split(' ').length<=2&&value.split(' ').every(percent);
  else if(key==='display')valid=/^(auto|block|swap|fallback|optional)$/.test(value);
  else if(key==='sizeAdjust')valid=percent(value);
  else if(['ascentOverride','descentOverride','lineGapOverride'].includes(key))valid=value==='normal'||percent(value);
  else if(key==='variant')valid=/^(normal|small-caps)$/.test(value);
  else if(key==='featureSettings'||key==='variationSettings')valid=value==='normal'||value.split(',').every(v=>key==='featureSettings'?/^\s*["'][\x20-\x7e]{4}["'](?: (?:\d+|on|off))?\s*$/.test(v):/^\s*["'][\x20-\x7e]{4}["'] -?\d+(?:\.\d+)?\s*$/.test(v));
  else if(key==='unicodeRange'){
   return value.split(',').map(v=>{const m=/^U\+([0-9A-F?]{1,6})(?:-([0-9A-F]{1,6}))?$/i.exec(v.trim());if(!m||m[2]&&m[1].includes('?')||/\?[0-9a-f]/i.test(m[1]))throw syntax();const start=parseInt(m[1].replace(/\?/g,'0'),16),end=parseInt(m[2]||m[1].replace(/\?/g,'F'),16);if(start>end||end>0x10ffff)throw syntax();return 'U+'+start.toString(16).toUpperCase()+(end===start?'':'-'+end.toString(16).toUpperCase())}).join(', ');
  }
  if(!valid)throw syntax();return value;
 }
 const familyName=value=>/^[-_a-zA-Z][-_a-zA-Z0-9]*$/.test(value)&&! /^(serif|sans-serif|monospace|cursive|fantasy|system-ui|inherit|initial|unset|revert|default)$/i.test(value)?value:JSON.stringify(value);
 const OriginalFontFace=globalThis.FontFace;
 function FontFace(family,source,descriptors={}){
  if(!new.target||arguments.length<2)throw new TypeError('Expected family and source');
  const object=Object.create(new.target.prototype);let reject;
  const s={family:familyName(String(family)),...defaults,status:'unloaded',loaded:new Promise((a,b)=>{reject=b})};
  faces.set(object,s);s.reject=reject;
  // Internal rejection handling does not change the public promise identity.
  s.loaded.catch(()=>{});
  try{
   if(descriptors!==null&&typeof descriptors!=='object'&&typeof descriptors!=='function')throw new TypeError('Expected descriptor dictionary');
   for(const key of Object.keys(defaults))if(descriptors?.[key]!==undefined)s[key]=descriptor(key,descriptors[key]);
   if(ArrayBuffer.isView(source)||source instanceof ArrayBuffer){
    const bytes=ArrayBuffer.isView(source)?new Uint8Array(source.buffer,source.byteOffset,source.byteLength):new Uint8Array(source);
    // Every supported sfnt/WOFF container needs more than this prefix. A valid
    // prefix is not sufficient evidence of a decoded, usable font.
    if(bytes.length<12)throw syntax();throw unsupported();
   }
   s.source=String(source).trim();
   if(!/^(local|url)\(/i.test(s.source)||!s.source.endsWith(')'))throw syntax();
  }catch(e){if(e instanceof TypeError)throw e;s.status='error';reject(e)}
  return object;
 }
 FontFace.prototype=OriginalFontFace.prototype;
 Object.defineProperty(FontFace.prototype,'constructor',{value:FontFace,writable:true,configurable:true});
 Object.defineProperty(globalThis,'FontFace',{value:FontFace,writable:true,configurable:true});
 for(const key of ['family',...Object.keys(defaults),'status','loaded']){
  const d={get(){return requireFace(this)[key]},configurable:true,enumerable:true};
  if(key==='family'||key in defaults)d.set=function(value){const s=requireFace(this);s[key]=key==='family'?String(value):descriptor(key,value)};
  Object.defineProperty(FontFace.prototype,key,d);
 }
 Object.defineProperty(FontFace.prototype,'load',{value:function load(){try{const s=requireFace(this);if(s.status==='unloaded'){s.status='error';s.reject(unsupported())}return s.loaded}catch(e){return Promise.reject(e)}},writable:true,configurable:true,enumerable:true});
 function makeSet(owner){const value=new EventTarget();Object.setPrototypeOf(value,FontFaceSet.prototype);const state={values:new Set(),ready:typeof document==='object'&&owner===document?Promise.resolve(value):new Promise(()=>{})};sets.set(value,state);return value}
 for(const [key,get]of Object.entries({size:s=>s.values.size,status:()=> 'loaded',ready:s=>s.ready}))Object.defineProperty(FontFaceSet.prototype,key,{get(){return get(requireSet(this))},configurable:true,enumerable:true});
 const methods={
  add(face){const s=requireSet(this);requireFace(face);s.values.add(face);return this},
  delete(face){const s=requireSet(this);requireFace(face);return s.values.delete(face)},
  has(face){const s=requireSet(this);requireFace(face);return s.values.has(face)},
  clear(){requireSet(this).values.clear()},
  keys(){return requireSet(this).values.keys()},values(){return requireSet(this).values.values()},entries(){return requireSet(this).values.entries()},
  forEach(callback,thisArg){const s=requireSet(this);if(typeof callback!=='function')throw new TypeError('Expected callback');s.values.forEach(v=>callback.call(thisArg,v,v,this))},
  check(font,text=' '){const s=requireSet(this);if(arguments.length<1)throw new TypeError('Expected font');parseFont(font);String(text);if(s.values.size)throw unsupported();return true},
  load(font,text=' '){try{const s=requireSet(this);if(arguments.length<1)throw new TypeError('Expected font');parseFont(font);String(text);if(s.values.size)throw unsupported();return Promise.resolve([])}catch(e){return Promise.reject(e)}}
 };
 // A bounded shorthand grammar. More complex CSS must not be silently accepted.
 function parseFont(value){value=String(value).trim();if(!/^(?:(?:normal|italic|oblique|small-caps|bold|bolder|lighter|[1-9]\d{0,2})\s+)*(?:\d+(?:\.\d+)?(?:px|pt|em|rem|%)|(?:xx?-small|small|medium|large|xx?-large))(?:\s*\/\s*(?:normal|\d+(?:\.\d+)?(?:px|pt|em|rem|%)?))?\s+(?:["'][^"']+["']|[-_a-zA-Z][-_a-zA-Z0-9 ]*)(?:\s*,\s*(?:["'][^"']+["']|[-_a-zA-Z][-_a-zA-Z0-9 ]*))*$/.test(value))throw syntax()}
 for(const [name,value]of Object.entries(methods))Object.defineProperty(FontFaceSet.prototype,name,{value,writable:true,configurable:true,enumerable:true});
 Object.defineProperty(FontFaceSet.prototype,Symbol.iterator,{value:methods.values,writable:true,configurable:true});
 function owned(owner){let value=owners.get(owner);if(!value){value=makeSet(owner);owners.set(owner,value)}return value}
 if(typeof globalThis.Document==='function')Object.defineProperty(Document.prototype,'fonts',{get(){if(!(this instanceof Document))throw new TypeError('Illegal invocation');return owned(this)},configurable:true,enumerable:true});
 else Object.defineProperty(globalThis,'fonts',{get(){return owned(globalThis)},configurable:true,enumerable:true});
 if(typeof markNative==='function'){
  markNative(FontFace,'FontFace');
  for(const prototype of [FontFace.prototype,FontFaceSet.prototype])for(const key of Object.getOwnPropertyNames(prototype)){
   if(key==='constructor')continue;
   const d=Object.getOwnPropertyDescriptor(prototype,key);
   if(typeof d.value==='function')markNative(d.value,key);
   if(d.get)markNative(d.get,key,'get ');if(d.set)markNative(d.set,key,'set ');
  }
 }
})();
