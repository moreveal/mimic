const nativeFunctionNames = new WeakMap();
const nativeFunctionRecordGet = WeakMap.prototype.get.bind(nativeFunctionNames),
  nativeFunctionNameSet = WeakMap.prototype.set.bind(nativeFunctionNames),
  functionSourceApply = Reflect.apply;
const nativeFunctionNameGet = (fn) => nativeFunctionRecordGet(fn)?.name;
const nativeFunctionSourceGet = (fn) => {
  const record = nativeFunctionRecordGet(fn);
  return record === undefined
    ? undefined
    : record.source === undefined
      ? 'function ' + record.name + '() { [native code] }'
      : record.source;
};
const markForeignFunctionSource = (fn, source) =>
  nativeFunctionNameSet(fn, { __proto__: null, source });
const engineFunctionToString = Function.prototype.toString;
// Only explicitly marked platform functions override the engine's source.
// Reading callable properties here invokes user getters/proxy traps, and
// inspecting source text also misclassifies ordinary user function bodies.
// Keep binding names and imported immutable intrinsic sources in one registry.
// Source observation never reads public callable properties. The concise
// fallback has ordinary prototype-cycle behavior; V8 replaces it after restore
// with an engine-owned, nonconstructible callback.
Function.prototype.toString = {
  toString() {
    const source = nativeFunctionSourceGet(this);
    return source === undefined ? functionSourceApply(engineFunctionToString, this, []) : source;
  },
}.toString;
const nativeFunctionSourceState = [nativeFunctionSourceGet, engineFunctionToString];
const markNative = (fn, name, prefix = '') => {
  if (typeof fn === 'function')
    nativeFunctionNameSet(fn, {
      __proto__: null,
      name: prefix ? prefix + String(name) : String(name),
    });
};
markNative(Function.prototype.toString, 'toString');
