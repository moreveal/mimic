package browser

import (
	"context"
	"strconv"
	"testing"
	"time"
)

const canvasStateFixture = `(async()=>{
 const canvas=new OffscreenCanvas(3,2),ctx=canvas.getContext('2d');
 if(ctx!==canvas.getContext('2d')||ctx.canvas!==canvas||canvas.getContext('webgpu')!==null)return 'context identity';
 if(ctx.fillStyle!=='#000000')return 'default';ctx.fillStyle='red';ctx.save();ctx.fillStyle='blue';ctx.scale(2,3);ctx.restore();if(ctx.fillStyle!=='#ff0000'||ctx.getTransform().a!==1)return 'state restore';
 ctx.fillRect(0,0,2,1);if(Array.from(ctx.getImageData(0,0,3,1).data).join(',')!=='255,0,0,255,255,0,0,255,0,0,0,0')return 'fill pixels';
 ctx.clearRect(1,0,1,1);if(ctx.getImageData(1,0,1,1).data[3]!==0)return 'clear pixels';
 const bytes=new Uint8ClampedArray([1,2,3,4,5,6,7,8]),image=new ImageData(bytes,2);if(image.data!==bytes)return 'image identity';ctx.putImageData(image,0,1);if(Array.from(ctx.getImageData(0,1,2,1).data).join(',')!=='0,0,0,4,0,0,0,8')return 'premul';
 const snapshot=await createImageBitmap(canvas);if(snapshot.width!==3||snapshot.height!==2)return 'snapshot';snapshot.close();if(snapshot.width||snapshot.height)return 'close';
 ctx.fillStyle='blue';const bitmap=canvas.transferToImageBitmap();if(bitmap.width!==3||ctx.fillStyle!=='#0000ff'||ctx.getImageData(0,0,1,1).data[3]!==0)return 'transfer bitmap';
 ctx.scale(2,2);canvas.width=3;if(ctx!==canvas.getContext('2d')||ctx.fillStyle!=='#000000'||ctx.getTransform().a!==1)return 'resize';
 const other=new OffscreenCanvas(3,2);if(other.getContext('2d')===ctx)return 'isolation';
 return true;
})()`

func TestCanvasStateWindowAndWorker(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		for _, worker := range []bool{false, true} {
			script := canvasStateFixture
			if worker {
				code := "onmessage=async()=>{try{postMessage(await " + script + ")}catch(e){postMessage(String(e))}}"
				script = `new Promise((resolve,reject)=>{const url=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `]));const w=new Worker(url);w.onerror=e=>reject(Error(e.message));w.onmessage=e=>{w.terminate();URL.revokeObjectURL(url);resolve(e.data)};w.postMessage(null)})`
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			value, err := p.Evaluate(ctx, script)
			cancel()
			if err != nil || value != true {
				t.Fatalf("worker=%v: %v %v", worker, value, err)
			}
		}
	})
}

func TestHTMLCanvasUsesCanonicalContextState(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{const c=document.createElement('canvas');if(c.width!==300||c.height!==150)return 'dimensions';const ctx=c.getContext('2d');c.width=2;c.height=2;ctx.fillStyle='red';ctx.fillRect(0,0,1,1);if(ctx.getImageData(0,0,1,1).data[0]!==255)return 'pixels';c.width=2;return ctx===c.getContext('2d')&&ctx.fillStyle==='#000000'&&ctx.getImageData(0,0,1,1).data[3]===0})()`)
		if err != nil || value != true {
			t.Fatalf("HTML canvas: %v %v", value, err)
		}
	})
}

func TestCanvasTextObservationsAreLocalAndOrdered(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(async()=>{
 const c=new OffscreenCanvas(80,24),x=c.getContext('2d');x.font='10px sans-serif';
 const run=text=>{x.clearRect(0,0,80,24);x.fillText(text,2,12);return Array.from(x.getImageData(0,0,80,24).data)};
 const a=run('AAAA'),b=run('AAAB'),again=run('AAAA');if(a.join(',')!==again.join(','))return 'repeatability';let changes=0;const prefix=x.measureText('AAA').width,last=Math.max(x.measureText('A').width,x.measureText('B').width);
 for(let i=0;i<a.length;i++){if(a[i]===b[i])continue;changes++;const px=Math.floor(i/4)%80;if(px<Math.floor(2+prefix)-1||px>Math.ceil(2+prefix+last)+1)return 'nonlocal change'}if(!changes)return 'character ignored';
 const crop=x.getImageData(5,4,20,10).data;for(let y=0;y<10;y++)for(let xx=0;xx<20;xx++)for(let k=0;k<4;k++)if(crop[(y*20+xx)*4+k]!==again[((y+4)*80+xx+5)*4+k])return 'overlap';
 const snap=await createImageBitmap(c),copy=new OffscreenCanvas(80,24),cx=copy.getContext('2d');cx.drawImage(snap,0,0);if(Array.from(cx.getImageData(0,0,80,24).data).join(',')!==again.join(','))return 'snapshot';
 const cropped=await createImageBitmap(c,5,4,20,10),cc=new OffscreenCanvas(20,10),ccx=cc.getContext('2d');if(cropped.width!==20||cropped.height!==10)return 'crop dimensions';ccx.drawImage(cropped,0,0);if(Array.from(ccx.getImageData(0,0,20,10).data).join(',')!==Array.from(crop).join(','))return 'crop snapshot';
 x.clearRect(0,0,80,24);x.fillText('A',2,12);x.clearRect(0,0,80,24);if(x.getImageData(0,0,80,24).data.some(v=>v))return 'clear ordering';
 x.fillText('A',2,12);x.putImageData(new ImageData(80,24),0,0);if(x.getImageData(0,0,80,24).data.some(v=>v))return 'put ordering';
 const width=x.measureText('AAAA').width;if(width!==x.measureText('AAAB').width||width<=0)return 'metrics';
 for(const text of ['', '   ']){const m=x.measureText(text);if(m.actualBoundingBoxLeft||m.actualBoundingBoxRight||m.actualBoundingBoxAscent||m.actualBoundingBoxDescent)return 'empty ink'}
 x.letterSpacing='2px';if(x.measureText('AAAA').width!==width+8)return 'spacing';x.font='invalid';if(x.font!=='10px sans-serif')return 'font validation';
 c.width=80;if(x.getImageData(0,0,80,24).data.some(v=>v))return 'reset';
 const opaque=new OffscreenCanvas(2,2).getContext('2d',{alpha:false});if(opaque.getImageData(0,0,1,1).data[3]!==255)return 'opaque init';opaque.clearRect(0,0,2,2);if(opaque.getImageData(0,0,1,1).data[3]!==255)return 'opaque clear';
 if(x instanceof CanvasRenderingContext2D||!(x instanceof OffscreenCanvasRenderingContext2D))return 'prototype';
 if(Function.prototype.toString.call(OffscreenCanvas.prototype.getContext)!=='function getContext() { [native code] }')return 'native source';
 return true;
 })()`)
		if err != nil || value != true {
			t.Fatalf("text observations: %v %v", value, err)
		}
	})
}
