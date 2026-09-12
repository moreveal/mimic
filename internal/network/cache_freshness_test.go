package network

import (
	"net/http"
	"testing"
	"time"
)

// The hit/miss boundary was measured in Chrome 152.0.7977.83 with two
// navigations requesting the same script (freshness cases in the report).
func TestCacheFreshnessLifetimeAndAge(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, control, expires, age string
		date, modified              time.Duration
		want                        time.Duration
	}{
		{name: "heuristic", modified: -100 * 24 * time.Hour, want: 10 * 24 * time.Hour},
		{name: "zero is explicit", control: "max-age=0", modified: -100 * 24 * time.Hour},
		{name: "no-cache", control: "no-cache", modified: -100 * 24 * time.Hour},
		{name: "no-store", control: "no-store", modified: -100 * 24 * time.Hour},
		{name: "aged", control: "max-age=60", age: "120"},
		{name: "fresh", control: "max-age=60", want: time.Minute},
		{name: "apparent age", control: "max-age=60", date: -30 * time.Second, want: 30 * time.Second},
		{name: "intermediary age", control: "max-age=60", age: "40", date: -30 * time.Second, want: 20 * time.Second},
		{name: "invalid lifetime", control: "max-age=bad", modified: -100 * 24 * time.Hour},
		{name: "invalid expiry", expires: "0", modified: -100 * 24 * time.Hour},
		{name: "future modification", modified: 24 * time.Hour},
		{name: "no freshness metadata"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			h.Set("Date", now.Add(tc.date).Format(http.TimeFormat))
			if tc.modified != 0 {
				h.Set("Last-Modified", now.Add(tc.modified).Format(http.TimeFormat))
			}
			h.Set("Cache-Control", tc.control)
			h.Set("Age", tc.age)
			h.Set("Expires", tc.expires)
			if got := cacheFreshnessRemaining(h, now); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
