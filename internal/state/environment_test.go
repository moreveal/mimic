package state

import (
	"strings"
	"testing"
	"time"
)

func TestDerivedStateIsCoherent(t *testing.T) {
	before := time.Now().Add(-time.Second)
	e := ChromeDesktopWindows(Product{Name: "Chrome", Version: "152.0.0.0", FullVersion: "152.0.7977.82"})
	after := time.Now().Add(time.Second)
	e.Display.DeviceScaleFactor = 2
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := e.Screen().Width; got != 960 {
		t.Fatalf("screen width = %d", got)
	}
	if got := e.RequestHeaders()["User-Agent"]; got != e.Navigator().UserAgent {
		t.Fatal("HTTP and JS identities diverged")
	}
	if got := e.RequestHeaders()["Sec-CH-UA"]; !strings.Contains(got, `v="152"`) {
		t.Fatalf("client hints do not derive from product version: %s", got)
	}
	if e.Time.WallOrigin.Before(before) || e.Time.WallOrigin.After(after) {
		t.Fatalf("wall clock must represent runtime state, got %v", e.Time.WallOrigin)
	}
	if e.Preferences.ColorScheme != "dark" || e.Preferences.ReducedMotion {
		t.Fatalf("unexpected desktop preferences: %#v", e.Preferences)
	}
}
