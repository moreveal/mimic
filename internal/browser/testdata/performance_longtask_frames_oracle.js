(async()=>{
  const f=document.createElement('iframe');f.id='perf-frame';f.name='named-frame';f.src='/frame';await new Promise(r=>{f.onload=r;document.body.append(f)});
  const child=f.contentWindow,topRows=[],childRows=[],o=new PerformanceObserver(list=>topRows.push(...list.getEntries())),c=new child.PerformanceObserver(list=>childRows.push(...list.getEntries()));
  o.observe({type:'longtask'});c.observe({type:'longtask'});
  child.setTimeout(()=>{const start=child.performance.now();while(child.performance.now()-start<65){}},0);
  await new Promise(r=>setTimeout(r,120));o.disconnect();c.disconnect();
  const project=(entries,w)=>entries.map(e=>({name:e.name,brand:e instanceof w.PerformanceLongTaskTiming,navigation:e.navigationId===w.performance.getEntriesByType('navigation')[0].navigationId,attribution:e.attribution.map(a=>({name:a.name,type:a.containerType,id:a.containerId,frameName:a.containerName,src:a.containerSrc,brand:a instanceof w.TaskAttributionTiming}))}));
  const out={top:project(topRows,window),child:project(childRows,child)};f.remove();return out;
})()
