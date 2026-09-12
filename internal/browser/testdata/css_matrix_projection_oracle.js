(()=>{
 const out={},transforms=['none','skewX(20deg)','skewY(-15deg)','skew(10deg,20deg)','scale3d(2,3,4)','translate3d(10px,20px,30px)','rotateX(30deg)','rotateY(35deg)','rotate3d(1,2,3,25deg)','perspective(500px) translateZ(50px)','matrix3d(1,0,0,0,0,1,0,0,0,0,1,0,12,13,14,1)','translate(20%,50%) rotate(10deg)'];
 const e=document.createElement('div');e.style.cssText='position:fixed;left:40px;top:30px;width:100px;height:50px;transform-origin:20px 10px 5px';document.body.append(e);
 out.rects=transforms.map(transform=>{e.style.transform=transform;const r=e.getBoundingClientRect();return {transform,x:r.x,y:r.y,width:r.width,height:r.height}});
 const restore=[];let calls=[];for(const name of ['multiplySelf','translateSelf','scaleSelf','rotateSelf','rotateAxisAngleSelf','skewXSelf','skewYSelf']){const original=DOMMatrix.prototype[name];restore.push(()=>DOMMatrix.prototype[name]=original);DOMMatrix.prototype[name]=function(){calls.push(name);throw Error('author matrix method')}}
 try{out.parsed=new DOMMatrix('translate(2px,3px) rotate(20deg) skewY(5deg)').toFloat64Array().length;e.style.transform='perspective(500px) translateZ(50px)';out.privateProjection=e.getBoundingClientRect().width>100;out.calls=calls}catch(err){out.error=err.name}
 restore.forEach(f=>f());e.remove();return out;
})()
