(async () => {
  const target = document.createElement('div');
  target.style.cssText =
    'box-sizing:content-box;width:40px;height:20px;padding:3px;border:2px solid';
  document.body.append(target);
  const samples = [];
  await new Promise((resolve) => {
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      samples.push([
        entry.contentRect.width,
        entry.contentRect.height,
        entry.contentBoxSize[0].inlineSize,
        entry.borderBoxSize[0].inlineSize,
      ]);
      if (samples.length === 1) target.style.width = '75px';
      else {
        observer.disconnect();
        resolve();
      }
    });
    observer.observe(target);
  });
  target.remove();
  return samples;
})()
