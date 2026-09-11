(() => {
  const out={};
  for(const name of ['Navigator','Screen','History','Performance'])out[name]=Object.getOwnPropertyNames(globalThis[name].prototype);
  out.AbortSignal=Object.getOwnPropertyNames(AbortSignal);
  // Publication happens once: subsequent user delete/redefine changes order.
  const descriptor=Object.getOwnPropertyDescriptor(Navigator.prototype,'language');
  delete Navigator.prototype.language;Object.defineProperty(Navigator.prototype,'language',descriptor);
  out.reinserted=Object.getOwnPropertyNames(Navigator.prototype).slice(-1);
  return out;
})()
