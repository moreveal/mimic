//go:build windows && amd64

package browser

import (
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestWindowGlobalPublicationOrder(t *testing.T) {
	for _, profile := range []string{"secure", "insecure", "secure-isolated"} {
		t.Run(profile, func(t *testing.T) {
			raw, err := os.ReadFile("../../chrome/152/generated/window-" + profile + "-order.json")
			if err != nil {
				t.Fatal(err)
			}
			var expected []string
			if err = json.Unmarshal(raw, &expected); err != nil {
				t.Fatal(err)
			}
			p := bootstrapSnapshotPage(t)
			address := "http://localhost/"
			if profile == "insecure" {
				address = "http://order.invalid/"
			}
			u, _ := url.Parse(address)
			p.current = u
			p.documentSecurity = documentSecurity{secureContext: profile != "insecure", crossOriginIsolated: profile == "secure-isolated"}
			p.Top.Realm.url = u
			p.Top.Realm.origin = originOf(u.String())
			check := func(source string) {
				t.Helper()
				value := bootstrapSnapshotEvaluate(t, p, source).(string)
				var got []string
				if err := json.Unmarshal([]byte(value), &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, expected) {
					for i := 0; i < len(got) && i < len(expected); i++ {
						if got[i] != expected[i] {
							t.Fatalf("key %d: got %q want %q (length %d/%d)", i, got[i], expected[i], len(got), len(expected))
						}
					}
					t.Fatalf("key count %d want %d", len(got), len(expected))
				}
			}
			check(`JSON.stringify(Object.getOwnPropertyNames(globalThis))`)
			bootstrapSnapshotWarm(t, p)
			check(`(()=>{const f=document.createElement('iframe');document.body.appendChild(f);return f.contentWindow.eval('JSON.stringify(Object.getOwnPropertyNames(globalThis))')})()`)
			value := bootstrapSnapshotEvaluate(t, p, `(()=>{globalThis.orderFirst=1;globalThis.orderSecond=2;const d=Object.getOwnPropertyDescriptor(globalThis,'Option');delete globalThis.Option;Object.defineProperty(globalThis,'Option',d);return JSON.stringify(Object.getOwnPropertyNames(globalThis).slice(-3))})()`)
			if value != `["orderFirst","orderSecond","Option"]` {
				t.Fatalf("dynamic insertion order: %v", value)
			}
		})
	}
}
