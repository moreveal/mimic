package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/network"
)

func TestCertificateOverrideUsesBrowserAndPageSessionScope(t *testing.T) {
	fixture := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "TLS") }))
	fixture.Config.ErrorLog = log.New(io.Discard, "", 0)
	fixture.StartTLS()
	defer fixture.Close()
	u, _ := url.Parse(fixture.URL)
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	// This test performs several real TLS verification failures between CDP
	// reads. The pinned transport may race protocols before returning them.
	_ = c.SetReadDeadline(time.Now().Add(45 * time.Second))
	pageSession := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	otherID := wireCall(t, c, 2, "Target.createTarget", map[string]any{"url": "about:blank"})["targetId"].(string)
	other, _ := s.page(otherID)
	check := func(page *browser.Page, success bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := page.Loader().Load(ctx, network.Request{URL: u, Initiator: network.Navigation})
		if (err == nil) != success {
			t.Fatalf("Page %s success=%v error=%v", page.ID, success, err)
		}
	}
	flatCall(t, c, pageSession, 3, "Security.setIgnoreCertificateErrors", map[string]any{"ignore": true})
	check(s.Page, true)
	check(other, false)
	wireCall(t, c, 4, "Security.setIgnoreCertificateErrors", map[string]any{"ignore": true})
	check(other, true)
	futureID := wireCall(t, c, 5, "Target.createTarget", map[string]any{"url": "about:blank"})["targetId"].(string)
	future, _ := s.page(futureID)
	check(future, true)
	wireCall(t, c, 6, "Security.setIgnoreCertificateErrors", map[string]any{"ignore": false})
	check(s.Page, true)
	check(other, false)
	check(future, false)
	wireCall(t, c, 7, "Target.detachFromTarget", map[string]any{"sessionId": pageSession})
	check(s.Page, false)
}

func TestStorageCookiesUseRequestedBrowserContextAndNetworkURLFilter(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	contextID := wireCall(t, c, 1, "Target.createBrowserContext", nil)["browserContextId"].(string)
	wireCall(t, c, 2, "Storage.setCookies", map[string]any{"browserContextId": contextID, "cookies": []any{map[string]any{"name": "isolated", "value": "one", "url": "https://cookie.test/path"}}})
	if got := wireCall(t, c, 3, "Storage.getCookies", nil)["cookies"].([]any); len(got) != 0 {
		t.Fatal("context cookie leaked", got)
	}
	if got := wireCall(t, c, 4, "Storage.getCookies", map[string]any{"browserContextId": contextID})["cookies"].([]any); len(got) != 1 {
		t.Fatal(got)
	}
	wireCall(t, c, 5, "Storage.clearCookies", map[string]any{"browserContextId": contextID})
	if got := wireCall(t, c, 6, "Storage.getCookies", map[string]any{"browserContextId": contextID})["cookies"].([]any); len(got) != 0 {
		t.Fatal(got)
	}
	sid := wireCall(t, c, 7, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	flatCall(t, c, sid, 8, "Network.setCookies", map[string]any{"cookies": []any{
		map[string]any{"name": "scoped", "value": "a", "url": "https://cookie.test/", "path": "/wanted"},
		map[string]any{"name": "elsewhere", "value": "b", "url": "https://other.test/"},
	}})
	got := flatCall(t, c, sid, 9, "Network.getCookies", map[string]any{"urls": []any{"https://cookie.test/wanted/child", "https://cookie.test/wanted"}})["cookies"].([]any)
	if len(got) != 1 || got[0].(map[string]any)["name"] != "scoped" {
		t.Fatal(got)
	}
	if got := flatCall(t, c, sid, 10, "Network.getCookies", map[string]any{"urls": []any{"https://cookie.test/wanted-no"}})["cookies"].([]any); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestDeviceAndUserAgentOverridesShareObservationsAndRemainPageLocal(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	a := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	other := wireCall(t, c, 2, "Target.createTarget", map[string]any{"url": "about:blank"})["targetId"].(string)
	b := wireCall(t, c, 3, "Target.attachToTarget", map[string]any{"targetId": other, "flatten": true})["sessionId"].(string)
	flatCall(t, c, a, 4, "Emulation.setDeviceMetricsOverride", map[string]any{"width": 1600, "height": 900, "deviceScaleFactor": 2, "mobile": false, "screenOrientation": map[string]any{"type": "portraitPrimary", "angle": 90}})
	flatCall(t, c, a, 5, "Network.setUserAgentOverride", map[string]any{"userAgent": "Automation/1.0", "platform": "TestPlatform", "acceptLanguage": "fr-FR,en"})
	read := func(sid string, id int) map[string]any {
		return flatCall(t, c, sid, id, "Runtime.evaluate", map[string]any{"expression": "({width:innerWidth,height:innerHeight,dpr:devicePixelRatio,type:screen.orientation.type,angle:screen.orientation.angle,ua:navigator.userAgent,version:navigator.appVersion,platform:navigator.platform,languages:navigator.languages,brands:navigator.userAgentData?.brands??[]})", "returnByValue": true})["result"].(map[string]any)["value"].(map[string]any)
	}
	got := read(a, 6)
	if got["width"] != float64(1600) || got["height"] != float64(900) || got["dpr"] != float64(2) || got["type"] != "portrait-primary" || got["angle"] != float64(90) || got["ua"] != "Automation/1.0" || got["version"] != "1.0" || got["platform"] != "TestPlatform" || len(got["brands"].([]any)) != 0 {
		t.Fatal(got)
	}
	if isolated := read(b, 7); isolated["ua"] == got["ua"] || isolated["width"] == got["width"] || isolated["type"] == got["type"] {
		t.Fatal("emulation leaked", isolated)
	}
	flatCall(t, c, a, 8, "Emulation.clearDeviceMetricsOverride", nil)
	if got := read(a, 9); got["width"] != float64(s.Browser.Environment().Window.ViewportWidth) || got["type"] != "landscape-primary" {
		t.Fatal(got)
	}
}

func TestDetachReleasesFetchPauseAndPreservesPageNavigation(t *testing.T) {
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<title>Detached load</title><body>ready</body>")
	}))
	defer fixture.Close()
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	flatCall(t, c, sid, 2, "Runtime.enable", nil) // Attach debugger roots too; teardown must not deadlock.
	flatCall(t, c, sid, 3, "Fetch.enable", map[string]any{"patterns": []any{map[string]any{"urlPattern": "*", "resourceType": "Document"}}})
	_ = c.WriteJSON(map[string]any{"id": 4, "sessionId": sid, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL}})
	paused := false
	for !paused {
		var e map[string]any
		if err := c.ReadJSON(&e); err != nil {
			t.Fatal(err)
		}
		paused = e["method"] == "Fetch.requestPaused"
	}
	wireCall(t, c, 5, "Target.detachFromTarget", map[string]any{"sessionId": sid})
	next := wireCall(t, c, 6, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	// Reattaching releases no navigation/load barrier. Chrome 152 continues
	// the detached Page's request, then commits and loads it independently.
	// The old synchronous Page lock accidentally made an immediate title read
	// wait for that work. Observe explicit readiness through the new session.
	deadline := time.Now().Add(5 * time.Second)
	_ = c.SetReadDeadline(deadline)
	var got map[string]any
	for id := 7; time.Now().Before(deadline); id++ {
		got = flatCall(t, c, next, id, "Runtime.evaluate", map[string]any{
			"expression": "({url:location.href,title:document.title,ready:document.readyState,body:document.body?.textContent})", "returnByValue": true,
		})["result"].(map[string]any)["value"].(map[string]any)
		if (got["url"] == fixture.URL || got["url"] == fixture.URL+"/") && got["ready"] == "complete" {
			if got["title"] != "Detached load" || got["body"] != "ready" {
				t.Fatal(got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("detached Page navigation did not complete: %#v", got)
}

func TestNetworkIdleWaitsForQuietWindowAfterLoad(t *testing.T) {
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<title>Idle</title>") }))
	defer fixture.Close()
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	flatCall(t, c, sid, 2, "Page.enable", nil)
	flatCall(t, c, sid, 3, "Page.setLifecycleEventsEnabled", map[string]any{"enabled": true})
	_ = c.WriteJSON(map[string]any{"id": 4, "sessionId": sid, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL}})
	var loaded time.Time
	for {
		var event map[string]any
		if err := c.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event["method"] == "Page.lifecycleEvent" {
			p := event["params"].(map[string]any)
			if p["name"] == "load" {
				loaded = time.Now()
			}
			if p["name"] == "networkIdle" && !loaded.IsZero() {
				if elapsed := time.Since(loaded); elapsed < 450*time.Millisecond {
					raw, _ := json.Marshal(event)
					t.Fatalf("idle too early: %s (%s)", elapsed, raw)
				}
				return
			}
		}
		if _, ok := event["error"]; ok {
			t.Fatal(event)
		}
	}
}
