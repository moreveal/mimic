package browser

import (
	"context"
	"fmt"
	"testing"
)

// Observed with Chrome 152.0.7977.84 on a standards-mode document.
func TestOffsetParentMatchesChrome152(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{
document.body.innerHTML='<div id="normal"><a id="link">x</a></div><div id="positioned" style="position:relative"><a id="nested">x</a></div><div id="hidden" style="display:none"><a id="insideHidden">x</a></div><div id="fixed" style="position:fixed">x</div><table id="table"><tr><td id="cell"><span id="inCell">x</span></td></tr></table>';
return ['html','body','normal','link','positioned','nested','hidden','insideHidden','fixed','table','cell','inCell'].map(id=>{const e=id==='html'?document.documentElement:id==='body'?document.body:document.getElementById(id);return e.offsetParent?.id||e.offsetParent?.localName||null}).join(',')
})()`)
		if err != nil || fmt.Sprint(value) != ",,body,body,body,positioned,,,,body,table,cell" {
			t.Fatalf("offset parents: %v %v", value, err)
		}
	})
}
