(async()=>{
  const result={},wait=()=>new Promise(r=>setTimeout(r,30));
  const fetchOne=async label=>{await(await fetch('/resource?buffer='+label)).text();await wait()};
  performance.clearResourceTimings();performance.setResourceTimingBufferSize(0);
  const events=[],batches=[];const observer=new PerformanceObserver((list,o,options)=>batches.push({count:list.getEntries().length,dropped:options.droppedEntriesCount}));observer.observe({type:'resource',buffered:true});
  performance.addEventListener('resourcetimingbufferfull',()=>events.push('first'));
  performance.onresourcetimingbufferfull=()=>events.push('old');
  performance.addEventListener('resourcetimingbufferfull',()=>events.push('last'));
  performance.onresourcetimingbufferfull=()=>{events.push('replacement');performance.setResourceTimingBufferSize(1)};
  await fetchOne('grow');result.grow={events:events.slice(),buffer:performance.getEntriesByType('resource').length,batches:batches.slice()};
  events.length=0;batches.length=0;performance.onresourcetimingbufferfull=()=>{events.push('clear');performance.clearResourceTimings()};
  await fetchOne('clear');result.clear={events:events.slice(),buffer:performance.getEntriesByType('resource').length,batches:batches.slice()};
  events.length=0;batches.length=0;performance.onresourcetimingbufferfull=null;
  await fetchOne('drop');result.drop={events:events.slice(),buffer:performance.getEntriesByType('resource').length,batches:batches.slice()};
  observer.disconnect();return result;
})()
