package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestConsumedServicesChromeOracle(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		source, err := os.ReadFile("testdata/consumed_services_oracle.js")
		if err != nil {
			t.Fatal(err)
		}
		expected, err := os.ReadFile("testdata/consumed_services_chrome152.json")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := p.Evaluate(ctx, string(source))
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err := json.Unmarshal(expected, &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(fmt.Sprint(actual)), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s; want %s", actual, expected)
		}
	})
}

func TestNavigatorWebDriverInvariant(t *testing.T) {
	if navigatorWebDriver {
		t.Fatal("WebDriver must never be enabled by host configuration")
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(()=>{const f=document.createElement('iframe');document.body.append(f);const d=Object.getOwnPropertyDescriptor(Navigator.prototype,'webdriver');return navigator.webdriver===false&&f.contentWindow.navigator.webdriver===false&&d.get.call(navigator)===false})()`)
		if err != nil || value != true {
			t.Fatalf("webdriver invariant: %v, %v", value, err)
		}
	})
}

func TestCrashReportDiagnosticsOwnBuffer(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := p.Evaluate(ctx, `(async()=>{await crashReport.initialize(32);crashReport.set('phase','ready');try{crashReport.set('phase','x'.repeat(64))}catch(e){if(e.name!=='NotAllowedError')throw e}})()`)
		if err != nil {
			t.Fatal(err)
		}
		reports := p.CrashReports()
		if reports[p.Top.ID] != `{"phase":"ready"}` {
			t.Fatalf("diagnostic buffer: %v", reports)
		}
		reports[p.Top.ID] = "modified"
		if p.CrashReports()[p.Top.ID] != `{"phase":"ready"}` {
			t.Fatal("diagnostics exposed mutable storage")
		}
	})
}

func TestLaunchQueueRetainsAndDeliversURLLaunches(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := p.QueueLaunch("https://example.test/first"); err != nil {
			t.Fatal(err)
		}
		if err := p.QueueLaunch("https://example.test/second"); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, `new Promise(resolve=>{const out=[];launchQueue.setConsumer(p=>{out.push([p.targetURL,p.files.length,Object.isFrozen(p.files),p instanceof LaunchParams]);if(out.length===2)resolve(JSON.stringify(out))})})`)
		want := `[["https://example.test/first",0,true,true],["https://example.test/second",0,true,true]]`
		if err != nil || value != want {
			t.Fatalf("launch delivery: %v, %v", value, err)
		}
	})
}

func TestConsumedCacheChromeOracle(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}

		source, err := os.ReadFile("testdata/consumed_cache_oracle.js")
		if err != nil {
			t.Fatal(err)
		}
		expected, err := os.ReadFile("testdata/consumed_cache_chrome152.json")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := p.Evaluate(ctx, string(source))
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err := json.Unmarshal(expected, &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(fmt.Sprint(actual)), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s; want %s", actual, expected)
		}
	})
}

func TestConsumedCookieChromeOracle(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}

		source, err := os.ReadFile("testdata/consumed_cookie_oracle.js")
		if err != nil {
			t.Fatal(err)
		}
		expected, err := os.ReadFile("testdata/consumed_cookie_chrome152.json")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := p.Evaluate(ctx, string(source))
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err := json.Unmarshal(expected, &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(fmt.Sprint(actual)), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s; want %s", actual, expected)
		}
	})
}
