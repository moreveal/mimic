package browser

import (
	"context"
	"testing"
	"time"
)

// Measured with frozen Chrome 152.0.7977.82: a closed details descendant
// retains its 100x30 rect but checkVisibility, hit testing and IO exclude it.
// Opening the host changes eligibility, not the descendant's authored style.
func TestBlitzSkippedContentsPreserveRectsButExcludeInputAndObservers(t *testing.T) {
	p := blitzStandardsPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := p.Evaluate(ctx, `(async()=>{
document.body.innerHTML='<details><summary>Title</summary><button style="width:100px;height:30px">Hidden</button></details>';
const e=document.querySelector('button'),d=document.querySelector('details');
const read=()=>{const r=e.getBoundingClientRect();return [r.width,r.height,e.checkVisibility(),document.elementFromPoint(r.x+2,r.y+2)===e]};
const closed=read();
const io=await new Promise(resolve=>{const observer=new IntersectionObserver(es=>{observer.disconnect();resolve(es[0].isIntersecting)});observer.observe(e)});
d.open=true;const opened=read();d.open=false;const reclosed=read();
return JSON.stringify([closed,io,opened,reclosed]);
})()`)
	if err != nil || v != `[[100,30,false,false],false,[100,30,true,true],[100,30,false,false]]` {
		t.Fatalf("skipped content observations=%v error=%v", v, err)
	}
	assertBlitzOwnerActive(t, p)
}
