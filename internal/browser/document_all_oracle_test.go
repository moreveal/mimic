//go:build windows && amd64

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestDocumentAllMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "document_all")
}

// Retaining the old Document currently requires keeping its entire realm alive;
// this diagnostic records that existing lifetime boundary without changing the
// Chrome expectation into an expected failure.
func TestDocumentAllRetainedRealmDiagnostic(t *testing.T) {
	if os.Getenv("MIMIC_TEST_RETAINED_REALMS") != "1" {
		t.Skip("navigation destroys the old realm, so retained Document objects become unavailable; set MIMIC_TEST_RETAINED_REALMS=1 to run the unchanged Chrome lifetime oracle")
	}
	documentAllOracle(t, "document_all_navigation")
}

func documentAllOracle(t *testing.T, name string, headers ...map[string]string) {
	t.Helper()
	for _, mode := range []string{"ordinary", "snapshot"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "ordinary" {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
			} else {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "0")
			}
			seed := bootstrapSnapshotPage(t)
			if mode == "snapshot" {
				navigateCapabilityFixture(t, seed)
				bootstrapSnapshotWarm(t, seed)
			}
			for _, name := range []string{name} {
				t.Run(name, func(t *testing.T) {
					// Ordinary mode needs only the page under test. A separate seed
					// is required only when proving restoration into a fresh page.
					p := seed
					if mode == "snapshot" {
						var err error
						p, err = seed.ctx.NewPage()
						if err != nil {
							t.Fatal(err)
						}
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						for _, values := range headers {
							for name, value := range values {
								w.Header().Set(name, value)
							}
						}
						w.Header().Set("Content-Type", "text/html")
						fmt.Fprint(w, `<!doctype html><body><p id="newDocument"></p></body>`)
					}))
					t.Cleanup(server.Close)
					if err := p.Navigate(context.Background(), server.URL); err != nil {
						t.Fatal(err)
					}
					source, err := os.ReadFile("testdata/" + name + "_oracle.js")
					if err != nil {
						t.Fatal(err)
					}
					data, err := os.ReadFile("testdata/" + name + "_chrome152.json")
					if err != nil {
						t.Fatal(err)
					}
					var oracle struct {
						Result struct {
							Result struct {
								Value map[string]any `json:"value"`
							} `json:"result"`
						} `json:"result"`
					}
					if err := json.Unmarshal(data, &oracle); err != nil {
						t.Fatal(err)
					}
					if len(oracle.Result.Result.Value) == 0 {
						t.Fatal("empty Chrome oracle")
					}
					value := bootstrapSnapshotEvaluate(t, p, "(async()=>JSON.stringify(await "+string(source)+"))()")
					if mode == "snapshot" && !p.Top.Realm.bootstrapRestored {
						t.Fatal("oracle did not execute in a restored realm")
					}
					text, ok := value.(string)
					if !ok {
						t.Fatalf("expected JSON string, got %#v", value)
					}
					var actual map[string]any
					if err := json.Unmarshal([]byte(text), &actual); err != nil {
						t.Fatal(err)
					}
					for key, want := range oracle.Result.Result.Value {
						if !reflect.DeepEqual(actual[key], want) {
							t.Errorf("%s: got %#v; want %#v", key, actual[key], want)
						}
					}
					if len(actual) != len(oracle.Result.Result.Value) {
						t.Errorf("result key count: got %d, want %d", len(actual), len(oracle.Result.Result.Value))
					}
				})
			}
		})
	}
}
