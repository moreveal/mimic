package browser

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Waveform tests use an explicit error budget for the portable FFT/libm DSP.
// API state/error values are compared exactly; existing exact PCM fixtures are
// unchanged. Every captured output sample participates in this comparison.
func TestOfflineAudioSynthesisOracles(t *testing.T) {
	for _, name := range []string{"audio_oscillator", "audio_compressor", "audio_synthesis_graph", "audio_oscillator_schedule", "audio_synthesis_relations", "audio_oscillator_extremes"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + name + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + strings.ReplaceAll(name, "_", "-") + "-chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Result map[string]any `json:"result"`
			}
			if err = json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			historyTestPages(t, func(t *testing.T, p *Page) {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				value, err := p.Evaluate(ctx, "(async()=>JSON.stringify(await "+string(source)+"))()")
				if err != nil {
					t.Fatal(err)
				}
				var actual map[string]any
				if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
					t.Fatal(err)
				}
				for key, want := range capture.Result {
					got := actual[key]
					numeric := name == "audio_oscillator_schedule" || strings.HasPrefix(key, "pcm") || strings.HasPrefix(key, "sine") || strings.HasPrefix(key, "square") || strings.HasPrefix(key, "sawtooth") || strings.HasPrefix(key, "triangle")
					if !numeric {
						if !reflect.DeepEqual(got, want) {
							t.Errorf("%s got %v want %v", key, got, want)
						}
						continue
					}
					tolerance := 1e-5
					maxError := 0.0
					var compare func(any, any)
					compare = func(a, b any) {
						switch b := b.(type) {
						case float64:
							a, ok := a.(float64)
							if !ok {
								t.Errorf("%s missing number", key)
								return
							}
							d := math.Abs(a - b)
							maxError = math.Max(maxError, d)
						case []any:
							a, ok := a.([]any)
							if !ok || len(a) != len(b) {
								t.Errorf("%s array shape differs", key)
								return
							}
							for i := range b {
								compare(a[i], b[i])
							}
						case map[string]any:
							a, ok := a.(map[string]any)
							if !ok {
								t.Errorf("%s object shape differs", key)
								return
							}
							for k, v := range b {
								compare(a[k], v)
							}
						default:
							if !reflect.DeepEqual(a, b) {
								t.Errorf("%s value differs", key)
							}
						}
					}
					compare(got, want)
					if maxError > tolerance {
						t.Errorf("%s max absolute error %.9g exceeds %.9g", key, maxError, tolerance)
					} else {
						t.Logf("%s max absolute error %.9g", key, maxError)
					}
				}
			})
		})
	}
}
