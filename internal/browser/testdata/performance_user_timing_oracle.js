(() => {
  performance.clearMarks();performance.clearMeasures();
  const out={}, attempt=fn=>{try{const v=fn();return v&&v.entryType?{name:v.name,type:v.entryType,start:v.startTime,duration:v.duration,detail:v.detail}:v===undefined?'undefined':v}catch(e){return {error:e.name}}};
  const detail={a:[1],map:new Map([['a',2]])},m=performance.mark('late',{startTime:8,detail});detail.a[0]=9;
  const early=performance.mark('early',{startTime:2}), duplicate=performance.mark('late',{startTime:8});
  const json=e=>{const j=e.toJSON();delete j.navigationId;return j};
  out.mark={json:json(m),clone:m.detail!==detail&&m.detail.a[0]===1,map:m.detail.map instanceof Map,detailSame:m.detail===m.detail,identity:performance.getEntriesByName('late')[0]===m,duplicateIdentity:duplicate!==m,brand:m instanceof PerformanceMark&&m instanceof PerformanceEntry,own:Object.keys(m)};
  m.detail.a[0]=7;out.mark.mutableDetail=m.detail.a[0]===7;
  out.order=performance.getEntriesByType('mark').map(e=>e.name);
  out.construct=attempt(()=>new PerformanceMark('constructed',{startTime:4,detail:{x:1}}));out.constructNotBuffered=performance.getEntriesByName('constructed').length===0;
  out.measureNamed=attempt(()=>performance.measure('named','early','late'));
  out.measureOptions=attempt(()=>performance.measure('options',{start:1,duration:3,detail:{x:2}}));
  out.measureEndDuration=attempt(()=>performance.measure('end-duration',{end:2,duration:5}));
  out.measureNegative=attempt(()=>performance.measure('negative',{start:8,end:2}));
  const measured=performance.getEntriesByName('named')[0];out.measure={brand:measured instanceof PerformanceMeasure,identity:measured===performance.getEntriesByName('named')[0],json:json(measured),own:Object.keys(measured)};
  const errors={markMissing:()=>performance.mark(),markNegative:()=>performance.mark('x',{startTime:-1}),markNaN:()=>performance.mark('x',{startTime:NaN}),markInfinity:()=>performance.mark('x',{startTime:Infinity}),markSymbol:()=>performance.mark(Symbol()),markDetailFunction:()=>performance.mark('x',{detail:()=>{}}),markReserved:()=>performance.mark('navigationStart'),markOptionsNumber:()=>performance.mark('x',2),markNull:()=>performance.mark('null',{startTime:null}),markBigint:()=>performance.mark('x',{startTime:1n}),measureMissing:()=>performance.measure(),measureUnknown:()=>performance.measure('x','unknown'),measureOptionsEmpty:()=>performance.measure('x',{}),measureAllThree:()=>performance.measure('x',{start:1,end:2,duration:1}),measureDurationOnly:()=>performance.measure('x',{duration:1}),measureOptionsThird:()=>performance.measure('x',{start:1},'early'),measureNegativeStart:()=>performance.measure('x',{start:-1,end:2}),measureNegativeDuration:()=>performance.measure('x',{start:1,duration:-2}),measureNaN:()=>performance.measure('x',{start:NaN}),measureDetailFunction:()=>performance.measure('x',{start:0,detail:()=>{}})};
  for(const [name,fn] of Object.entries(errors)) {const result=attempt(fn);out[name]=result&&result.type?{type:result.type,negativeStart:result.start<0,negativeDuration:result.duration<0}:result;}
  let order=[];const options={get detail(){order.push('detail');return null},get startTime(){order.push('startTime');return 1}};performance.mark({toString(){order.push('name');return 'conversion'}},options);out.markConversion=order;
  order=[];performance.measure({toString(){order.push('name');return 'conversion'}},{get detail(){order.push('detail')},get duration(){order.push('duration')},get end(){order.push('end');return 2},get start(){order.push('start');return 1}});out.measureConversion=order;
  const circular={};circular.self=circular;const cm=performance.mark('cycle',{detail:circular});out.cycle=cm.detail!==circular&&cm.detail.self===cm.detail;
  performance.clearMarks('late');out.clearName=performance.getEntriesByName('late').length;out.retained=m.name==='late'&&m.detail.a[0]===7;
  performance.clearMarks();performance.clearMeasures();out.cleared=performance.getEntriesByType('mark').length+performance.getEntriesByType('measure').length;return out;
})()
