(()=>{
 const proto=Object.getPrototypeOf(navigator),d=Object.getOwnPropertyDescriptor(proto,'gpu'),g=d.get;
 const attempt=fn=>{try{return fn()}catch(e){return e.name}};
 return {name:g.name,length:g.length,source:Function.prototype.toString.call(g),keys:Reflect.ownKeys(g),enumerable:d.enumerable,configurable:d.configurable,tag:String(navigator.gpu),identity:g.call(navigator)===navigator.gpu,invalid:[null,{},proto,Object.create(navigator)].map(x=>attempt(()=>g.call(x)===navigator.gpu)),construct:attempt(()=>{new g();return true})};
})()
