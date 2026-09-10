(async()=>{
  await (await fetch('/echo?resource-json=1')).text();
  const e=performance.getEntriesByType('resource').find(e=>e.name.includes('resource-json=1'));
  const fields=['workerStart','redirectStart','redirectEnd','firstInterimResponseStart','deliveryType','navigationId'];
  const before=e.toJSON(),present=fields.map(k=>[k,Object.prototype.hasOwnProperty.call(before,k),before[k]===e[k]]);
  Object.defineProperty(e,'workerStart',{get(){throw new Error('public getter must not be called')}});
  Object.defineProperty(e,'serverTiming',{get(){throw new Error('public getter must not be called')}});
  let override;try{e.toJSON();override='ignored'}catch(err){override=err.name+': '+err.message}
  let brand;try{PerformanceResourceTiming.prototype.toJSON.call({});brand='accepted'}catch(err){brand=err.name}
  return {present,override,brand};
})()
