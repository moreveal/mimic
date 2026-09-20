package browser

import (
	"context"
	"testing"
)

const blitzCaptionLifecycle = `(()=>{
document.head.innerHTML='<style>body{margin:0}table{border-spacing:0}td{padding:0;width:80px;height:20px}caption{height:30px}#after{height:5px}</style>';
document.body.innerHTML='<table><caption></caption><tbody><tr><td></td></tr></tbody></table><div id="after"></div>';
const table=document.querySelector('table'),caption=document.querySelector('caption'),cell=document.querySelector('td'),after=document.querySelector('#after');
const read=()=>[table,caption,cell,after].map(e=>{const r=e.getBoundingClientRect();return [r.x,r.y,r.width,r.height]});
const result=[read()];
caption.style.captionSide='bottom';result.push(read());
caption.style.width='160px';result.push(read());
caption.style.margin='5px 7px';result.push(read());
caption.remove();result.push([table,cell,after].map(e=>{const r=e.getBoundingClientRect();return [r.x,r.y,r.width,r.height]}));
table.prepend(caption);result.push(read());
return JSON.stringify(result);
})()`

func TestBlitzCaptionLayoutLifecycle(t *testing.T) {
	p := blitzStandardsPage(t)
	v, err := p.Evaluate(context.Background(), blitzCaptionLifecycle)
	// Captured against frozen Chrome 152.0.7977.82, profile 1272px viewport.
	if err != nil || v != `[[[0,0,80,50],[0,0,80,30],[0,30,80,20],[0,50,1272,5]],[[0,0,80,50],[0,20,80,30],[0,0,80,20],[0,50,1272,5]],[[0,0,160,50],[0,20,160,30],[0,0,160,20],[0,50,1272,5]],[[0,0,174,60],[7,25,160,30],[0,0,174,20],[0,60,1272,5]],[[0,0,80,20],[0,0,80,20],[0,20,1272,5]],[[0,0,174,60],[7,25,160,30],[0,0,174,20],[0,60,1272,5]]]` {
		t.Fatalf("caption lifecycle=%v error=%v", v, err)
	}
	assertBlitzOwnerActive(t, p)
}
