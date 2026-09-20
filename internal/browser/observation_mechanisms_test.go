package browser

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestObservationMechanismOracles(t *testing.T) {
	serialBrowserTest(t)
	for _, name := range []string{"input_enumerations", "system_colors", "system_fonts", "css_supports", "css_contract", "css_display", "document_focus", "focus_frames", "window_focus", "navigator_realms", "gpu_info", "gpu_contract", "gpu_realms", "realtime_audio", "offline_audio_lifecycle", "audio_analyser", "audio_latency", "audio_decode", "audio_worklet", "audio_automation", "audio_node_surface", "audio_media_nodes", "audio_graph_topology", "audio_filters", "audio_filter_pcm", "canvas_blends", "canvas_system_colors"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + name + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("testdata/" + name + "_chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Result   map[string]any `json:"result"`
				Metadata struct {
					SourceSHA256  string `json:"sourceSHA256"`
					ChromeVersion string `json:"chromeVersion"`
					BrowserMode   string `json:"browserMode"`
				} `json:"captureMetadata"`
			}
			if err := json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			// Captures hash UTF-8 source with normalized newlines, as Python's
			// text reader does on the Windows oracle host.
			hash := sha256.Sum256([]byte(strings.ReplaceAll(string(source), "\r\n", "\n")))
			if capture.Metadata.SourceSHA256 != fmt.Sprintf("%x", hash) || capture.Metadata.ChromeVersion != "152.0.7977.82" || capture.Metadata.BrowserMode != "headful" {
				t.Fatal("oracle provenance does not match source and reference mode")
			}
			historyTestPages(t, func(t *testing.T, p *Page) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if name == "navigator_realms" || name == "gpu_info" || name == "gpu_contract" || name == "gpu_realms" {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "text/html")
						_, _ = w.Write([]byte("<!doctype html><body>"))
					}))
					defer server.Close()
					if err := p.Navigate(ctx, server.URL); err != nil {
						t.Fatal(err)
					}
				}
				if name == "realtime_audio" {
					if _, err := p.Evaluate(ctx, "0"); err != nil {
						t.Fatal(err)
					}
					if err := p.DispatchInput(ctx, 0, "mousedown"); err != nil {
						t.Fatal(err)
					}
				}
				expression := strings.TrimSuffix(strings.TrimSpace(string(source)), ";")
				v, err := p.Evaluate(ctx, "(async()=>JSON.stringify(await "+expression+"))()")
				if err != nil {
					t.Fatal(err)
				}
				var actual map[string]any
				if err := json.Unmarshal([]byte(v.(string)), &actual); err != nil {
					t.Fatal(err)
				}
				for key, want := range capture.Result {
					if !reflect.DeepEqual(actual[key], want) {
						t.Errorf("%s", observationDifference(key, actual[key], want))
					}
				}
			})
		})
	}
}

func observationDifference(path string, actual, expected any) string {
	if a, ok := actual.([]any); ok {
		if b, ok := expected.([]any); ok && len(a) == len(b) {
			for i := range a {
				if !reflect.DeepEqual(a[i], b[i]) {
					return observationDifference(fmt.Sprintf("%s[%d]", path, i), a[i], b[i])
				}
			}
		}
	}
	return fmt.Sprintf("%s: got %v want %v", path, actual, expected)
}
