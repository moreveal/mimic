package cdp

import "testing"

func TestDownloadBehaviorValidatesPolicyAndContext(t *testing.T) {
	s, _ := runningServer(t)
	if err := s.setDownloadBehavior(map[string]any{"behavior": "allowAndName"}); err == nil {
		t.Fatal("allowAndName accepted without a destination")
	}
	if err := s.setDownloadBehavior(map[string]any{"behavior": "allow", "downloadPath": "relative"}); err == nil {
		t.Fatal("relative download destination accepted")
	}
	if err := s.setDownloadBehavior(map[string]any{"behavior": "allow", "downloadPath": t.TempDir(), "browserContextId": "missing"}); err == nil {
		t.Fatal("unknown context accepted")
	}
	if err := s.setDownloadBehavior(map[string]any{"behavior": "allowAndName", "downloadPath": t.TempDir(), "eventsEnabled": true}); err != nil {
		t.Fatal(err)
	}
	if got := s.downloadPolicies[""]; got.behavior != "allowAndName" || !got.eventsEnabled {
		t.Fatalf("unexpected policy: %+v", got)
	}
}
