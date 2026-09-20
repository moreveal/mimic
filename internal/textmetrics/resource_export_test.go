package textmetrics

import "testing"

func TestFontResourceBytesOwnership(t *testing.T) {
	e := New()
	defer e.Close()
	id, err := e.LocalFont("Courier New")
	if err != nil || id == "" {
		t.Skip("Courier New unavailable")
	}
	first, index, err := e.FontResourceBytes(id)
	if err != nil || len(first) == 0 || index < 0 {
		t.Fatalf("export: %d %d %v", len(first), index, err)
	}
	original := first[0]
	first[0] ^= 255
	second, _, err := e.FontResourceBytes(id)
	if err != nil || second[0] != original {
		t.Fatal("caller mutated canonical font bytes")
	}
	other := New()
	defer other.Close()
	if _, _, err := other.FontResourceBytes(id); err == nil {
		t.Fatal("font resource leaked between Page engines")
	}
}
