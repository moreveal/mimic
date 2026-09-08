package chrome

import "testing"

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
}
