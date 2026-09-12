(()=>{
  const result={},attempt=value=>{try{const m=performance.mark('clone-probe',{detail:value});return {success:true,keys:Object.keys(m.detail)}}catch(e){return {error:e.name}}};
  const mark=performance.mark('source',{detail:{value:1}}),observer=new PerformanceObserver(()=>{});
  for(const [name,value]of [['mark',mark],['nestedMark',{mark}],['performance',performance],['observer',observer],['timing',performance.timing],['memory',performance.memory],['eventCounts',performance.eventCounts]])result[name]=attempt(value);
  const frame=document.createElement('iframe');document.body.append(frame);result.foreignMark=attempt(frame.contentWindow.performance.mark('foreign'));frame.remove();
  performance.clearMarks();return result;
})()
