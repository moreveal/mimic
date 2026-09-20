package browser

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// This diagnostic isolates the page operations visible in the captured
// Wikipedia trace. Run with -v to inspect timings and host crossing counts.
func TestWikipediaHotPathProfile(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_PROFILE_HOSTS", "1")
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	eval := func(label, source string) {
		t.Helper()
		beforeReady := countTraceName(p, "Document.readyStateValue")
		beforeQuery := liveDiagnosticCost(t, p, "host:queryAllWithin")
		started := time.Now()
		value, err := p.Evaluate(ctx, source)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s result=%v readyState=%d queryAllWithin=%d", label, time.Since(started), value,
			countTraceName(p, "Document.readyStateValue")-beforeReady,
			liveDiagnosticCost(t, p, "host:queryAllWithin")-beforeQuery)
	}
	eval("setup", `document.body.innerHTML=Array.from({length:8},(_,i)=>'<table><tbody>'+Array.from({length:35},(_,j)=>'<tr><td>Some text '+i+' '+j+'</td><td>Other text</td></tr>').join('')+'</tbody></table>').join('')+'<h1 id="heading">Heading</h1>'; true`)
	p.Top.Realm.SetReadyState("complete")
	eval("children walk", `(()=>{let n=0;for(const table of document.body.children)for(const body of table.children)for(const row of body.children)n+=row.children.length;return n})()`)
	var finish func() (any, error)
	if path := os.Getenv("MIMIC_HOTPATH_NATIVE_PROFILE"); path != "" {
		finish, err = p.Top.Realm.runtime.(interface {
			ProfileWorkloadCPU() (func() (any, error), error)
		}).ProfileWorkloadCPU()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			profile, err := finish()
			if err != nil {
				t.Error(err)
				return
			}
			data, err := json.Marshal(profile)
			if err == nil {
				err = os.WriteFile(path, data, 0644)
			}
			if err != nil {
				t.Error(err)
			}
		}()
	}
	eval("scroll height", `document.body.scrollHeight`)
	eval("table geometry", `(()=>{let n=0;for(const table of document.querySelectorAll('table'))n+=table.getBoundingClientRect().height;return n})()`)
	eval("heading visibility", `(()=>{const e=document.getElementById('heading');return [e.checkVisibility(),e.getBoundingClientRect().width,e.offsetParent]})()`)
}

func countTraceName(p *Page, name string) int {
	n := 0
	for _, event := range p.Trace().Events() {
		if event.Name == name {
			n++
		}
	}
	return n
}
