(async()=>{
  const out={};performance.clearMarks();const entries=[];const o=new PerformanceObserver(list=>entries.push(...list.getEntries()));o.observe({type:'longtask'});
  await new Promise(resolve=>setTimeout(()=>{const start=performance.now();while(performance.now()-start<65){};queueMicrotask(()=>{const start=performance.now();while(performance.now()-start<10){}});resolve()},0));
  await new Promise(resolve=>setTimeout(resolve,100));o.disconnect();const e=entries.find(e=>e.duration>=60);
  out.task=e?{type:e.entryType,name:e.name,brand:e instanceof PerformanceLongTaskTiming,duration:e.duration>=65,start:e.startTime>=0,own:Object.keys(e),jsonKeys:Object.keys(e.toJSON()),attribution:e.attribution.map(a=>({type:a.entryType,name:a.name,start:a.startTime,duration:a.duration,containerType:a.containerType,containerSrc:a.containerSrc,containerId:a.containerId,containerName:a.containerName,brand:a instanceof TaskAttributionTiming})),frozen:Object.isFrozen(e.attribution),same:e.attribution===e.attribution,entrySame:e.attribution[0]===e.attribution[0]}:null;
  out.timeline=performance.getEntriesByType('longtask').length;
  const buffered=new PerformanceObserver(()=>{});buffered.observe({type:'longtask',buffered:true});out.buffered=buffered.takeRecords().some(x=>x===e);buffered.disconnect();return out;
})()
