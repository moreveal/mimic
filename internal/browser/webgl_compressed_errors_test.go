//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestWebGLCompressedErrorsFrozenChrome(t *testing.T) {
	source, err := os.ReadFile("testdata/webgl_compressed_errors_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/webgl_compressed_errors_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Observation any `json:"observation"`
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	for _, restored := range []bool{false, true} {
		t.Run(strconv.FormatBool(restored), func(t *testing.T) {
			t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", map[bool]string{false: "1", true: "0"}[restored])
			seed := bootstrapSnapshotPage(t)
			navigateCapabilityFixture(t, seed)
			if restored {
				bootstrapSnapshotWarm(t, seed)
			}
			p, err := seed.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			navigateCapabilityFixture(t, p)
			value, err := p.Evaluate(context.Background(), string(source))
			if err != nil {
				t.Fatal(err)
			}
			if d := bootstrapSnapshotDifference("WebGL compressed_errors", capture.Observation, value); d != "" {
				t.Fatal(d)
			}
		})
	}
}
