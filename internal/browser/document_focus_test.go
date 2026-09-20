package browser

import (
	"context"
	"testing"
)

func TestFocusEmulationProjectsIntoDocument(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, page *Page) {
		page.mu.Lock()
		page.pageFocused = false
		page.mu.Unlock()
		check := func(want bool) {
			value, err := page.Evaluate(context.Background(), "document.hasFocus()")
			if err != nil || value != want {
				t.Fatalf("document.hasFocus()=%v, want %v: %v", value, want, err)
			}
		}
		check(false)
		page.SetFocusEmulationEnabled(true)
		check(true)
		page.SetFocusEmulationEnabled(false)
		check(false)
	})
}
