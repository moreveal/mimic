package browser

import (
	"context"
	"encoding/json"
	"testing"
)

func liveDiagnosticCost(t *testing.T, page *Page, name string) uint64 {
	t.Helper()
	encoded, err := json.Marshal(page.LiveDiagnostics())
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	var total uint64
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if costs, ok := value["costs"].(map[string]any); ok {
				if cost, ok := costs[name].(map[string]any); ok {
					total += uint64(cost["count"].(float64))
				}
			}
			for key, child := range value {
				if key != "costs" {
					walk(child)
				}
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(value)
	return total
}

func TestIsolatedStyleObservationsUseCanonicalOwner(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, `document.head.innerHTML='<style>#probe{width:20px}#probe:checked{width:40px}</style>';document.body.innerHTML='<input id="probe" type="checkbox">'`, DebuggerOptions{})
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "style-observer")
		if err != nil {
			t.Fatal(err)
		}
		read := func(want string) {
			t.Helper()
			result, err := d.Evaluate(context.Background(), p.Top.ID, world, `(()=>{const e=document.getElementById('probe');return JSON.stringify([getComputedStyle(e).width,e.checkVisibility(),e.getBoundingClientRect().width,e===document.getElementById('probe')])})()`, DebuggerOptions{ReturnByValue: true})
			if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != want {
				t.Fatalf("owner observation: %#v, %v; want %s", result, err, want)
			}
		}
		read(`["20px",true,20,true]`)
		debuggerEval(t, d, `document.styleSheets[0].insertRule('#probe{width:60px}',2)`, DebuggerOptions{})
		read(`["60px",true,60,true]`)
		debuggerEval(t, d, `document.getElementById('probe').style.display='none'`, DebuggerOptions{})
		read(`["60px",false,0,true]`)
		debuggerEval(t, d, `document.getElementById('probe').style.display='';document.styleSheets[0].deleteRule(2);document.getElementById('probe').checked=true`, DebuggerOptions{})
		read(`["40px",true,40,true]`)
		debuggerEval(t, d, `Element.prototype.checkVisibility=()=>{throw new Error('main-world override')}`, DebuggerOptions{})
		read(`["40px",true,40,true]`)
	})
}

func TestIsolatedStyleObservationInitializesDeferredOwner(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "initial-style")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(context.Background(), p.Top.ID, world, `document.body.checkVisibility()`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != true {
			t.Fatalf("initial owner: %#v %v", result, err)
		}
	})
}

func TestIsolatedStyleBatchesLargeStableDocument(t *testing.T) {
	t.Setenv("MIMIC_PROFILE_HOSTS", "1")
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, `document.head.innerHTML='<style>.item{display:block}.item.hidden{display:none}</style>';document.body.innerHTML=Array.from({length:160},(_,i)=>'<div class="item '+(i%2?'hidden':'')+'"></div>').join('')`, DebuggerOptions{})
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "style-batch")
		if err != nil {
			t.Fatal(err)
		}
		before := liveDiagnosticCost(t, p, "host:foreignComputedStyleFlatTree")
		result, err := d.Evaluate(context.Background(), p.Top.ID, world, `Array.from(document.querySelectorAll('.item'),e=>getComputedStyle(e).display).filter(v=>v==='none').length`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != float64(80) {
			t.Fatalf("batched styles: %#v %v", result, err)
		}
		after := liveDiagnosticCost(t, p, "host:foreignComputedStyleFlatTree")
		if calls := after - before; calls > 2 {
			t.Fatalf("large stable style read used %d owner crossings", calls)
		}
		debuggerEval(t, d, `document.styleSheets[0].insertRule('.item{display:inline}',2)`, DebuggerOptions{})
		result, err = d.Evaluate(context.Background(), p.Top.ID, world, `getComputedStyle(document.querySelector('.item')).display`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != "inline" {
			t.Fatalf("post-mutation style: %#v %v", result, err)
		}
	})
}

func TestIsolatedInnerTextTracksOwnerMutations(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, `document.body.innerHTML='<div id="probe"><span>A</span><span style="display:none">B</span></div>'`, DebuggerOptions{})
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "text-observer")
		if err != nil {
			t.Fatal(err)
		}
		read := func(want string) {
			t.Helper()
			result, err := d.Evaluate(context.Background(), p.Top.ID, world, `document.getElementById('probe').innerText`, DebuggerOptions{ReturnByValue: true})
			if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != want {
				t.Fatalf("innerText: %#v %v; want %q", result, err, want)
			}
		}
		read("A")
		debuggerEval(t, d, `document.querySelector('#probe span:last-child').style.display='inline'`, DebuggerOptions{})
		read("AB")
		debuggerEval(t, d, `document.querySelector('#probe span').textContent='C'`, DebuggerOptions{})
		read("CB")
		result, err := d.Evaluate(context.Background(), p.Top.ID, world, `(()=>{const inert=document.implementation.createHTMLDocument(''),node=inert.createElement('div');node.textContent='detached';return node.innerText})()`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != "detached" {
			t.Fatalf("inert innerText: %#v %v", result, err)
		}
	})
}
