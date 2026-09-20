package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSelectorMatchCacheUsesCanonicalMutationJournal(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, `document.head.innerHTML='<style>.probe{width:20px}.probe[data-mode="on"]{width:40px}#scope[data-wide="on"] .probe{width:60px}</style>';document.body.innerHTML='<div id="scope"><div id="probe" class="probe"></div></div><div id="other"></div>'`, DebuggerOptions{})
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "selector-journal")
		if err != nil {
			t.Fatal(err)
		}
		read := func(want string) {
			t.Helper()
			result, err := d.Evaluate(context.Background(), p.Top.ID, world, `getComputedStyle(document.getElementById('probe')).width`, DebuggerOptions{ReturnByValue: true})
			if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != want {
				t.Fatalf("computed width: %#v %v; want %s", result, err, want)
			}
		}
		read("20px")
		// A mutation from the canonical owner realm to another node cannot affect
		// these local-only selectors and may reuse the retained match array.
		debuggerEval(t, d, `document.getElementById('other').setAttribute('data-unused','1')`, DebuggerOptions{})
		read("20px")
		// A relevant target mutation must reject the retained result across realms.
		debuggerEval(t, d, `document.getElementById('probe').setAttribute('data-mode','on')`, DebuggerOptions{})
		read("40px")
		debuggerEval(t, d, `document.getElementById('probe').removeAttribute('data-mode')`, DebuggerOptions{})
		read("20px")
		// Complex selectors are reusable only across mutations of attributes that
		// are absent from their complete selector program. A relevant ancestor
		// mutation therefore rejects the retained match array.
		debuggerEval(t, d, `document.getElementById('scope').setAttribute('data-wide','on')`, DebuggerOptions{})
		read("60px")
	})
}

func TestSelectorMatchCacheFallsBackAfterJournalOverflow(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, `document.head.innerHTML='<style>.probe{width:20px}.probe[data-mode="on"]{width:40px}</style>';document.body.innerHTML='<div id="probe" class="probe"></div><div id="other"></div>'`, DebuggerOptions{})
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "selector-overflow")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Evaluate(context.Background(), p.Top.ID, world, `getComputedStyle(document.getElementById('probe')).width`, DebuggerOptions{ReturnByValue: true}); err != nil {
			t.Fatal(err)
		}
		debuggerEval(t, d, `for(let i=0;i<300;i++)document.getElementById('other').setAttribute('data-unused',String(i));document.getElementById('probe').setAttribute('data-mode','on')`, DebuggerOptions{})
		result, err := d.Evaluate(context.Background(), p.Top.ID, world, `getComputedStyle(document.getElementById('probe')).width`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != "40px" {
			t.Fatalf("overflow fallback: %#v %v", result, err)
		}
	})
}

func TestSelectorMatchCacheDoesNotSurviveNavigation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		width := "20px"
		if r.URL.Path == "/second" {
			width = "70px"
		}
		fmt.Fprintf(w, `<!doctype html><style>.probe{width:%s}</style><div class="probe"></div>`, width)
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/first"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `getComputedStyle(document.querySelector('.probe')).width`, "20px")
		if err := p.Navigate(context.Background(), server.URL+"/second"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `getComputedStyle(document.querySelector('.probe')).width`, "70px")
	})
}
