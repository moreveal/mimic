package cdp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeolocationOverrideCommands(t *testing.T) {
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html>") }))
	defer fixture.Close()
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	wireCall(t, c, 2, "Browser.grantPermissions", map[string]any{"permissions": []string{"geolocation"}})
	flatCall(t, c, sid, 3, "Emulation.setGeolocationOverride", map[string]any{"latitude": 41, "longitude": 44, "accuracy": 5, "altitude": 10, "heading": 360, "speed": 0})
	if err := s.Page.Navigate(context.Background(), fixture.URL); err != nil {
		t.Fatal(err)
	}
	read := func() any {
		t.Helper()
		value, err := s.Page.Evaluate(context.Background(), `new Promise(resolve=>navigator.geolocation.getCurrentPosition(p=>resolve(p.coords.latitude===41&&p.coords.longitude===44&&p.coords.accuracy===5&&p.coords.altitude===10&&p.coords.heading===360&&p.coords.speed===0),e=>resolve(e.code)))`)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if got := read(); got != true {
		t.Fatalf("override: %#v", got)
	}
	flatCall(t, c, sid, 4, "Emulation.clearGeolocationOverride", map[string]any{})
	if got := read(); got != float64(2) {
		t.Fatalf("clear: %#v", got)
	}
	flatCall(t, c, sid, 5, "Page.setGeolocationOverride", map[string]any{"latitude": 41, "longitude": 44, "accuracy": 5})
	flatCall(t, c, sid, 6, "Page.clearGeolocationOverride", map[string]any{})
	wireCall(t, c, 7, "Browser.setPermission", map[string]any{"permission": map[string]any{"name": "geolocation"}, "setting": "denied", "origin": fixture.URL})
	if got := read(); got != float64(1) {
		t.Fatalf("permission denied: %#v", got)
	}
	wireCall(t, c, 8, "Browser.resetPermissions", map[string]any{})
	value, err := s.Page.Evaluate(context.Background(), `navigator.permissions.query({name:'geolocation'}).then(p=>p.state==='prompt')`)
	if err != nil || value != true {
		t.Fatalf("permission reset: %#v, %v", value, err)
	}
}
