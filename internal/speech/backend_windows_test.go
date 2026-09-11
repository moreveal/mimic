//go:build windows && cgo

package speech

import (
	"os"
	"testing"
	"time"
)

func TestWindowsProviderNativeNotifications(t *testing.T) {
	if os.Getenv("MIMIC_TEST_SYSTEM_SPEECH") != "1" {
		t.Skip("opt-in installed Windows voice provider test")
	}
	events := make(chan Event, 64)
	backend := Open(func(e Event) { events <- e })
	defer backend.Close()
	select {
	case e := <-events:
		if e.Kind != "voices" || len(e.Voices) == 0 {
			t.Fatalf("native discovery: %+v", e)
		}
		for _, v := range e.Voices {
			if v.Name == "" || v.ID == "" || v.Lang == "" {
				t.Fatalf("incomplete native voice: %+v", v)
			}
		}
		t.Logf("installed voices: %+v", e.Voices)
	case <-time.After(15 * time.Second):
		t.Fatal("voice discovery timed out")
	}
	// Native synthesis with zero volume exercises the provider's real completion
	// notification without producing audible output or assuming a duration.
	backend.Speak(Utterance{ID: 1, Text: "mimic", Volume: 0, Rate: 1, Pitch: 1})
	started := false
	deadline := time.After(15 * time.Second)
	for {
		select {
		case e := <-events:
			t.Logf("native event: %+v", e)
			if e.Kind == "error" {
				t.Fatalf("native synthesis failed: %+v", e)
			}
			if e.Kind == "start" {
				started = true
			}
			if e.Kind == "end" {
				if !started || e.ID != 1 || e.CharIndex != 5 {
					t.Fatalf("native event order: %+v", e)
				}
				return
			}
		case <-deadline:
			t.Fatal("native completion timed out")
		}
	}
}

func TestWindowsProviderAllocationFailureStaysResponsive(t *testing.T) {
	events := make(chan Event, 8)
	// No native handle models CreateEvent/allocation failure before COM setup.
	b := &windowsBackend{done: make(chan struct{}), wake: make(chan struct{}, 1), notify: func(e Event) { events <- e }}
	go b.run()
	defer b.Close()
	next := func() Event {
		t.Helper()
		select {
		case e := <-events:
			return e
		case <-time.After(3 * time.Second):
			t.Fatal("failed provider stopped responding")
			return Event{}
		}
	}
	if e := next(); e.Kind != "voices" || len(e.Voices) != 0 {
		t.Fatalf("discovery: %+v", e)
	}
	// Submit after the initial discovery, rather than racing the worker start.
	for _, id := range []uint64{1, 2} {
		b.Speak(Utterance{ID: id, Text: "unavailable", Rate: 1, Pitch: 1})
		if e := next(); e.Kind != "error" || e.ID != id || e.Error != "synthesis-unavailable" {
			t.Fatalf("failure delivery: %+v", e)
		}
		b.Cancel()
	}
	joined := make(chan struct{})
	go func() { b.Close(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(3 * time.Second):
		t.Fatal("failed provider close did not join")
	}
	b.Speak(Utterance{ID: 3})
}
