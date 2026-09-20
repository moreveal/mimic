package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBlitzSVGIntrinsicChrome152(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	raw, err := os.ReadFile("../../docs/performance/blitz-svg-intrinsic-chrome152-2026-09-20.json")
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		BodyStyle, FixtureTemplate string
		Cases                      []struct {
			Display, Attributes    string
			Parent, SVG, Following []float64
		}
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		for _, tc := range receipt.Cases {
			t.Run(tc.Display+"/"+tc.Attributes, func(t *testing.T) {
				html := strings.ReplaceAll(strings.ReplaceAll(receipt.FixtureTemplate, "DISPLAY", tc.Display), "ATTRIBUTES", tc.Attributes)
				encoded, _ := json.Marshal(html)
				style, _ := json.Marshal(receipt.BodyStyle)
				value, err := p.Evaluate(context.Background(), `(()=>{document.body.style.cssText=`+string(style)+`;document.body.innerHTML=`+string(encoded)+`;return JSON.stringify(['p','s','t'].map(id=>{const r=document.getElementById(id).getBoundingClientRect();return [r.x,r.y,r.width,r.height]}))})()`)
				if err != nil {
					t.Fatal(err)
				}
				var got [][]float64
				if err := json.Unmarshal([]byte(value.(string)), &got); err != nil {
					t.Fatal(err)
				}
				want := [][]float64{tc.Parent, tc.SVG, tc.Following}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("got%v want%v", got, want)
				}
				if p.Top.Realm.blitz == nil || p.Top.Realm.blitz.document.Owner == nil || p.Top.Realm.blitz.fallback != "" {
					t.Fatal("SVG intrinsic test fell back")
				}
			})
		}
	})
}
