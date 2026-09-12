(() => {
  const input=document.createElement('input');input.id='performance-input';document.body.append(input);input.focus();
  const counts=performance.eventCounts,events=[],first=[];
  new PerformanceObserver(list=>events.push(...list.getEntries())).observe({type:'event',durationThreshold:16});
  new PerformanceObserver(list=>first.push(...list.getEntries())).observe({type:'first-input'});
  input.addEventListener('keydown',e=>{if(e.isTrusted){const start=performance.now();while(performance.now()-start<110){}}});
  input.addEventListener('keyup',e=>{if(e.isTrusted){const start=performance.now();while(performance.now()-start<80){}}});
  input.dispatchEvent(new KeyboardEvent('keydown',{bubbles:true}));input.click();
  const synthetic={keydown:counts.get('keydown'),click:counts.get('click'),interaction:performance.interactionCount};
  globalThis.performanceInputResult=()=>({synthetic,counts:{same:counts===performance.eventCounts,brand:counts instanceof EventCounts,keydown:counts.get('keydown'),keyup:counts.get('keyup'),interaction:performance.interactionCount},events:events.map(e=>({type:e.entryType,name:e.name,brand:e instanceof PerformanceEventTiming,target:e.target===input,cancelable:e.cancelable,order:e.startTime<=e.processingStart&&e.processingStart<=e.processingEnd,durationGrid:e.duration%8===0,durationEnough:e.duration>=64,interaction:e.interactionId>0,keys:Object.keys(e.toJSON())})),first:first.map(e=>({type:e.entryType,name:e.name,brand:e instanceof PerformanceEventTiming,distinct:!events.includes(e),target:e.target===input,interaction:e.interactionId===events.find(x=>x.name===e.name)?.interactionId})),sharedInteraction:events.length>=2&&events[0].interactionId===events[1].interactionId,hidden:performance.getEntriesByType('event').length===0&&performance.getEntriesByType('first-input').length===0});
  const original=performanceInputResult;globalThis.performanceInputResult=()=>{const result=original();result.hidden={event:performance.getEntriesByType('event').length,first:performance.getEntriesByType('first-input').length};return result};
  return true;
})()
