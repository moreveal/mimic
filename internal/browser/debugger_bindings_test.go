package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDebuggerBindingsPersistAcrossDocumentsAndRemoveOnlySubscription(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx := context.Background()
		d := NewDebugger(page)
		defer d.Close()
		var received []string
		d.BindingCalled = func(realmID, name, payload string) { received = append(received, name+":"+payload) }
		if err := d.AddBinding(ctx, "automationBinding", "", "", false); err != nil {
			t.Fatal(err)
		}
		debuggerEval(t, d, `automationBinding('first')`, DebuggerOptions{})
		fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`<script>automationBinding('second')</script>`))
		}))
		defer fixture.Close()
		if err := page.Navigate(ctx, fixture.URL); err != nil {
			t.Fatal(err)
		}
		if len(received) != 2 || received[0] != "automationBinding:first" || received[1] != "automationBinding:second" {
			t.Fatal(received)
		}
		d.RemoveBinding("automationBinding")
		if got := debuggerEval(t, d, `automationBinding('ignored');typeof automationBinding`, DebuggerOptions{}); got["value"] != "function" {
			t.Fatal(got)
		}
		if len(received) != 2 {
			t.Fatal(received)
		}
	})
}

func TestDebuggerBindingExecutionContextNameMatchesFutureWorld(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx := context.Background()
		d := NewDebugger(page)
		defer d.Close()
		var received []string
		d.BindingCalled = func(realmID, name, payload string) { received = append(received, payload) }
		if err := d.AddBinding(ctx, "worldBinding", "", "utility", true); err != nil {
			t.Fatal(err)
		}
		if got := debuggerEval(t, d, `typeof worldBinding`, DebuggerOptions{}); got["value"] != "undefined" {
			t.Fatal(got)
		}
		world, err := page.IsolatedWorld(ctx, page.Top.ID, "utility")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(ctx, page.Top.ID, world, `worldBinding('utility');typeof worldBinding`, DebuggerOptions{})
		if err != nil || result["result"].(map[string]any)["value"] != "function" || len(received) != 1 || received[0] != "utility" {
			t.Fatalf("binding %#v %#v %v", result, received, err)
		}
	})
}
