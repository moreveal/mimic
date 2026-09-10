(() => {
  const names = ['Navigator','Storage','Worker','HTMLIFrameElement','SVGElement','SVGGraphicsElement',
    'SVGGeometryElement','SVGSVGElement','SVGLength','SVGPoint','DOMMatrix','CanvasRenderingContext2D',
    'RTCPeerConnection','GPU','GPUAdapter','HTMLMediaElement','FontFace','FontFaceSet'];
  const out = {};
  for (const name of names) {
    const c = globalThis[name];
    if (typeof c !== 'function') {out[name]={type:typeof c};continue;}
    const p=c.prototype, props={};
    for (const k of Object.getOwnPropertyNames(p).sort()) {
      const d=Object.getOwnPropertyDescriptor(p,k);
      props[k]={enumerable:d.enumerable, configurable:d.configurable};
      if ('value' in d) Object.assign(props[k],{writable:d.writable,type:typeof d.value,...(typeof d.value==='function'?{name:d.value.name,length:d.value.length}:{})});
      else Object.assign(props[k],{get:typeof d.get,set:typeof d.set});
    }
    out[name]={type:typeof c,parent:Object.getPrototypeOf(p)?.constructor?.name,props};
  }
  return out;
})()
