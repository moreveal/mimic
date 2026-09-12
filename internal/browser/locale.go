package browser

import (
	"math"
	"time"
	_ "time/tzdata" // Profiles must not depend on an OS zoneinfo installation.

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/state"
)

// The location belongs to this realm's immutable context environment. Never
// change time.Local or ICU's process-global defaults: Pages run concurrently.
func installLocaleHost(host map[string]any, runtime engine.Runtime, locale state.Locale) {
	location, err := time.LoadLocation(locale.Timezone)
	if err != nil {
		panic(err) // Environment validation precedes realm construction.
	}
	host["intlEnvironment"] = runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		name := locale.IntlLocale
		if name == "" && len(locale.Languages) > 0 {
			name = locale.Languages[0]
		}
		if name == "" {
			name = "en-US"
		}
		return runtime.Value(map[string]any{"locale": name, "timeZone": locale.Timezone}), nil
	})
	host["dateZone"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		ms := numarg(args, 0)
		if math.IsNaN(ms) || math.IsInf(ms, 0) || math.Abs(ms) > 8.64e15+2592000000 {
			return runtime.Value(nil), nil
		}
		instant := time.UnixMilli(int64(ms)).In(location)
		_, offset := instant.Zone()
		start, end := instant.ZoneBounds()
		lo, hi := float64(-8.64e15-2592000001), float64(8.64e15+2592000001)
		if !start.IsZero() {
			lo = float64(start.UnixMilli())
		}
		if !end.IsZero() {
			hi = float64(end.UnixMilli())
		}
		return runtime.Value([]any{offset * 1000, lo, hi}), nil
	})
}
