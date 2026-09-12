package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestSVGAuditDifferential(t *testing.T) {
	for _, name := range []string{"svg-surface", "svg-values", "svg-reflections", "svg-coordinates", "svg-text-positions", "svg-use", "svg-path-metrics", "svg-attributes", "svg-edge-cases", "svg-states", "svg-mutations", "svg-dom-matrix", "svg-zoom", "svg-stroke"} {
		t.Run(name, func(t *testing.T) {
			parallelOracle(t)
			source, err := os.ReadFile("testdata/" + strings.ReplaceAll(name, "-", "_") + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + name + "-chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Result any `json:"result"`
			}
			if err = json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			historyTestPages(t, func(t *testing.T, p *Page) {
				navigateCapabilityFixture(t, p)
				v, err := p.Evaluate(context.Background(), "JSON.stringify("+string(source)+")")
				if err != nil {
					t.Fatal(err)
				}
				var actual any
				if err = json.Unmarshal([]byte(v.(string)), &actual); err != nil {
					t.Fatal(err)
				}
				var diffs []string
				var compare func(string, any, any)
				compare = func(path string, a, b any) {
					if strings.HasPrefix(path, "svg-path-metrics/") && strings.HasSuffix(path, "/length") {
						aa, ok := a.(float64)
						bb, bok := b.(float64)
						if ok && bok && strings.Contains(path, "/circle/") && math.Abs(aa-bb) < .5 {
							return
						}
						if ok && bok && strings.Contains(path, "/ellipse/") && math.Abs(aa-bb) < .7 {
							return
						}
						if ok && bok && strings.Contains(path, "/arc/") && math.Abs(aa-bb) < .03 {
							return
						}
						if ok && bok && (strings.Contains(path, "/curve/") || strings.Contains(path, "/cubic/")) && math.Abs(aa-bb) < .003 {
							return
						}
					}
					if strings.Contains(path, "/matrixOps") {
						aa, ok := a.(float64)
						bb, bok := b.(float64)
						if ok && bok && math.Abs(aa-bb) <= 1e-14 {
							return
						}
					}
					if strings.Contains(path, "/matrixOps") {
						aa, ok := a.([]any)
						bb, bok := b.([]any)
						if ok && bok && len(aa) == len(bb) {
							for i := range aa {
								compare(fmt.Sprintf("%s/%d", path, i), aa[i], bb[i])
							}
							return
						}
					}
					if reflect.DeepEqual(a, b) {
						return
					}
					if strings.HasSuffix(path, "/members") {
						aa, aok := a.([]any)
						bb, bok := b.([]any)
						if aok && bok {
							am, bm := map[string]any{}, map[string]any{}
							for _, row := range aa {
								x := row.([]any)
								am[x[0].(string)] = x[1:]
							}
							for _, row := range bb {
								x := row.([]any)
								bm[x[0].(string)] = x[1:]
							}
							compare(path+"/descriptors", am, bm)
							return
						}
					}
					am, ok := a.(map[string]any)
					bm, bok := b.(map[string]any)
					if ok && bok {
						keys := map[string]bool{}
						for k := range am {
							keys[k] = true
						}
						for k := range bm {
							keys[k] = true
						}
						for k := range keys {
							compare(path+"/"+k, am[k], bm[k])
						}
						return
					}
					diffs = append(diffs, fmt.Sprintf("%s: got %v want %v", path, a, b))
				}
				compare(name, actual, capture.Result)
				sort.Strings(diffs)
				if len(diffs) > 0 {
					for i, d := range diffs {
						if i >= 20 {
							break
						}
						t.Log(d)
					}
					t.Fatalf("%d differential groups differ", len(diffs))
				}
			})
		})
	}
}

func TestSVGAuditExplicitBoundaries(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{const ns='http://www.w3.org/2000/svg',out=[];for(const [tag,invoke]of [['animate',n=>n.getStartTime()],['svg',n=>n.getIntersectionList(n.createSVGRect(),null)]]){const n=document.createElementNS(ns,tag);try{invoke(n);out.push('silent success')}catch(e){out.push(e.name)}}return JSON.stringify(out)})()`)
		if err != nil {
			t.Fatal(err)
		}
		if value != `["NotSupportedError","NotSupportedError"]` {
			t.Fatalf("unsupported API silently succeeded: %v", value)
		}
		seen := map[string]bool{}
		for _, e := range p.Trace().Events() {
			if strings.HasPrefix(e.Name, "SVG.") {
				if e.Data["reasonAvailable"] != true || e.Data["site"] == "legacy-host" {
					t.Errorf("missing boundary reason: %v", e.Data)
				}
				seen[e.Name] = true
			}
		}
		for _, name := range []string{"SVG.SVGAnimationElement.getStartTime", "SVG.SVGSVGElement.getIntersectionList"} {
			if !seen[name] {
				t.Errorf("missing diagnostic %s", name)
			}
		}
	})
}
