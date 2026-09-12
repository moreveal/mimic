package main

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPDateTranslationPreservesCacheLifetimeAndAge(t *testing.T) {
	date := time.Date(2026, 9, 12, 0, 51, 28, 0, time.UTC)
	h := http.Header{"Date": {date.Format(http.TimeFormat)}, "Expires": {date.Add(time.Hour).Format(http.TimeFormat)}, "Last-Modified": {date.Add(-2 * time.Hour).Format(http.TimeFormat)}, "Age": {"37"}, "Cache-Control": {"max-age=14400, must-revalidate"}, "Etag": {"unchanged"}}
	rebaseHTTPDates(h, 7*time.Hour)
	for name, offset := range map[string]time.Duration{"Date": 0, "Expires": time.Hour, "Last-Modified": -2 * time.Hour} {
		got, err := http.ParseTime(h.Get(name))
		if err != nil || !got.Equal(date.Add(7*time.Hour+offset)) {
			t.Fatalf("%s: %s, %v", name, h.Get(name), err)
		}
	}
	if h.Get("Age") != "37" || h.Get("Cache-Control") != "max-age=14400, must-revalidate" || h.Get("Etag") != "unchanged" {
		t.Fatal("relative cache policy changed")
	}
	h = http.Header{"Expires": {"0"}, "Last-Modified": {"invalid"}}
	rebaseHTTPDates(h, time.Hour)
	if h.Get("Date") != "" || h.Get("Expires") != "0" || h.Get("Last-Modified") != "invalid" {
		t.Fatal("absent/invalid dates must remain unchanged")
	}
}
