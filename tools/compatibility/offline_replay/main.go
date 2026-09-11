package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"time"

	chrome "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/browser"
	v8 "github.com/moreveal/mimic/internal/engine/v8"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dir := flag.String("capture", "", "private manual capture directory")
	out := flag.String("out", "", "private output directory")
	pattern := flag.String("dynamic-segment", "", "optional private route regexp with one captured dynamic ID; explicit response substitution")
	overrides := flag.String("body-overrides", "", "optional JSON map from fixture index to replacement body path, for separate instrumentation runs")
	deadline := flag.Duration("timeout", 60*time.Second, "bounded wall time")
	steps := flag.Int("steps", 1600, "maximum scheduler advances")
	sleep := flag.Duration("sleep", 20*time.Millisecond, "wall time between advances")
	advance := flag.Duration("advance", 50*time.Millisecond, "logical time per advance")
	maxRequests := flag.Int("max-requests", 256, "maximum replacement transport calls")
	flag.Parse()
	if *dir == "" || *out == "" || *deadline <= 0 || *deadline > 2*time.Minute || *steps < 1 || *steps > 10000 || *maxRequests < 1 || *maxRequests > 10000 || *sleep < 0 || *sleep > time.Second || *advance <= 0 || *advance > time.Second {
		return fmt.Errorf("provide -capture and -out with bounded timeout/steps/requests")
	}
	if err := os.MkdirAll(*out, 0700); err != nil {
		return err
	}
	c, err := readCapture(*dir)
	if err != nil {
		return err
	}
	if *overrides != "" {
		data, err := os.ReadFile(*overrides)
		if err != nil {
			return err
		}
		c.SourceHashes["body-overrides"] = digest(data)
		var files map[string]string
		if err = json.Unmarshal(data, &files); err != nil {
			return err
		}
		for key, file := range files {
			index, err := strconv.Atoi(key)
			if err != nil {
				return err
			}
			body, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			found := false
			for _, f := range c.Fixtures {
				if f.Index == index {
					f.Body = body
					found = true
				}
			}
			if !found {
				return fmt.Errorf("override index %d does not exist", index)
			}
			c.SourceHashes["override-"+key] = digest(body)
		}
	}
	var segment *regexp.Regexp
	if *pattern != "" {
		segment, err = regexp.Compile(*pattern)
		if err != nil {
			return err
		}
		if segment.NumSubexp() != 1 {
			return fmt.Errorf("dynamic-segment must have exactly one captured ID")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), *deadline)
	defer cancel()
	t := newReplay(c, cancel, segment, *maxRequests)
	b, err := browser.New(v8.Factory{}, chrome.New())
	if err != nil {
		return err
	}
	bc := b.NewContext()
	defer bc.Close()
	page, err := bc.NewPage()
	if err != nil {
		return err
	}
	// The only transport is this recorded-fixture source. Never instantiate or
	// fall back to http.DefaultTransport, a TLS client or live target networking.
	page.Loader().SetTransport(t)
	unsubscribe := page.Trace().Subscribe(t.observe)
	defer unsubscribe()
	navigation := page.Navigate(ctx, c.DocumentURL)
	var pump error
	completedSteps := 0
	for ; completedSteps < *steps && ctx.Err() == nil; completedSteps++ {
		if *sleep > 0 {
			select {
			case <-time.After(*sleep):
			case <-ctx.Done():
			}
		}
		if ctx.Err() != nil {
			break
		}
		if pump = page.AdvanceTime(ctx, *advance); pump != nil {
			break
		}
	}
	if err = writeJSON(filepath.Join(*out, "replay-trace.json"), page.Trace().Events()); err != nil {
		return err
	}
	summary := t.summary()
	summary["navigationError"] = fmt.Sprint(navigation)
	summary["pumpError"] = fmt.Sprint(pump)
	summary["contextError"] = fmt.Sprint(ctx.Err())
	summary["completedSteps"] = completedSteps
	summary["bounds"] = map[string]any{"timeoutMs": deadline.Milliseconds(), "steps": *steps, "sleepMs": sleep.Milliseconds(), "advanceMs": advance.Milliseconds(), "maxRequests": *maxRequests}
	if info, ok := debug.ReadBuildInfo(); ok {
		summary["buildInfo"] = info
	}
	if executable, e := os.Executable(); e == nil {
		if data, e := os.ReadFile(executable); e == nil {
			summary["binarySHA256"] = digest(data)
		}
	}
	if err = writeJSON(filepath.Join(*out, "replay-result.json"), summary); err != nil {
		return err
	}
	fmt.Printf("offline replay: cycles=%d, transport requests=%d, unused=%d, context=%v\n", t.cycle, t.requests, len(summary["unusedFixtures"].([]map[string]any)), ctx.Err())
	return nil
}
func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
