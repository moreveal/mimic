(() => {
  const out = {}, run = (name, fn) => { try { out[name] = fn(); } catch (e) { out[name] = {error: e.name}; } };
  const d = document.implementation.createHTMLDocument('all');
  d.body.innerHTML = '<div id="one" name="divName"></div><input id="dup" name="inputName"><img id="dup" name="imageName"><a name="anchorName"></a><span id="01"></span><div id="item"></div>';
  const a = d.all;
  const value = x => {
    if (x === undefined) return 'undefined';
    if (x === null) return null;
    if (typeof x !== 'object' && typeof x !== 'function' && x !== d.all) return x;
    if (x.nodeType) return x.tagName + ':' + (x.id || x.getAttribute('name') || '');
    return {tag: Object.prototype.toString.call(x), length:x.length, elements:Array.from(x, n => n.tagName + ':' + (n.id || n.getAttribute('name') || ''))};
  };
  const desc = (x,k) => { const p=Object.getOwnPropertyDescriptor(x,k); if (!p) return null; return {enumerable:p.enumerable,configurable:p.configurable,writable:p.writable,get:typeof p.get,set:typeof p.set,value:'value' in p ? (typeof p.value === 'function' ? {type:'function',name:p.value.name,length:p.value.length} : value(p.value)) : undefined}; };
  run('operators', () => ({type:typeof a,boolean:!!a,equalNull:a==null,equalUndefined:a==undefined,strictUndefined:a===undefined,strictNull:a===null,tag:Object.prototype.toString.call(a),same:a===d.all,instance:a instanceof HTMLAllCollection,object:a instanceof Object,string:String(a),nullish:(a??'fallback')===a,or:a||'fallback'}));
  run('prototype',()=>({own:Reflect.ownKeys(a).map(String),prototype:Reflect.ownKeys(Object.getPrototypeOf(a)).map(String),parent:Object.getPrototypeOf(HTMLAllCollection.prototype)===Object.prototype,constructor:desc(HTMLAllCollection.prototype,'constructor'),length:desc(HTMLAllCollection.prototype,'length'),item:desc(HTMLAllCollection.prototype,'item'),namedItem:desc(HTMLAllCollection.prototype,'namedItem'),iterator:desc(HTMLAllCollection.prototype,Symbol.iterator),tag:desc(HTMLAllCollection.prototype,Symbol.toStringTag),document:desc(Document.prototype,'all')}));
  run('constructor',()=>new HTMLAllCollection());
  run('staticMethods',()=>{
    const p=HTMLAllCollection.prototype, fs={constructor:HTMLAllCollection,item:p.item,namedItem:p.namedItem,allGetter:Object.getOwnPropertyDescriptor(Document.prototype,'all').get,lengthGetter:Object.getOwnPropertyDescriptor(p,'length').get};
    return Object.fromEntries(Object.entries(fs).map(([name,f])=>{
      let constructable=true;try{Reflect.construct(function(){},[],f)}catch(e){constructable=e.name}
      const prototype=Object.getOwnPropertyDescriptor(f,'prototype');
      return [name,{keys:Reflect.ownKeys(f).map(String),name:f.name,length:f.length,constructable,prototype:prototype?{writable:prototype.writable,enumerable:prototype.enumerable,configurable:prototype.configurable,isInterfacePrototype:prototype.value===p}:null}];
    }));
  });
  run('elements',()=>value(a));
  for (const [label,args] of [['none',[]],['undefined',[undefined]],['null',[null]],['zero',[0]],['stringZero',['0']],['negative',[-1]],['fraction',[3.9]],['leadingZero',['01']],['missing',['missing']],['dup',['dup']],['dupZero',['dup',0]],['dupOne',['dup',1]],['dupMissing',['dup',8]],['one',['one']],['oneZero',['one',0]],['symbol',[Symbol()]]]) {
    run('call-'+label,()=>value(a(...args)));
    run('item-'+label,()=>value(a.item(...args)));
    run('namedItem-'+label,()=>value(a.namedItem(...args)));
  }
  run('names',()=>Object.fromEntries(['divName','inputName','imageName','anchorName','dup','item','missing','01'].map(k=>[k,{property:typeof a[k]==='function'?'function':value(a[k]),named:value(a.namedItem(k)),own:Object.hasOwn(a,k),descriptor:desc(a,k)}])));
  run('enumeration',()=>({keys:Object.keys(a),forIn:(()=>{const keys=[];for(const k in a)keys.push(k);return keys;})(),index:desc(a,'0'),missing:desc(a,'999')}));
  run('illegalReceiver',()=>HTMLAllCollection.prototype.item.call({},0));
  run('live',()=>{const n=d.createElement('p'), before=a.length; n.id='added';d.body.appendChild(n);const add=[a.length-before,a.added===n,a.item(a.length-1)===n];n.remove();return {add,remove:a.length===before,missing:a.added===undefined};});
  run('duplicateLive',()=>{const c=a.namedItem('dup'),same=c===a.namedItem('dup');const n=d.createElement('img');n.id='dup';d.body.appendChild(n);const after=c.length;n.remove();return {same,tag:Object.prototype.toString.call(c),after,removed:c.length};});
  run('nameEligibility',()=>{const q=document.implementation.createHTMLDocument('names');return Object.fromEntries(['a','applet','area','audio','button','div','embed','fieldset','form','iframe','img','input','map','meta','object','output','param','select','textarea','video','svg'].map(tag=>{const n=q.createElement(tag);n.setAttribute('name','probe');q.body.appendChild(n);const found=q.all.namedItem('probe')===n;n.remove();return [tag,found];}));});
  run('propertyMutations',()=>Object.fromEntries(['0','999','one','missing','4294967295'].map(key=>{
    const q=document.implementation.createHTMLDocument('mutation'), n=q.createElement('p');n.id='one';q.body.appendChild(n);const c=q.all;
    const set=Reflect.set(c,key,'assigned'),afterSet=c[key]==='assigned';
    const define=Reflect.defineProperty(c,key,{value:'defined',writable:true,configurable:true,enumerable:true}),afterDefine=c[key]==='defined';
    const remove=Reflect.deleteProperty(c,key),afterDelete=c[key]===undefined;
    return [key,{set,afterSet,define,afterDefine,remove,afterDelete}];
  })));
  run('expandoPrecedence',()=>{const q=document.implementation.createHTMLDocument('expando'), c=q.all;c.later='expando';const n=q.createElement('p');n.id='later';q.body.appendChild(n);const before=c.later, named=c.namedItem('later')===n;delete c.later;return {before,named,after:c.later===n};});
  run('frame',()=>{const f=document.createElement('iframe');document.body.appendChild(f);try{const w=f.contentWindow,b=w.document.all;w.remoteAll=a;return {different:b!==a,prototype:Object.getPrototypeOf(b)===w.HTMLAllCollection.prototype,notParent:!(b instanceof HTMLAllCollection),same:b===w.document.all,type:typeof b,bool:!!b,equalNull:b==null,strictUndefined:b===undefined,roundTrip:w.eval('remoteAll')===a,local:w.eval('({type:typeof document.all,bool:!!document.all,equalNull:document.all==null,strictUndefined:document.all===undefined,same:document.all===document.all,instance:document.all instanceof HTMLAllCollection})')};}finally{f.remove();}});
  return out;
})()
