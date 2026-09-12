(async()=>{
  const f=document.createElement('iframe');f.src='/lifecycle';await new Promise(resolve=>{f.onload=resolve;document.body.append(f)});await new Promise(resolve=>setTimeout(resolve,40));
  const w=f.contentWindow;w.samplePerformanceLifecycle('after');
  const out={rows:w.performanceLifecycleRows,normal:w.performanceLifecycleNormal.length,buffered:w.performanceLifecycleBuffered.length,normalSame:w.performanceLifecycleNormal[0]===w.performanceLifecycleEntry,bufferedSame:w.performanceLifecycleBuffered[0]===w.performanceLifecycleEntry};f.remove();return out;
})()
