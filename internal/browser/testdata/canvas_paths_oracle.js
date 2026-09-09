(()=>{
 const c=new OffscreenCanvas(32,32),x=c.getContext('2d'),pixel=(a,b)=>Array.from(x.getImageData(a,b,1,1).data),out={};
 x.fillStyle='red';x.beginPath();x.rect(2,2,8,8);x.fill();out.rect=[pixel(4,4),pixel(1,1)];
 x.save();x.beginPath();x.rect(4,4,2,2);x.clip();x.fillStyle='blue';x.fillRect(0,0,32,32);x.restore();out.clip=[pixel(4,4),pixel(3,3)];
 x.clearRect(0,0,32,32);x.beginPath();x.arc(16,16,12,0,Math.PI*2);x.arc(16,16,6,0,Math.PI*2);x.fill('evenodd');out.evenodd=[pixel(16,16)[3],pixel(25,16)[3],pixel(31,31)[3]];
 x.beginPath();x.rect(2,2,8,8);out.point=[x.isPointInPath(4,4),x.isPointInPath(20,20)];x.save();x.beginPath();x.rect(15,15,4,4);x.restore();out.pathNotSaved=[x.isPointInPath(3,3),x.isPointInPath(16,16)];
 x.reset();const g=x.createLinearGradient(0,0,32,0);g.addColorStop(0,'black');g.addColorStop(1,'white');x.fillStyle=g;x.fillRect(0,0,32,32);out.gradient=pixel(2,2)[0]<pixel(20,2)[0];
 x.reset();x.fillStyle='red';x.fillRect(0,0,32,32);x.globalCompositeOperation='copy';x.fillStyle='blue';x.fillRect(4,4,4,4);out.copy=[pixel(0,0),pixel(5,5)];
 x.reset();x.fillStyle='red';x.beginPath();x.moveTo(2,2);x.lineTo(15,2);x.lineTo(2,15);x.closePath();x.fill();out.triangle=[pixel(4,4)[3],pixel(14,14)[3]];
 x.reset();x.translate(12,12);x.rotate(Math.PI/2);x.fillRect(0,0,4,8);out.transform=[pixel(6,14)[3],pixel(14,14)[3]];
 x.reset();x.beginPath();x.rect(1,1,4,4);c.width=32;out.resizePath=x.isPointInPath(2,2);
 return out;
})()
