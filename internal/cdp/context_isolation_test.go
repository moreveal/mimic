package cdp

import "testing"

func TestGeneratedContextsIsolateTasksWithoutChangingDefaultCDP(t *testing.T) {
	s, addr := runningServer(t)
	connection := browserConnection(t, addr)
	defaultTarget := wireCall(t, connection, 1, "Target.createTarget", map[string]any{"url": "about:blank"})["targetId"].(string)
	defaultPage, ok := s.Context.Page(defaultTarget)
	if !ok || defaultPage.ContextID() != s.Page.ContextID() || defaultPage.Environment().ProfileID != s.Page.Environment().ProfileID {
		t.Fatal("Target.createTarget without browserContextId changed default CDP semantics")
	}

	first := wireCall(t, connection, 2, "Mimic.createContext", map[string]any{})
	second := wireCall(t, connection, 3, "Mimic.createContext", map[string]any{})
	firstID := first["browserContextId"].(string)
	secondID := second["browserContextId"].(string)
	defer wireCall(t, connection, 20, "Target.disposeBrowserContext", map[string]any{"browserContextId": firstID})
	defer wireCall(t, connection, 21, "Target.disposeBrowserContext", map[string]any{"browserContextId": secondID})
	if firstID == secondID || first["profileId"] == second["profileId"] {
		t.Fatal("generated contexts must identify separate tasks")
	}
	firstTarget := wireCall(t, connection, 4, "Target.createTarget", map[string]any{"browserContextId": firstID, "url": "about:blank"})["targetId"].(string)
	siblingTarget := wireCall(t, connection, 5, "Target.createTarget", map[string]any{"browserContextId": firstID, "url": "about:blank"})["targetId"].(string)
	secondTarget := wireCall(t, connection, 6, "Target.createTarget", map[string]any{"browserContextId": secondID, "url": "about:blank"})["targetId"].(string)
	a, _ := s.Browser.Context(firstID)
	b, _ := s.Browser.Context(secondID)
	firstPage, _ := a.Page(firstTarget)
	siblingPage, _ := a.Page(siblingTarget)
	secondPage, _ := b.Page(secondTarget)
	if firstPage.Environment().ProfileID != siblingPage.Environment().ProfileID || firstPage.Environment().ProfileID == secondPage.Environment().ProfileID {
		t.Fatal("profile must belong to Context, not Page")
	}
	wireCall(t, connection, 7, "Storage.setCookies", map[string]any{"browserContextId": firstID, "cookies": []any{map[string]any{"name": "account", "value": "one", "url": "https://isolation.test/"}}})
	if got := wireCall(t, connection, 8, "Storage.getCookies", map[string]any{"browserContextId": secondID})["cookies"].([]any); len(got) != 0 {
		t.Fatal("cookie crossed task contexts", got)
	}
	if got := wireCall(t, connection, 9, "Storage.getCookies", map[string]any{"browserContextId": firstID})["cookies"].([]any); len(got) != 1 {
		t.Fatal("cookie missing from its task context", got)
	}
}
