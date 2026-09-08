package chrome

import (
	"testing"

	"github.com/moreveal/mimic/internal/state"
)

func TestGetReturnsExactVersionedBundle(t *testing.T) {
	bundle, err := Get(152)
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.Version(); got.Milestone != 152 || got.Version != "152.0.7977.82" || got.ChromiumRevision != 1669021 {
		t.Fatalf("unexpected bundle pin: %+v", got)
	}
	if _, err := Get(153); err == nil {
		t.Fatal("uninstalled milestone must not silently fall back")
	}
	if bundle.Environment().Mode != state.BrowserModeHeadful {
		t.Fatal("registry default must select the authoritative headful oracle")
	}
	headless, err := GetForMode(152, state.BrowserModeHeadless)
	if err != nil || headless.Environment().Mode != state.BrowserModeHeadless {
		t.Fatalf("explicit headless selection failed: %v", err)
	}
}
