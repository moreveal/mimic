(async () => {
  const out={}, frame=document.createElement('iframe');
  frame.src='/ignored-by-srcdoc';
  frame.srcdoc='<!doctype html><body><p id="first">first</p><script>window.marker=1;window.saved=()=>document.title;document.title="first";</script>';
  const load=action=>new Promise(resolve=>{frame.onload=resolve;action()});
  await load(()=>document.body.appendChild(frame));
  const win=frame.contentWindow,doc=frame.contentDocument;
  out.initial={url:doc.URL,marker:win.marker||null,title:doc.title,origin:win.origin===origin,base:doc.baseURI===document.baseURI};
  if(win.marker!==1){frame.remove();return out}
  const saved=win.saved, first=doc.getElementById('first');
  await load(()=>{frame.srcdoc='<!doctype html><title>second</title><body><script>window.marker=2;</script>'});
  out.replacement={sameWindow:win===frame.contentWindow,newDocument:doc!==frame.contentDocument,oldTitle:doc.title,oldNode:first.textContent,oldFunctionDocument:saved(),marker:win.marker,oldDefaultView:doc.defaultView===null};
  // Merely changing src while srcdoc is present must not navigate its document.
  const secondDoc=frame.contentDocument;
  frame.src='/pending-src';
  await new Promise(resolve=>setTimeout(resolve,40));
  out.srcShadowed={sameDocument:frame.contentDocument===secondDoc,marker:win.marker};
  await load(()=>{frame.srcdoc=''});
  out.empty={url:frame.contentDocument.URL,body:frame.contentDocument.body.innerHTML,marker:typeof win.marker};
  await load(()=>{frame.srcdoc='<title>cancelled</title>';frame.srcdoc='<title>latest</title>'});
  out.superseded={title:frame.contentDocument.title,url:frame.contentDocument.URL};
  await load(()=>{frame.src='/fallback-src';frame.removeAttribute('srcdoc')});
  out.removed={path:win.location.pathname,networkDocument:!!frame.contentDocument.getElementById('newDocument'),sameWindow:win===frame.contentWindow};
  frame.remove();
  return out;
})()
