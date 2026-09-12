(()=>{
 const x=new OffscreenCanvas(200,100).getContext('2d'),out={};
 for(const font of ['10px Arial','16px Arial','italic 16px Arial','14px monospace'])for(const text of ['','A','gj','Hello world',' AV ','ffi']){
  x.font=font;x.textBaseline='alphabetic';x.textAlign='left';const m=x.measureText(text);out[font+'|'+text]=Object.fromEntries(Object.getOwnPropertyNames(Object.getPrototypeOf(m)).filter(k=>k!=='constructor').map(k=>[k,m[k]]));
 }
 x.font='16px Arial';for(const baseline of ['top','hanging','middle','alphabetic','ideographic','bottom']){x.textBaseline=baseline;const m=x.measureText('Ag');out['baseline|'+baseline]=Object.fromEntries(['width','fontBoundingBoxAscent','fontBoundingBoxDescent','actualBoundingBoxAscent','actualBoundingBoxDescent','emHeightAscent','emHeightDescent','hangingBaseline','alphabeticBaseline','ideographicBaseline'].map(k=>[k,m[k]]))}
 x.textBaseline='alphabetic';for(const align of ['start','end','left','right','center']){x.textAlign=align;const m=x.measureText('AV');out['align|'+align]={width:m.width,left:m.actualBoundingBoxLeft,right:m.actualBoundingBoxRight}}
 x.textAlign='left';for(const spacing of ['0px','1px','-1px']){x.letterSpacing=spacing;x.wordSpacing='3px';const m=x.measureText(' AV\tffi\n');out['spacing|'+spacing]={width:m.width}}
 out.spacingState={letter:x.letterSpacing,word:x.wordSpacing};for(const kind of ['offscreen','html']){const c=kind==='html'?document.createElement('canvas'):new OffscreenCanvas(10,10),cx=c.getContext('2d');cx.font='16px Arial';for(const word of ['0px','3px']){cx.wordSpacing=word;out[kind+'|word|'+word]={space:cx.measureText(' ').width,text:cx.measureText('A A').width}}}
 const pixelContext=new OffscreenCanvas(80,24).getContext('2d');pixelContext.font='16px Arial';const pixels=word=>{pixelContext.clearRect(0,0,80,24);pixelContext.wordSpacing=word;pixelContext.fillText('A A',2,16);return Array.from(pixelContext.getImageData(0,0,80,24).data).join(',')};out.wordPixelsEqual=pixels('0px')===pixels('3px');
 return out;
})()
