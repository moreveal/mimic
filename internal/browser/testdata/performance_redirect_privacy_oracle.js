(async()=>{
  performance.clearResourceTimings();const same=location.origin,cross=same.replace('127.0.0.1','localhost'),out={};
  const hop=(origin,next,tao)=>origin+'/redirect?next='+encodeURIComponent(next)+(tao?'&tao=*':'');
  const cases={same:hop(same,same+'/resource',false),allowed:hop(same,hop(cross,same+'/resource',true),false),allAllowed:hop(same,hop(cross,same+"/resource?tao=*",true),false),hiddenHop:hop(same,hop(cross,same+'/resource',false),false),hiddenFinal:hop(same,cross+'/resource',false)};
  for(const [label,url]of Object.entries(cases)){
    await(await fetch(url)).text();await new Promise(r=>setTimeout(r,20));const e=performance.getEntriesByName(url)[0];
    out[label]={one:performance.getEntriesByName(url).length===1,redirect:e.redirectEnd>0,details:e.requestStart>0,fetchOrder:e.fetchStart>=e.startTime,duration:e.duration===e.responseEnd-e.startTime,hiddenStart:e.startTime===e.fetchStart,status:e.responseStatus,body:e.decodedBodySize>0};
  }
  const f=document.createElement('iframe');f.src=hop(same,same+'/frame',false);await new Promise(r=>{f.onload=r;document.body.append(f)});const n=f.contentWindow.performance.getEntriesByType('navigation')[0];
  out.navigation={name:n.name===same+'/frame',redirectCount:n.redirectCount,redirectEnd:n.redirectEnd>0,redirectStart:n.redirectStart>=0&&n.redirectStart<=n.redirectEnd,fetchStart:n.fetchStart>=n.redirectEnd,order:n.responseEnd<=n.domInteractive,resourceOwner:performance.getEntriesByName(f.src).length===1};f.remove();return out;
})()
