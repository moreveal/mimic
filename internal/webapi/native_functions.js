  const nativeFunctionNames=new WeakMap();
  const nativeFunctionNameGet=WeakMap.prototype.get.bind(nativeFunctionNames),nativeFunctionNameSet=WeakMap.prototype.set.bind(nativeFunctionNames),functionSourceApply=Reflect.apply;
  const engineFunctionToString=Function.prototype.toString;
  // Only explicitly marked platform functions override the engine's source.
  // Reading callable properties here invokes user getters/proxy traps, and
  // inspecting source text also misclassifies ordinary user function bodies.
  // Keep only the binding name during bootstrap. Construct the source string
  // when it is observed, without reading any public property of the function.
  Function.prototype.toString=new Proxy(engineFunctionToString,{apply(target,thisArg,args){const native=nativeFunctionNameGet(thisArg);return native===undefined?functionSourceApply(target,thisArg,args):'function '+native+'() { [native code] }'}});
  const markNative=(fn,name,prefix='')=>{if(typeof fn==='function')nativeFunctionNameSet(fn,prefix?prefix+String(name):String(name))};
  markNative(Function.prototype.toString,'toString');
