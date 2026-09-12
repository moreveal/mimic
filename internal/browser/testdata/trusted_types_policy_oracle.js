(() => {
  const out={}, attempt=(name,fn)=>{try{const v=fn();out[name]=v===undefined?'undefined':v}catch(e){out[name]={error:e.name,message:e.message}}};
  attempt('emptyIdentity',()=>[trustedTypes.emptyHTML===trustedTypes.emptyHTML,trustedTypes.emptyScript===trustedTypes.emptyScript]);
  attempt('emptyDefault',()=>trustedTypes.defaultPolicy===null);
  for(const name of ['allowed','allowed','other','default','default','','a b','*','none','a.b#c=1/2@x-%_'])attempt('name'+name+(out['name'+name]===undefined?'':'Duplicate'),()=>trustedTypes.createPolicy(name,{createHTML:s=>s}).name);
  attempt('optionsOrder',()=>{const seen=[],o={};for(const n of ['createHTML','createScript','createScriptURL'])Object.defineProperty(o,n,{get(){seen.push(n);return s=>s}});trustedTypes.createPolicy('allowed',o);return seen});
  attempt('callbackReceiver',()=>{const o={createScript:function(s,...a){return JSON.stringify([this===undefined,this===o,s,...a])}},p=trustedTypes.createPolicy('allowed',o);return String(p.createScript(42,'x','y'))});
  attempt('capturedCallback',()=>{const o={createHTML:s=>'old'},p=trustedTypes.createPolicy('allowed',o);o.createHTML=s=>'new';return String(p.createHTML('x'))});
  for(const [n,v] of [['null',null],['undefined',undefined],['number',42]])attempt('callback'+n,()=>String(trustedTypes.createPolicy('allowed',{createHTML:()=>v}).createHTML('x')));
  attempt('missingCallback',()=>trustedTypes.createPolicy('allowed',{}).createScript('x'));
  attempt('invalidCallback',()=>trustedTypes.createPolicy('allowed',{createHTML:42}));
  attempt('missingOptions',()=>trustedTypes.createPolicy('allowed'));
  attempt('missingInput',()=>trustedTypes.createPolicy('allowed',{createHTML:s=>s}).createHTML());
  attempt('symbolInput',()=>trustedTypes.createPolicy('allowed',{createHTML:s=>s}).createHTML(Symbol('x')));
  attempt('factoryReceiver',()=>TrustedTypePolicyFactory.prototype.isHTML.call({},trustedTypes.emptyHTML));
  attempt('valueReceiver',()=>TrustedHTML.prototype.toString.call({}));
  attempt('wrongValueReceiver',()=>TrustedHTML.prototype.toString.call(trustedTypes.emptyScript));
  attempt('toJSONOverride',()=>{const v=trustedTypes.emptyHTML;v.toString=()=> 'tampered';return v.toJSON()});
  attempt('forgedAndProxy',()=>[trustedTypes.isScript(Object.create(TrustedScript.prototype)),trustedTypes.isScript(new Proxy(trustedTypes.emptyScript,{}))]);
  attempt('mapping',()=>[
    trustedTypes.getAttributeType('script','src'),trustedTypes.getAttributeType('SCRIPT','SRC'),
    trustedTypes.getAttributeType('script','href','http://www.w3.org/2000/svg'),
    trustedTypes.getAttributeType('script','href','http://www.w3.org/2000/svg','http://www.w3.org/1999/xlink'),
    trustedTypes.getAttributeType('a','onmadeup'),trustedTypes.getAttributeType('a','onclick'),
    trustedTypes.getAttributeType('script','src','urn:other'),trustedTypes.getAttributeType('script','src',null,'urn:other'),
    trustedTypes.getPropertyType('div','innerHTML'),trustedTypes.getPropertyType('script','textContent'),
    trustedTypes.getPropertyType('a','onclick'),trustedTypes.getPropertyType('script','SRC'),
    trustedTypes.getAttributeType('object','data'),trustedTypes.getAttributeType('embed','src'),
  ]);
  for(const kind of ['TrustedHTML','TrustedScript','TrustedScriptURL','TrustedTypePolicy','TrustedTypePolicyFactory'])attempt('construct:'+kind,()=>new globalThis[kind]());
  for(const name of ['isHTML','isScript','isScriptURL'])attempt('missing:'+name,()=>trustedTypes[name]());
  attempt('mappingNamespaces',()=>[
    trustedTypes.getPropertyType('script','href','http://www.w3.org/2000/svg'),
    trustedTypes.getPropertyType('script','textContent','http://www.w3.org/2000/svg'),
    trustedTypes.getPropertyType('div','innerHTML','urn:other'),
    trustedTypes.getAttributeType('div','onclick','urn:other'),
    trustedTypes.getTypeMapping('http://www.w3.org/2000/svg'),trustedTypes.getTypeMapping('urn:other'),
  ]);
  attempt('typeMapping',()=>trustedTypes.getTypeMapping());
  return out;
})()
