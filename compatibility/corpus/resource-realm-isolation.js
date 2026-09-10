(async () => {
  const pending = fetch('/echo?scope=parent&delay=0.7').then(r=>r.text());
  await new Promise(resolve=>setTimeout(resolve,50));
  const frame=document.createElement('iframe'); frame.src='/';
  await new Promise(resolve=>{frame.addEventListener('load',resolve,{once:true});document.body.append(frame)});
  await pending;
  await new Promise(resolve=>setTimeout(resolve,20));
  const scopes=win=>win.performance.getEntriesByType('resource').map(e=>new URL(e.name).searchParams.get('scope')).filter(Boolean);
  const value={parent:scopes(window),child:JSON.parse(frame.contentWindow.eval("JSON.stringify(performance.getEntriesByType('resource').map(e=>new URL(e.name).searchParams.get('scope')).filter(Boolean))"))};
  frame.remove();return value;
})()
