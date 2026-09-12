(async()=>{
  const out={isolated:crossOriginIsolated,exposed:typeof performance.measureUserAgentSpecificMemory};
  if(out.exposed!=='function')return out;
  const method=Performance.prototype.measureUserAgentSpecificMemory;
  out.receiver=await(async()=>{try{const p=method.call({});try{await p;return 'resolved'}catch(e){return 'async:'+e.name}}catch(e){return 'sync:'+e.name}})();
  const probe=performance.measureUserAgentSpecificMemory();out.promise=probe instanceof Promise;await Promise.resolve(probe).catch(()=>{});
  try {
  const result=await performance.measureUserAgentSpecificMemory();
  out.result={keys:Object.keys(result),positive:Number.isInteger(result.bytes)&&result.bytes>0,breakdown:Array.isArray(result.breakdown),sum:result.bytes===result.breakdown.reduce((n,b)=>n+b.bytes,0),rows:result.breakdown.map(b=>({keys:Object.keys(b),bytesNonnegative:Number.isInteger(b.bytes)&&b.bytes>=0,types:b.types,attribution:b.attribution.map(a=>({keys:Object.keys(a),scope:a.scope,url:a.url===location.href?'self':a.url,container:a.container}))})).sort((a,b)=>a.types.join().localeCompare(b.types.join()))};
  } catch(error) {out.result={error:error.name}}
  return out;
})()
