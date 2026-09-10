(async () => {
  const frame = document.createElement('iframe');
  const loaded = new Promise(resolve => { frame.onload = resolve; });
  document.body.appendChild(frame);
  await loaded;
  // Finish setup/native polling before the operation under test.
  await new Promise(resolve => setTimeout(resolve, 30));
  frame.contentWindow.eval(`
    function enqueue() { Promise.resolve().then(()=>parent.frameJobOrder.push('child-job')); }
    globalThis.jobProbe = {
      get value() { enqueue(); return 7; },
      call() { enqueue(); },
      throwing() { enqueue(); throw new Error('probe'); }
    };
    globalThis.JobConstructor = function() { enqueue(); };
  `);
  const result = {};
  for (const mode of ['eval', 'getter', 'call', 'construct', 'throw']) {
    globalThis.frameJobOrder = [];
    if (mode === 'eval') frame.contentWindow.eval('enqueue()');
    if (mode === 'getter') void frame.contentWindow.jobProbe.value;
    if (mode === 'call') frame.contentWindow.jobProbe.call();
    if (mode === 'construct') new frame.contentWindow.JobConstructor();
    if (mode === 'throw') {
      let caught = false;
      try { frame.contentWindow.jobProbe.throwing(); } catch { caught = true; }
      if (!caught) throw new Error('missing cross-realm exception');
    }
    frameJobOrder.push('parent-sync');
    await new Promise(resolve => setTimeout(() => {
      frameJobOrder.push('parent-timer');
      resolve();
    }, 0));
    result[mode] = frameJobOrder.slice();
  }
  delete globalThis.frameJobOrder;
  frame.remove();
  return result;
})()
