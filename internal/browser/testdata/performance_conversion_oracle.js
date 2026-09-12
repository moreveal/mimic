(()=>{
  const out={},log=[],record=(key,value)=>({get(){log.push(key);return value},configurable:true});
  const value=label=>({valueOf(){log.push(label);return 24}});
  const options={};for(const [key,v]of [['buffered',false],['durationThreshold',value('number')],['entryTypes',undefined],['type',{toString(){log.push('string');return 'event'}}]])Object.defineProperty(options,key,record(key,v));
  const observer=new PerformanceObserver(()=>{});observer.observe(options);out.observe=log.splice(0);observer.disconnect();
  const markOptions={};for(const [key,v]of [['detail',{get data(){log.push('clone');return 1}}],['startTime',value('start-number')]])Object.defineProperty(markOptions,key,record(key,v));
  performance.mark({toString(){log.push('name');return 'conversion'}},markOptions);out.mark=log.splice(0);
  const measureOptions={};for(const [key,v]of [['detail',{get data(){log.push('clone');return 1}}],['duration',undefined],['end',value('end-number')],['start',0]])Object.defineProperty(measureOptions,key,record(key,v));
  try{performance.measure('conversion',measureOptions)}catch(e){log.push(e.name)}out.measure=log.splice(0);
  if(typeof document!=='undefined'){const timing=performance.timing,n=performance.getEntriesByType('navigation')[0];out.legacy={fetch:timing.fetchStart>=timing.navigationStart,dns:timing.domainLookupStart>=timing.fetchStart,request:timing.requestStart>=timing.connectEnd,zero:timing.redirectStart===0&&timing.unloadEventStart===0,relative:Math.abs(timing.fetchStart-performance.timeOrigin-n.fetchStart)<1.1}}
  return out;
})()
