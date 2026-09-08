package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moreveal/mimic/chrome"
	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/cdp"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/state"
)

func TestStartupPhaseProfile(t *testing.T) {
	dir := os.Getenv("MIMIC_PROFILE_DIR")
	if dir == "" {
		t.Skip("diagnostic startup replay")
	}
	var rows []map[string]any
	for i := 0; i < 5; i++ {
		row := map[string]any{"iteration": i}
		start := time.Now()
		bundle, err := chrome.GetForMode(152, state.BrowserMode("headless"))
		if err != nil {
			t.Fatal(err)
		}
		row["bundle_ms"] = float64(time.Since(start)) / 1e6
		start = time.Now()
		b, err := browser.New(v8engine.Factory{}, bundle)
		if err != nil {
			t.Fatal(err)
		}
		row["browser_ms"] = float64(time.Since(start)) / 1e6
		start = time.Now()
		s, err := cdp.New(b)
		if err != nil {
			t.Fatal(err)
		}
		row["cdp_and_initial_page_ms"] = float64(time.Since(start)) / 1e6
		start = time.Now()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		row["listen_ms"] = float64(time.Since(start)) / 1e6
		listener.Close()
		s.Close(context.Background())
		rows = append(rows, row)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "startup.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
