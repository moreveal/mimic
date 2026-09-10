(function normalize(value, seen = new WeakSet()) {
  // Reserved marker objects are not valid corpus output values. Tag JS values
  // which CDP's JSON by-value projection otherwise drops or turns into null.
  if (value === undefined) return {$jsValue:'undefined'};
  if (typeof value === 'number') {
    if (Number.isNaN(value)) return {$jsValue:'NaN'};
    if (!Number.isFinite(value)) return {$jsValue:String(value)};
    if (Object.is(value,-0)) return {$jsValue:'-0'};
  }
  if (typeof value === 'bigint') return {$jsValue:'bigint',value:String(value)};
  if (typeof value === 'function' || typeof value === 'symbol') throw new TypeError('Corpus must describe functions and symbols explicitly');
  if (value === null || typeof value !== 'object') return value;
  if (seen.has(value)) throw new TypeError('Corpus must describe cyclic identity explicitly');
  if (Object.prototype.hasOwnProperty.call(value,'$jsValue')) throw new TypeError('Reserved corpus marker');
  seen.add(value);
  try {
    if (Array.isArray(value)) return Array.from({length:value.length},(_,i)=>Object.prototype.hasOwnProperty.call(value,i)?normalize(value[i],seen):{$jsValue:'hole'});
    const result={};
    for (const key of Object.keys(value)) Object.defineProperty(result,key,{value:normalize(value[key],seen),enumerable:true,writable:true,configurable:true});
    return result;
  } finally {seen.delete(value)}
})
