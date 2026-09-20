package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Diagnostic wall accounting only. Nested native resolve phases overlap the
// nativeResolve envelope; never sum both into producer time.
type blitzAccounting struct {
	started, last time.Time
	phases        map[string]float64
}

func newBlitzAccounting() *blitzAccounting {
	if os.Getenv("MIMIC_PROFILE_BLITZ") != "1" {
		return nil
	}
	now := time.Now()
	return &blitzAccounting{started: now, last: now, phases: make(map[string]float64)}
}
func (a *blitzAccounting) mark(name string) {
	if a == nil {
		return
	}
	now := time.Now()
	a.phases[name] += float64(now.Sub(a.last).Nanoseconds()) / 1e6
	a.last = now
}
func (a *blitzAccounting) finish(realm string, state *blitzDocument, builds, generation uint64, err error) {
	if a == nil {
		return
	}
	a.mark("finalize")
	outcome := "reuse"
	if state.document.Builds != builds {
		outcome = "first-build"
		if builds > 0 {
			outcome = "owner-rebuild"
		}
	} else if state.generation != generation {
		outcome = "rebuild"
	}
	if state.fallback != "" {
		outcome = "fallback"
	}
	message := ""
	if err != nil {
		outcome = "error"
		message = err.Error()
	}
	record := map[string]any{"realm": realm, "key": state.key, "outcome": outcome, "builds": state.document.Builds, "generation": state.generation, "previousGeneration": generation, "phasesWallMS": a.phases, "totalWallMS": float64(time.Since(a.started).Nanoseconds()) / 1e6, "error": message, "fallback": state.fallback}
	data, _ := json.Marshal(record)
	fmt.Fprintf(os.Stderr, "BLITZ accounting %s\n", data)
}
