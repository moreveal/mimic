package browser

import (
	"context"
	"testing"
	"time"
)

func TestDebuggerIsolatedWorldSharesDOMWithoutSharingGlobals(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		d := NewDebugger(page)
		defer d.Close()
		debuggerEval(t, d, `globalThis.mainMarker=7;document.body.setAttribute('data-shared','main');document.body.mainExpando=1`, DebuggerOptions{})
		world, err := page.IsolatedWorld(ctx, page.Top.ID, "automation")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(ctx, page.Top.ID, world, `globalThis.worldMarker=11;document.body.setAttribute('data-world','yes');[typeof mainMarker,document.body.getAttribute('data-shared'),document.body.mainExpando,document.body instanceof HTMLElement,document.defaultView===window]`, DebuggerOptions{ReturnByValue: true})
		if err != nil {
			t.Fatal(err)
		}
		if got := result["result"].(map[string]any)["value"].([]any); got[0] != "undefined" || got[1] != "main" || got[2] != nil || got[3] != true || got[4] != true {
			t.Fatalf("isolated observations: %#v", result)
		}
		result = map[string]any{"result": debuggerEval(t, d, `[typeof worldMarker,document.body.getAttribute('data-world')]`, DebuggerOptions{ReturnByValue: true})}
		if got := result["result"].(map[string]any)["value"].([]any); got[0] != "undefined" || got[1] != "yes" {
			t.Fatal(result)
		}
		again, err := page.IsolatedWorld(ctx, page.Top.ID, "automation")
		if err != nil || again != world {
			t.Fatalf("world identity %s %s %v", world, again, err)
		}
	})
}

func TestDebuggerWorldMutationObserversUseCanonicalRecords(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		d := NewDebugger(page)
		defer d.Close()
		world, err := page.IsolatedWorld(ctx, page.Top.ID, "observer")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(ctx, page.Top.ID, world, `new Promise(resolve=>{new MutationObserver(records=>resolve(records.map(record=>[record.type,record.target===document.body,record.addedNodes[0]===document.querySelector('p')]))).observe(document.body,{childList:true})})`, DebuggerOptions{})
		if err != nil {
			t.Fatal(err)
		}
		promise := result["result"].(map[string]any)["objectId"].(string)
		debuggerEval(t, d, `document.body.appendChild(document.createElement('p'))`, DebuggerOptions{})
		result, err = d.AwaitPromise(ctx, promise, DebuggerOptions{ReturnByValue: true})
		if err != nil {
			t.Fatal(err)
		}
		got := result["result"].(map[string]any)["value"].([]any)[0].([]any)
		if got[0] != "childList" || got[1] != true || got[2] != true {
			t.Fatalf("mutation: %#v", result)
		}
	})
}

func TestDebuggerWorldDocumentWriteExecutesInDocumentMainRealm(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		d := NewDebugger(page)
		defer d.Close()
		world, err := page.IsolatedWorld(ctx, page.Top.ID, "content")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(ctx, page.Top.ID, world, `document.open();document.write('<body><script>globalThis.parserMarker=42<\/script><iframe name="child"></iframe></body>');document.close();typeof parserMarker`, DebuggerOptions{})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != "undefined" {
			t.Fatalf("world parser scope: %#v %v", result, err)
		}
		if got := debuggerEval(t, d, `parserMarker`, DebuggerOptions{}); got["value"] != float64(42) {
			t.Fatal(got)
		}
		if got := debuggerEval(t, d, `document.querySelector('iframe').contentWindow.name`, DebuggerOptions{}); got["value"] != "child" {
			t.Fatal(got)
		}
	})
}

func TestIframeNameReflectsCanonicalAttributeAndBrowsingContext(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		got := debuggerEval(t, d, `const named=document.createElement('iframe');named.name='initial';document.body.appendChild(named);const names=[named.getAttribute('name'),named.contentWindow.name];named.name='updated';names.push(named.contentWindow.name);names.join(',')`, DebuggerOptions{})
		if got["value"] != "initial,initial,initial" {
			t.Fatal(got)
		}
	})
}

func TestDebuggerIsolatedWorldChildWindowPreservesWorldBoundary(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		d := NewDebugger(page)
		defer d.Close()
		debuggerEval(t, d, `globalThis.childFrame=document.createElement('iframe');document.body.append(childFrame);childFrame.contentWindow.mainOnly=42`, DebuggerOptions{})
		world, err := page.IsolatedWorld(ctx, page.Top.ID, "boundary")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(ctx, page.Top.ID, world, `const child=document.querySelector('iframe').contentWindow;child.utilityOnly=7;[typeof child.mainOnly,child===frames[0],child.eval('typeof mainOnly'),child.parent===window]`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil {
			t.Fatalf("world child %#v %v", result, err)
		}
		got := result["result"].(map[string]any)["value"].([]any)
		if got[0] != "undefined" || got[1] != true || got[2] != "undefined" || got[3] != true {
			t.Fatal(got)
		}
		if got := debuggerEval(t, d, `typeof childFrame.contentWindow.utilityOnly`, DebuggerOptions{}); got["value"] != "undefined" {
			t.Fatal(got)
		}
	})
}
