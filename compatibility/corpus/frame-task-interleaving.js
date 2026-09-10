(async () => {
  const frame = document.createElement('iframe');
  const loaded = new Promise(resolve => { frame.onload = resolve; });
  document.body.appendChild(frame);
  await loaded;
  return await new Promise(resolve => {
    const rows = [];
    const handler = event => {
      if (event.source !== frame.contentWindow) return;
      rows.push({sent: event.data, observed: frame.contentWindow.step});
      if (rows.length === 5) {
        removeEventListener('message', handler);
        frame.remove();
        resolve(rows);
      }
    };
    addEventListener('message', handler);
    frame.contentWindow.eval(`
      window.step = 0;
      function tick() {
        step++;
        parent.postMessage(step, '*');
        if (step < 5) setTimeout(tick, 0);
      }
      setTimeout(tick, 0);
    `);
  });
})()
