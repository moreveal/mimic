package browser

import (
	"context"
	"testing"
)

// A floated inline child yields control to Parley's advanced line breaker.
// The simple break_all_lines path cannot consume that yield and used to spin.
func TestBlitzFloatedInlineChildCompletesLayout(t *testing.T) {
	parallelBrowserTest(t)
	p := blitzStandardsPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
		document.head.innerHTML = '<style>body{margin:0}.pager{width:600px;margin:0;padding:0;list-style:none}.pager li{display:inline}.pager .next{float:right}</style>';
		document.body.innerHTML = '<ul class="pager"><li class="current">Page 1 of 50</li><li class="next"><a href="#next">Next</a></li></ul>';
		const pager = document.querySelector('.pager').getBoundingClientRect();
		const current = document.querySelector('.current').getBoundingClientRect();
		const next = document.querySelector('.next').getBoundingClientRect();
		return JSON.stringify([pager.width, next.x > current.x, Number.isFinite(pager.height)]);
	})()`)
	if err != nil || value != `[600,true,true]` {
		t.Fatalf("floated pager layout=%v error=%v", value, err)
	}
	assertBlitzOwnerActive(t, p)
}
