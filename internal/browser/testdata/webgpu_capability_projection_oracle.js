(async()=>{
 const gpu=navigator.gpu,adapter=await gpu.requestAdapter(),device=await adapter.requestDevice();
 const limits=o=>Object.fromEntries(Object.getOwnPropertyNames(Object.getPrototypeOf(o)).filter(k=>k!=='constructor').map(k=>[k,o[k]]));
 const out={wgsl:Array.from(gpu.wgslLanguageFeatures||[]),wgslTag:Object.prototype.toString.call(gpu.wgslLanguageFeatures),adapterLimits:limits(adapter.limits),deviceLimits:limits(device.limits),features:Array.from(adapter.features)};
 const f=gpu.wgslLanguageFeatures;if(f){let count=0,valid=true;f.forEach(function(v,k,set){count++;valid&&=v===k&&set===f&&this===out},out);out.languageSet={identity:gpu.wgslLanguageFeatures===f,size:f.size,count,valid,entries:Array.from(f.entries()).every(([a,b])=>a===b),has:Array.from(f).every(v=>f.has(v)),missing:f.has('not_a_language_feature')}}
 device.destroy();return out;
})()
