(async () => {
  const order = [], raw = [], start = performance.now();
  const mark = name => {order.push(name); raw.push(performance.now());};
  mark('sync');
  queueMicrotask(() => {mark('microtask'); Promise.resolve().then(() => mark('nested-microtask'));});
  Promise.resolve().then(() => mark('promise'));
  await new Promise(resolve => setTimeout(() => {mark('timer'); queueMicrotask(() => mark('timer-microtask')); resolve();}, 10));
  mark('continuation');
  const frame = document.createElement('iframe');
  const frames = [];
  frame.onload = () => frames.push('load');
  frame.src = '/';
  await new Promise(resolve => {frame.addEventListener('load', resolve, {once: true}); document.body.append(frame);});
  frames.push(frame.contentDocument.readyState);
  frame.remove();
  const workerSource = `onmessage=()=>{const a=['message'];queueMicrotask(()=>a.push('microtask'));setTimeout(()=>postMessage(a.concat('timer')),0)}`;
  const blob = URL.createObjectURL(new Blob([workerSource], {type:'text/javascript'}));
  const worker = new Worker(blob);
  const workerOrder = await new Promise(resolve=>{worker.onmessage=e=>resolve({value:e.data});worker.onerror=e=>resolve({exception:{message:e.message}});worker.postMessage(null);});
  worker.terminate(); URL.revokeObjectURL(blob);
  const before = performance.now();
  await (await fetch('/echo?timing=1')).text();
  const entries = performance.getEntriesByType('resource').filter(e=>e.name.includes('timing=1'));
  return {order, monotonic: raw.every((v,i)=>i===0||v>=raw[i-1]), positiveElapsed: performance.now()>start,
    frames, workerOrder, resource: entries.map(e=>({type:e.initiatorType, nonnegative:e.duration>=0,
      ordered:e.startTime<=e.responseEnd, startsAfterProbe:e.startTime>=before,
      status:e.responseStatus, protocol:e.nextHopProtocol}))};
})()
