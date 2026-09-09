  const nativeFunctionText=new WeakMap();
  const nativeFunctionTextGet=WeakMap.prototype.get.bind(nativeFunctionText),nativeFunctionTextSet=WeakMap.prototype.set.bind(nativeFunctionText),functionSourceApply=Reflect.apply;
  const engineFunctionToString=Function.prototype.toString;
  // Only explicitly marked platform functions override the engine's source.
  // Reading callable properties here invokes user getters/proxy traps, and
  // inspecting source text also misclassifies ordinary user function bodies.
  Function.prototype.toString=new Proxy(engineFunctionToString,{apply(target,thisArg,args){const native=nativeFunctionTextGet(thisArg);return native||functionSourceApply(target,thisArg,args)}});
  const markNative=(fn,name,prefix='')=>{if(typeof fn==='function')nativeFunctionTextSet(fn,'function '+prefix+String(name)+'() { [native code] }')};
