// Shared Window/Worker Trusted Types model. The private binding brand travels
// through the existing realm reference bridge; public prototypes never confer
// trust, and values keep their original object identity across realms.
const trustedNativeString=String,trustedObjectCreate=Object.create;
const trustedRegExpTest=Function.prototype.call.bind(RegExp.prototype.test);
const trustedString=value=>{if(typeof value==='symbol')throw new TypeError('Cannot convert a Symbol value to a string');return trustedNativeString(value)};
const trustedIncludes=Function.prototype.call.bind(Array.prototype.includes);
const trustedSetHas=Function.prototype.call.bind(Set.prototype.has),trustedSetAdd=Function.prototype.call.bind(Set.prototype.add);
const trustedKinds=['TrustedHTML','TrustedScript','TrustedScriptURL'];
const trustedMethods=['createHTML','createScript','createScriptURL'];
const trustedApply=Reflect.apply;
const trustedTypeOf=value=>{
  const binding=bindingGet(value)||referenceGet(value)?.binding;
  const kind=binding?.kind;return kind==='TrustedHTML'||kind==='TrustedScript'||kind==='TrustedScriptURL'?kind:null;
};
const trustedSource=(value,kind)=>callRealmBinding(value,requireRealmBinding(value,kind),'source',[]);
const trustedValue=(Ctor,text)=>{
  const value=trustedObjectCreate(Ctor.prototype);
  registerRealmBinding(value,Ctor===TrustedHTML?'TrustedHTML':Ctor===TrustedScript?'TrustedScript':'TrustedScriptURL',{source:()=>text});
  return value;
};
class TrustedHTML {
  constructor(){throw new TypeError('Illegal constructor')}
  toString(){return trustedSource(this,'TrustedHTML')}
  toJSON(){return trustedSource(this,'TrustedHTML')}
}
class TrustedScript {
  constructor(){throw new TypeError('Illegal constructor')}
  toString(){return trustedSource(this,'TrustedScript')}
  toJSON(){return trustedSource(this,'TrustedScript')}
}
class TrustedScriptURL {
  constructor(){throw new TypeError('Illegal constructor')}
  toString(){return trustedSource(this,'TrustedScriptURL')}
  toJSON(){return trustedSource(this,'TrustedScriptURL')}
}
const trustedConstructors={TrustedHTML,TrustedScript,TrustedScriptURL};
const trustedPolicySlots=new WeakMap(),trustedPolicyGet=WeakMap.prototype.get.bind(trustedPolicySlots);
const trustedFactorySlots=new WeakMap(),trustedFactoryGet=WeakMap.prototype.get.bind(trustedFactorySlots);
const trustedPolicyInvoke=(receiver,method,input,args,count)=>{
  const binding=requireRealmBinding(receiver,'TrustedTypePolicy');
  const prefix="Failed to execute '"+method+"' on 'TrustedTypePolicy': ";
  if(!count)throw new TypeError(prefix+'1 argument required, but only 0 present.');
  if(typeof input==='symbol')throw new TypeError(prefix+'Cannot convert a Symbol value to a string');
  input=trustedString(input);
  return callRealmBinding(receiver,binding,method,[input,...args]);
};
class TrustedTypePolicy {
  constructor(token,name,options){
    if(token!==hostToken)throw new TypeError('Illegal constructor');
    trustedPolicySlots.set(this,{name,options});
    const operations={name:()=>name};
    for(let i=0;i<3;i++){
      const method=trustedMethods[i],kind=trustedKinds[i],callback=options[method];
      operations[method]=(input,...args)=>{
        if(!callback)throw new TypeError("Failed to execute '"+method+"' on 'TrustedTypePolicy': Policy "+name+"'s TrustedTypePolicyOptions did not specify a '"+method+"' member.");
        const result=trustedApply(callback,undefined,[input,...args]);
        return trustedValue(trustedConstructors[kind],result==null?'':trustedString(result));
      };
    }
    registerRealmBinding(this,'TrustedTypePolicy',operations);
  }
  get name(){return callRealmBinding(this,requireRealmBinding(this,'TrustedTypePolicy'),'name',[])}
  createHTML(input,...args){return trustedPolicyInvoke(this,'createHTML',input,args,arguments.length)}
  createScript(input,...args){return trustedPolicyInvoke(this,'createScript',input,args,arguments.length)}
  createScriptURL(input,...args){return trustedPolicyInvoke(this,'createScriptURL',input,args,arguments.length)}
}
let trustedDefaultFactory;
const trustedEventAttributes=new Set(host.trustedTypesEventAttributes?host.trustedTypesEventAttributes():[]);
// Event handler attributes use the captured platform surface, not arbitrary
// on-prefixed attributes or application-added properties.
const trustedCaptureEvents=()=>{
  if(trustedEventAttributes.size)return;
  for(const name of ['HTMLElement','SVGElement','MathMLElement','Window']){
    let prototype=globalThis[name]?.prototype;
    while(prototype&&prototype!==Object.prototype){for(const key of Object.getOwnPropertyNames(prototype))if(key.startsWith('on'))trustedEventAttributes.add(key);prototype=Object.getPrototypeOf(prototype)}
  }
};
const trustedAttributeInfo=(tag,name,ns='http://www.w3.org/1999/xhtml',attrNS='')=>{
  ns=ns||'http://www.w3.org/1999/xhtml';tag=tag.toLowerCase();name=name.toLowerCase();
  const html=ns==='http://www.w3.org/1999/xhtml',svg=ns==='http://www.w3.org/2000/svg';
  if((html||svg||ns==='http://www.w3.org/1998/Math/MathML')&&!attrNS&&trustedSetHas(trustedEventAttributes,name))return ['TrustedScript','Element '+name];
  if(html&&!attrNS){
    if(tag==='script'&&name==='src')return ['TrustedScriptURL','HTMLScriptElement src'];
    if(tag==='iframe'&&name==='srcdoc')return ['TrustedHTML','HTMLIFrameElement srcdoc'];
    if(tag==='object'&&name==='data')return ['TrustedScriptURL','HTMLObjectElement data'];
    if(tag==='object'&&name==='codebase')return ['TrustedScriptURL','HTMLObjectElement codeBase'];
    if(tag==='embed'&&name==='src')return ['TrustedScriptURL','HTMLEmbedElement src'];
  }
  if(svg&&tag==='script'&&name==='href'&&(!attrNS||attrNS==='http://www.w3.org/1999/xlink'))return ['TrustedScriptURL','SVGScriptElement href'];
  return null;
};
class TrustedTypePolicyFactory {
  constructor(token){
    if(token!==hostToken)throw new TypeError('Illegal constructor');
    const state={defaultPolicy:null,names:new Set(),emptyHTML:trustedValue(TrustedHTML,''),emptyScript:trustedValue(TrustedScript,'')};
    trustedFactorySlots.set(this,state);trustedDefaultFactory=this;
    registerRealmBinding(this,'TrustedTypePolicyFactory',{
      emptyHTML:()=>state.emptyHTML,emptyScript:()=>state.emptyScript,defaultPolicy:()=>state.defaultPolicy,
      createPolicy:(name,options)=>{
        const prefix="Failed to execute 'createPolicy' on 'TrustedTypePolicyFactory': ";
        const policy=host.trustedTypesPolicy();
        for(const rules of policy.rules){
          if(!trustedRegExpTest(/^[a-zA-Z0-9\-#=_/@.%]*$/,name)||!trustedIncludes(rules,name)&&!trustedIncludes(rules,'*'))throw new TypeError(prefix+'Policy "'+name+'" disallowed.');
          if(trustedSetHas(state.names,name)&&!trustedIncludes(rules,"'allow-duplicates'"))throw new TypeError(prefix+'Policy with name "'+name+'" already exists.');
        }
        if(name==='default'&&state.defaultPolicy)throw new TypeError(prefix+'Policy with name "default" already exists.');
        const result=new TrustedTypePolicy(hostToken,name,options);trustedSetAdd(state.names,name);
        if(name==='default')state.defaultPolicy=result;
        return result;
      }
    });
  }
  createPolicy(name,options={}){
    const binding=requireRealmBinding(this,'TrustedTypePolicyFactory');
    if(!arguments.length)throw new TypeError("Failed to execute 'createPolicy' on 'TrustedTypePolicyFactory': 1 argument required, but only 0 present.");
    name=trustedString(name);
    if(options!=null&&!['object','function'].includes(typeof options))throw new TypeError("Failed to execute 'createPolicy' on 'TrustedTypePolicyFactory': The provided value is not of type 'TrustedTypePolicyOptions'.");
    const captured={};
    for(const method of trustedMethods){const callback=options?.[method];if(callback!==undefined&&typeof callback!=='function')throw new TypeError("Failed to execute 'createPolicy' on 'TrustedTypePolicyFactory': Failed to read the '"+method+"' property from 'TrustedTypePolicyOptions': The given value is not a function.");captured[method]=callback}
    return callRealmBinding(this,binding,'createPolicy',[name,captured]);
  }
  isHTML(value){requireRealmBinding(this,'TrustedTypePolicyFactory');if(!arguments.length)throw new TypeError("Failed to execute 'isHTML' on 'TrustedTypePolicyFactory': 1 argument required, but only 0 present.");return trustedTypeOf(value)==='TrustedHTML'}
  isScript(value){requireRealmBinding(this,'TrustedTypePolicyFactory');if(!arguments.length)throw new TypeError("Failed to execute 'isScript' on 'TrustedTypePolicyFactory': 1 argument required, but only 0 present.");return trustedTypeOf(value)==='TrustedScript'}
  isScriptURL(value){requireRealmBinding(this,'TrustedTypePolicyFactory');if(!arguments.length)throw new TypeError("Failed to execute 'isScriptURL' on 'TrustedTypePolicyFactory': 1 argument required, but only 0 present.");return trustedTypeOf(value)==='TrustedScriptURL'}
  get emptyHTML(){return callRealmBinding(this,requireRealmBinding(this,'TrustedTypePolicyFactory'),'emptyHTML',[])}
  get emptyScript(){return callRealmBinding(this,requireRealmBinding(this,'TrustedTypePolicyFactory'),'emptyScript',[])}
  get defaultPolicy(){return callRealmBinding(this,requireRealmBinding(this,'TrustedTypePolicyFactory'),'defaultPolicy',[])}
  getAttributeType(tag,name,ns='',attrNS=''){
    requireRealmBinding(this,'TrustedTypePolicyFactory');if(arguments.length<2)throw new TypeError('Not enough arguments');
    tag=trustedString(tag);name=trustedString(name);ns=ns==null?'':trustedString(ns);attrNS=attrNS==null?'':trustedString(attrNS);
    if(ns&&ns!=='http://www.w3.org/1999/xhtml'&&ns!=='http://www.w3.org/2000/svg')return null;
    return trustedAttributeInfo(tag,name,ns,attrNS)?.[0]||null;
  }
  getPropertyType(tag,name,ns=''){
    requireRealmBinding(this,'TrustedTypePolicyFactory');if(arguments.length<2)throw new TypeError('Not enough arguments');
    tag=trustedString(tag).toLowerCase();name=trustedString(name);ns=ns==null?'':trustedString(ns);
    if(name==='innerHTML'||name==='outerHTML')return 'TrustedHTML';
    if(ns==='http://www.w3.org/2000/svg'&&tag==='script'&&name==='href')return 'TrustedScriptURL';
    if(ns&&ns!=='http://www.w3.org/1999/xhtml')return null;
    if(tag==='script'&&['text','textContent','innerText'].includes(name))return 'TrustedScript';
    if(tag==='script'&&name==='src'||tag==='object'&&['data','codeBase'].includes(name)||tag==='embed'&&name==='src')return 'TrustedScriptURL';
    if(tag==='iframe'&&name==='srcdoc')return 'TrustedHTML';
    return null;
  }
  getTypeMapping(ns=''){
    requireRealmBinding(this,'TrustedTypePolicyFactory');ns=trustedString(ns);
    if(ns&&ns!=='http://www.w3.org/1999/xhtml')return null;
    const attributes={};for(const name of trustedEventAttributes)attributes[name]='TrustedScript';
    return {'*':{attributes,properties:{innerHTML:'TrustedHTML',outerHTML:'TrustedHTML'}},embed:{attributes:{src:'TrustedScriptURL'},properties:{src:'TrustedScriptURL'}},iframe:{attributes:{srcdoc:'TrustedHTML'},properties:{srcdoc:'TrustedHTML'}},object:{attributes:{data:'TrustedScriptURL',codebase:'TrustedScriptURL'},properties:{data:'TrustedScriptURL',codeBase:'TrustedScriptURL'}},script:{attributes:{href:'TrustedScriptURL',src:'TrustedScriptURL'},properties:{href:'TrustedScriptURL',src:'TrustedScriptURL',text:'TrustedScript',innerText:'TrustedScript',textContent:'TrustedScript'}}};
  }
}
// Chrome labels these generated-source prefixes as Function, even when the
// same text is passed to eval. This is an observed string classification, not a
// replacement of native Function/eval: lexical scope and constructor identity
// remain entirely engine-owned.
const trustedStartsWith=Function.prototype.call.bind(String.prototype.startsWith);
const trustedCodeSink=source=>(trustedStartsWith(source,'(function anonymous')||trustedStartsWith(source,'(function* anonymous')||trustedStartsWith(source,'(async function anonymous')||trustedStartsWith(source,'(async function* anonymous'))?'Function':'eval';
const trustedEnforceString=(text,kind,sink,prefix,code=false)=>{
  const state=host.trustedTypesPolicy();if(!state.required)return text;
  const policy=trustedFactoryGet(trustedDefaultFactory)?.defaultPolicy;
  const callback=policy&&trustedPolicyGet(policy).options[(kind==='TrustedHTML'?'createHTML':kind==='TrustedScript'?'createScript':'createScriptURL')];
  if(callback){
    const result=trustedApply(callback,undefined,[text,kind,sink]);
    if(result!=null){const converted=trustedString(result);if(code&&converted!==text)throw new EvalError("Evaluating a string as JavaScript violates this document's Trusted Type assignment requirements.");return converted}
  }
  if(!state.enforced)return text;
  if(code)throw new EvalError("Evaluating a string as JavaScript violates this document's Trusted Type assignment requirements.");
  throw new TypeError(prefix+"This document requires '"+kind+"' assignment"+(policy?" and the 'default' policy failed to execute.":'.'));
};
const trustedConvert=(value,kind,sink,prefix,owner=null)=>{
  if(trustedTypeOf(value)===kind)return trustedSource(value,kind);
  const text=trustedString(value);
  if(owner){const slot=elementSlot(owner);if(slot){const remote=host.trustedTypesOwner(slot.nodeId);if(remote)return unwrapCrossRealm(remote.frame,remote)(text,kind,sink,prefix)}}
  return trustedEnforceString(text,kind,sink,prefix);
};
// The engine invokes this for native eval and every dynamic Function kind.
// Non-string eval preserves identity, including boxed strings and wrong kinds.
const evalSourceResolver=(value,isCodeLike=false)=>{
  const sink=typeof value==='string'?trustedCodeSink(value):'eval';
  const kind=trustedTypeOf(value);
  if(typeof value!=='string'&&kind!=='TrustedScript')return undefined;
  let source,failed=false;
  try{source=kind==='TrustedScript'?trustedSource(value,kind):trustedEnforceString(value,'TrustedScript',sink,'',true)}
  catch{failed=true}
  const blocked=host.trustedTypesPolicy().evalBlocked;
  if(blocked)throw new EvalError("Evaluating a string as JavaScript violates the following Content Security Policy directive because 'unsafe-eval' is not an allowed source of script: "+blocked+'".\n');
  if(failed)throw new EvalError("Evaluating a string as JavaScript violates this document's Trusted Type assignment requirements.");
  return source;
};

const trustedTimerArgument=handler=>typeof handler==='function'||trustedTypeOf(handler)==='TrustedScript'?handler:trustedString(handler);
