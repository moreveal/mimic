(async()=>{
  performance.clearResourceTimings();const url=location.origin+'/redirect?timing=1';await(await fetch(url)).text();await new Promise(r=>setTimeout(r,30));
  const entries=performance.getEntriesByType('resource'),entry=entries.find(e=>e.name===url);
  return {count:entries.length,originalName:!!entry,redirect:entry?{visible:entry.redirectStart>0,order:entry.startTime<=entry.redirectStart&&entry.redirectStart<=entry.redirectEnd&&entry.redirectEnd<=entry.fetchStart&&entry.fetchStart<=entry.requestStart&&entry.requestStart<=entry.responseStart&&entry.responseStart<=entry.responseEnd,duration:entry.duration===entry.responseEnd-entry.startTime,status:entry.responseStatus}:null};
})()
