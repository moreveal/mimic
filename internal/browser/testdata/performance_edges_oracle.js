(() => {
  const out={},attempt=fn=>{try{const v=fn();return v===undefined?'undefined':v}catch(e){return e.name}};
  performance.clearMarks();performance.clearMeasures();
  performance.mark('ordered',{startTime:12});performance.mark('ordered',{startTime:2});out.latest=performance.measure('latest','ordered',{toString(){return 'ordered'}}).startTime;
  out.emptyType=performance.getEntriesByName('ordered','').length;
  const m=performance.mark('data',{startTime:0,detail:undefined});out.undefinedDetail=m.detail;
  out.constructPrototype=attempt(()=>{class Derived extends PerformanceMark{}const m=new Derived('sub',{startTime:1});return [m instanceof Derived,m instanceof PerformanceMark,m.name,performance.getEntriesByName('sub').length]});
  out.entrySerializer=attempt(()=>{const m=performance.getEntriesByName('data')[0];Object.defineProperty(m,'name',{get(){throw Error('override')}});return PerformanceEntry.prototype.toJSON.call(m).name});
  const invalids=[undefined,null,3,'data',true,1n,NaN,{},[],{detail:7},{start:null,end:2},{start:undefined,end:2},{start:Infinity},{end:'missing'},{end:'navigationStart'},{start:'loadEventEnd'},{start:'unloadEventStart'}];
  out.measureCases=invalids.map(o=>attempt(()=>{const m=performance.measure('edge',o);return {startZero:m.startTime===0,negative:m.duration<0,detail:m.detail}}));
  const o=new PerformanceObserver(()=>{});o.observe({type:'mark'});const a=performance.mark('z',{startTime:9}),b=performance.mark('y',{startTime:3});out.takeOrder=o.takeRecords().map(e=>e.name);o.disconnect();
  out.legacySerializer=attempt(()=>{Object.defineProperty(performance.timing,'navigationStart',{get(){throw Error('override')}});return performance.timing.toJSON().navigationStart>0});
  if(typeof document!=='undefined'){
    const confidence=performance.getEntriesByType('navigation')[0].confidence;
    out.confidence={tag:Object.prototype.toString.call(confidence),same:confidence===performance.getEntriesByType('navigation')[0].confidence,keys:Object.keys(confidence),prototype:Object.getOwnPropertyNames(Object.getPrototypeOf(confidence)),rateRange:confidence.randomizedTriggerRate>=0&&confidence.randomizedTriggerRate<=1,value:['high','low'].includes(confidence.value),jsonKeys:Object.keys(confidence.toJSON()),rateStable:confidence.toJSON().randomizedTriggerRate===confidence.randomizedTriggerRate};
  }
  performance.clearMarks();performance.clearMeasures();return out;
})()
