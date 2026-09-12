(async()=>{
  const out={}, wait=()=>new Promise(r=>setTimeout(r,30));performance.clearResourceTimings();
  const origin=location.origin,cross=origin.replace('127.0.0.1','localhost');
  const project=e=>{const j=e.toJSON();return {keys:Object.keys(j),name:e.name.startsWith(origin)?'same':'cross',type:e.entryType,initiator:e.initiatorType,brand:e instanceof PerformanceResourceTiming,identity:e===performance.getEntriesByName(e.name)[0],own:Object.keys(e),duration:e.duration===e.responseEnd-e.startTime,order:e.startTime<=e.fetchStart&&e.fetchStart<=e.responseEnd,phases:e.domainLookupStart<=e.domainLookupEnd&&e.domainLookupEnd<=e.connectStart&&e.connectStart<=e.connectEnd&&e.connectEnd<=e.requestStart&&e.requestStart<=e.responseStart&&e.responseStart<=e.responseEnd,hidden:e.requestStart===0,body:e.decodedBodySize,transferRelation:e.transferSize===e.encodedBodySize+300,status:e.responseStatus,protocol:e.nextHopProtocol,contentType:e.contentType,contentEncoding:e.contentEncoding,renderBlocking:e.renderBlockingStatus,finalHeaders:e.finalResponseHeadersStart===e.responseStart,serverFrozen:Object.isFrozen(e.serverTiming),serverSame:e.serverTiming===e.serverTiming,serverEntrySame:e.serverTiming[0]===e.serverTiming[0],server:e.serverTiming.map(s=>s.toJSON()),serverBrand:e.serverTiming.every(s=>s instanceof PerformanceServerTiming),jsonServerSame:j.serverTiming===e.serverTiming};};
  for(const [label,url,mode] of [['same',origin+'/resource','cors'],['tao',cross+'/resource?tao=*','cors'],['hidden',cross+'/resource','cors'],['opaque',cross+'/resource?tao=*&opaque=1','no-cors']]){
    await (await fetch(url,{mode})).text();await wait();const entry=performance.getEntriesByName(url)[0];out[label]=entry?project(entry):{present:false};
  }
  const before=performance.getEntriesByType('resource')[0];performance.clearResourceTimings();out.clear={empty:performance.getEntriesByType('resource').length===0,retained:before.entryType==='resource'};
  performance.setResourceTimingBufferSize(1);let events=[];performance.onresourcetimingbufferfull=function(e){events.push({trusted:e.isTrusted,target:e.target===performance,thisValue:this===performance,type:e.type,bubbles:e.bubbles,cancelable:e.cancelable})};
  const observed=[];const ob=new PerformanceObserver(list=>observed.push(...list.getEntries().map(e=>e.name)));ob.observe({type:'resource'});
  for(let i=0;i<3;i++){await(await fetch('/resource?buffer='+i)).text();await wait()}
  out.buffer={count:performance.getEntriesByType('resource').length,events,observed:observed.length};ob.disconnect();performance.onresourcetimingbufferfull=null;performance.clearResourceTimings();performance.setResourceTimingBufferSize(250);
  return out;
})()
