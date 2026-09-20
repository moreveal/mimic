package browser

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Exact Chrome152 receipts audit old hand-authored expectations. Original tests
// remain unchanged: this is independent evidence, not a blanket green waiver.
func TestBlitzHandwrittenLayoutMatchesChrome152Receipts(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	raw, err := os.ReadFile("../../docs/performance/blitz-chrome152-handwritten-audit-2026-09-20.json")
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Cases []struct{ Test, Source, Result string }
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile("css_box_geometry_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, probe := range receipt.Cases {
		t.Run(probe.Test, func(t *testing.T) {
			// Fail if the audited source drifts: these results are not transferable to
			// a modified fixture without repeating the frozen browser measurement.
			section := strings.SplitN(string(original), "func "+probe.Test+"(", 2)
			if len(section) != 2 {
				t.Fatal("audited test source absent")
			}
			parts := strings.SplitN(section[1], "`", 3)
			if len(parts) != 3 || parts[1] != probe.Source {
				t.Fatal("Chrome receipt source differs from original fixture")
			}
			historyTestPages(t, func(t *testing.T, p *Page) {
				navigateCapabilityFixture(t, p)
				got, err := p.Evaluate(context.Background(), probe.Source)
				if err != nil || got != probe.Result {
					t.Fatalf("Chrome152 got%v want%s err%v", got, probe.Result, err)
				}
				if p.Top.Realm.blitz == nil || p.Top.Realm.blitz.document.Owner == nil || p.Top.Realm.blitz.fallback != "" {
					t.Fatal("native Chrome receipt gate fell back")
				}
			})
		})
	}
}
