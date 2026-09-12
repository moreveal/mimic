(()=>{const out={};for(const colorSpace of ['srgb','display-p3'])for(const colorType of ['unorm8','float16']){
 const c=new OffscreenCanvas(3,2),x=c.getContext('2d',{colorSpace,colorType,willReadFrequently:true}),key=colorSpace+'|'+colorType;
 const read=settings=>{const d=x.getImageData(0,0,1,1,settings);return {colorSpace:d.colorSpace,pixelFormat:d.pixelFormat,type:d.data.constructor.name,data:Array.from(d.data)}};
 const row={attributes:x.getContextAttributes(),colors:{}};for(const color of ['color(display-p3 1 .25 .5)','color(srgb .2 .4 .6 / .5)','#804020','color(display-p3 .1 .5 .8)','color(display-p3 .5 .25 .5 / .5)','color(display-p3 .5 .5 .5)']){x.clearRect(0,0,3,2);x.fillStyle=color;x.fillRect(0,0,1,1);row.colors[color]={style:x.fillStyle,default:read()};for(const space of ['srgb','display-p3'])for(const pixelFormat of ['rgba-unorm8','rgba-float16']){try{row.colors[color][space+'|'+pixelFormat]=read({colorSpace:space,pixelFormat})}catch(e){row.colors[color][space+'|'+pixelFormat]={error:e.name}}}}
 out[key]=row;
}return out})()
