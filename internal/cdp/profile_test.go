package cdp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMimicProfileContextsAndAtomicUpdates(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	schema := wireCall(t, c, 1, "Mimic.getProfileSchema", map[string]any{})
	if schema["schema"] == nil {
		t.Fatal(schema)
	}
	base := s.Browser.Environment().ProfileID
	create := func(id int, cpu, width int) string {
		// Ordinary contexts still support mutable CDP emulation. Provision their
		// environment through the internal test boundary, not the portable API.
		raw, _ := json.Marshal(map[string]any{"schemaVersion": 1, "baseProfile": base, "hardware": map[string]any{"logicalProcessors": cpu}, "window": map[string]any{"viewportWidth": width}, "locale": map[string]any{"languages": []string{"fr-FR", "en"}}})
		context, err := s.Browser.NewContextWithProfile(raw)
		if err != nil {
			t.Fatal(err)
		}
		return context.ID
	}
	ca, cb := create(2, 4, 900), create(3, 12, 1000)
	target := func(id int, context string) string {
		return wireCall(t, c, id, "Target.createTarget", map[string]any{"url": "about:blank", "browserContextId": context})["targetId"].(string)
	}
	a, b := target(4, ca), target(5, cb)
	attach := func(id int, target string) string {
		return wireCall(t, c, id, "Target.attachToTarget", map[string]any{"targetId": target, "flatten": true})["sessionId"].(string)
	}
	sa, sb := attach(6, a), attach(7, b)
	read := func(id int, sid string) map[string]any {
		return flatCall(t, c, sid, id, "Runtime.evaluate", map[string]any{"expression": "({width:innerWidth,cpu:navigator.hardwareConcurrency,language:navigator.language,dpr:devicePixelRatio})", "returnByValue": true})["result"].(map[string]any)["value"].(map[string]any)
	}
	if got := read(8, sa); got["cpu"] != float64(4) || got["width"] != float64(900) || got["language"] != "fr-FR" {
		t.Fatal(got)
	}
	if got := read(9, sb); got["cpu"] != float64(12) || got["width"] != float64(1000) {
		t.Fatal(got)
	}
	wireCall(t, c, 10, "Mimic.updateProfile", map[string]any{"targetId": a, "patch": map[string]any{"window": map[string]any{"viewportWidth": 800}, "display": map[string]any{"deviceScaleFactor": 2}}})
	if got := read(11, sa); got["width"] != float64(800) || got["dpr"] != float64(2) {
		t.Fatal(got)
	}
	_ = c.WriteJSON(map[string]any{"id": 12, "method": "Mimic.updateProfile", "params": map[string]any{"targetId": a, "patch": map[string]any{"window": map[string]any{"viewportWidth": 700}, "hardware": map[string]any{"logicalProcessors": 2}}}})
	reply := readReply(t, c, 12)
	if e, ok := reply["error"].(map[string]any); !ok || e["data"].(map[string]any)["reason"] != "requiresNewContext" {
		t.Fatal(reply)
	}
	if got := read(13, sa); got["width"] != float64(800) {
		t.Fatal("partial update", got)
	}
	wireCall(t, c, 14, "Mimic.resetProfileOverrides", map[string]any{"targetId": a})
	if got := read(15, sa); got["width"] != float64(900) || got["dpr"] != float64(1) {
		t.Fatal(got)
	}
	flatCall(t, c, sa, 16, "Emulation.setUserAgentOverride", map[string]any{"userAgent": "ExistingCDP/1", "acceptLanguage": "de-DE,en"})
	got := wireCall(t, c, 17, "Mimic.getProfile", map[string]any{"targetId": a})["profile"].(map[string]any)
	if got["identity"].(map[string]any)["userAgent"] != "ExistingCDP/1" || got["locale"].(map[string]any)["languages"].([]any)[0] != "de-DE" {
		t.Fatal(got)
	}
	wireCall(t, c, 18, "Target.disposeBrowserContext", map[string]any{"browserContextId": ca})
	wireCall(t, c, 19, "Target.disposeBrowserContext", map[string]any{"browserContextId": cb})
}

func TestMimicProfileProxyHeadersAndRedaction(t *testing.T) {
	seen := make(chan *http.Request, 16)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			t.Error("expected CONNECT")
			w.WriteHeader(400)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		seen <- req
		body, mime := "<title>proxied</title>", "text/html"
		switch req.URL.Path {
		case "/worker.js":
			mime = "text/javascript"
			body = `onmessage=async()=>{try{const r=await fetch('/redirect');postMessage(await r.text())}catch(e){postMessage(String(e))}}`
		case "/redirect":
			fmt.Fprint(conn, "HTTP/1.1 302 Found\r\nLocation: /echo\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
			return
		case "/echo":
			mime = "text/plain"
			body = req.Header.Get("Accept-Language")
		}
		fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nContent-Type: %s\r\nConnection: close\r\n\r\n%s", len(body), mime, body)
	}))
	defer proxy.Close()
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	cfg := map[string]any{"locale": map[string]any{"languages": []string{"fr-FR", "en"}}}
	result := wireCall(t, c, 1, "Mimic.importProfile", map[string]any{"mode": "manual", "profile": cfg})
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "secret-") {
		t.Fatal("exposed credentials")
	}
	ctx := wireCall(t, c, 2, "Mimic.createContext", map[string]any{"profile": result["profile"], "proxy": map[string]any{"server": proxy.URL, "username": "secret-user", "password": "secret-password"}})["browserContextId"].(string)
	target := wireCall(t, c, 3, "Target.createTarget", map[string]any{"browserContextId": ctx, "url": "about:blank"})["targetId"].(string)
	sid := wireCall(t, c, 4, "Target.attachToTarget", map[string]any{"targetId": target, "flatten": true})["sessionId"].(string)
	// A .invalid hostname proves the clear-text fallback cannot bypass the proxy.
	page, _ := s.page(target)
	page.LockCommands()
	err := page.Navigate(context.Background(), "http://profile.invalid/")
	page.UnlockCommands()
	if err != nil {
		t.Fatal(err)
	}
	value := flatCall(t, c, sid, 5, "Runtime.evaluate", map[string]any{"expression": `new Promise((resolve,reject)=>{const w=new Worker('/worker.js');w.onmessage=e=>{w.terminate();resolve(e.data)};w.onerror=reject;w.postMessage(1)})`, "awaitPromise": true, "returnByValue": true})
	if value["exceptionDetails"] != nil || value["result"].(map[string]any)["value"] != "fr-FR,en;q=0.9" {
		t.Fatal(value)
	}
	select {
	case r := <-seen:
		if !strings.HasPrefix(r.Header.Get("Accept-Language"), "fr-FR") {
			t.Fatal(r.Header)
		}
	default:
		t.Fatal("proxy not used")
	}
	result = wireCall(t, c, 6, "Mimic.getProfile", map[string]any{"targetId": target})
	raw, _ = json.Marshal(result)
	if strings.Contains(string(raw), "secret-") {
		t.Fatal("exposed credentials")
	}
	wireCall(t, c, 7, "Target.disposeBrowserContext", map[string]any{"browserContextId": ctx})
}
