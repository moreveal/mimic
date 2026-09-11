// Run on the fixture origin listed in result.json. Returns a Promise.
(async () => {
  let value = await (async () => {

const tag = v => Object.prototype.toString.call(v);
const error = e => {
 const message = String(e && e.message || e);
 let category = /illegal invocation|incompatible receiver|not of type|called on|does not implement/i.test(message) ? 'illegal-receiver' :
 /denied|cross-origin|blocked a frame/i.test(message) ? 'security' :
 /argument|parameter|convert|Symbol/i.test(message) ? 'argument' :
 /not defined/i.test(message) ? 'not-defined' :
 /not a function/i.test(message) ? 'not-callable' : 'other';
 return {exception: String(e && e.name || 'Error'), class: tag(e), messageCategory:category};
};
const scalar = v => {
 if (v === undefined) return {type:'undefined'};
 if (v === null) return {type:'object',value:null};
 const t = typeof v;
 if (t === 'number') return {type:t,value:Number.isNaN(v)?'NaN':v===Infinity?'Infinity':v===-Infinity?'-Infinity':Object.is(v,-0)?'-0':v};
 if (t === 'symbol' || t === 'bigint') return {type:t,value:String(v)};
 if (t === 'string' || t === 'boolean') return {type:t,value:v};
 return {type:t,tag:tag(v)};
};
const attempt = fn => {try{return {ok:scalar(fn())};}catch(e){return error(e);}};
const key = k => typeof k === 'symbol' ? {symbol:String(k)} : k;
const inspect = v => ({
 type:typeof v, strictUndefined:v===undefined, looseUndefined:v==undefined,
 strictNull:v===null, looseNull:v==null, boolean:Boolean(v),
 primitive:scalar(v), tag:attempt(()=>tag(v)),
 prototype:attempt(()=>Object.getPrototypeOf(v)),
 prototypeIsConstructorPrototype:attempt(()=>Object.getPrototypeOf(v)===v.constructor.prototype),
 constructor:attempt(()=>v.constructor), constructorName:attempt(()=>v.constructor.name),
 constructorIsGlobal:attempt(()=>v.constructor===globalThis[v.constructor.name]),
 ownKeys:(()=>{try{return Reflect.ownKeys(v).map(key);}catch(e){return error(e);}})(),
 functionShape:typeof v==='function'?{name:attempt(()=>v.name),length:attempt(()=>v.length),source:attempt(()=>Function.prototype.toString.call(v))}:null
});
const descriptor = (o,k) => {
 let depth=0;
 for(let p=o;p!==null;p=Object.getPrototypeOf(p),depth++){
  const d=Object.getOwnPropertyDescriptor(p,k);
  if(d){const result={depth,configurable:d.configurable,enumerable:d.enumerable};
   if('value' in d){result.kind='data';result.writable=d.writable;result.value=scalar(d.value);}
   else {result.kind='accessor';result.get=scalar(d.get);result.set=scalar(d.set);}
   return result;
  }
 }
 return null;
};

try {

return {status:'ok',value:await (inspect(document))};
} catch(e) {return {status:'exception',error:error(e)};}
})();
  for (const key of ["value", "constructorName", "ok", "value"]) {
    if (value === null || value === undefined || !Object.prototype.hasOwnProperty.call(value, key)) return {"$missing":true};
    value = value[key];
  }
  return value;
})()
