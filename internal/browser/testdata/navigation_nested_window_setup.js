(async () => {
  const frame=document.createElement('iframe');
  frame.srcdoc=`<!doctype html><body><script>
    const inner=document.createElement('iframe');document.body.appendChild(inner);
    inner.contentDocument.title='retained nested';
    window.exposedNested=inner.contentWindow;
  </script>`;
  await new Promise(resolve=>{frame.onload=resolve;document.body.appendChild(frame);});
  // Export only the grandchild WindowProxy, no ancestor eval or nested object.
  globalThis.__retainedNestedWindow=frame.contentWindow.exposedNested;
  await new Promise(resolve=>{frame.onload=resolve;frame.src='/navigation-nested-window';frame.removeAttribute('srcdoc');});
  frame.remove();
  return true;
})()
