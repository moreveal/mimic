(async () => {
  const adapter = await navigator.gpu.requestAdapter();
  const frame = document.createElement('iframe');
  await new Promise((resolve) => {
    frame.onload = resolve;
    document.body.append(frame);
  });
  const child = await frame.contentWindow.eval(
    `(async()=>{const a=await navigator.gpu.requestAdapter();return {gpu:navigator.gpu===navigator.gpu,adapter:a===await navigator.gpu.requestAdapter(),device:a.info.device,features:[...a.features].sort()}})()`,
  );
  const source = `(async()=>{const a=await navigator.gpu.requestAdapter();postMessage({gpu:navigator.gpu===navigator.gpu,adapter:a===await navigator.gpu.requestAdapter(),device:a.info.device,features:[...a.features].sort()})})()`;
  const worker = await new Promise((resolve, reject) => {
    const url = URL.createObjectURL(new Blob([source], { type: 'text/javascript' })),
      instance = new Worker(url);
    instance.onmessage = (event) => {
      instance.terminate();
      URL.revokeObjectURL(url);
      resolve(event.data);
    };
    instance.onerror = (event) => reject(event.message);
  });
  frame.remove();
  return {
    parent: {
      gpu: navigator.gpu === navigator.gpu,
      device: adapter.info.device,
      features: [...adapter.features].sort(),
    },
    child,
    worker,
  };
})();
