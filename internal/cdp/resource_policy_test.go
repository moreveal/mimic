package cdp

import "testing"

func TestMimicResourcePolicyCommands(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	schema := wireCall(t, c, 1, "Mimic.getResourcePolicySchema", map[string]any{})
	if schema["schema"] == nil {
		t.Fatalf("schema: %v", schema)
	}
	context := s.Browser.NewContext()
	policy := map[string]any{"schemaVersion": 1, "rules": []any{map[string]any{"id": "images", "match": map[string]any{"kinds": []string{"image"}}, "work": map[string]any{"network": false}}}}
	validated := wireCall(t, c, 2, "Mimic.validateResourcePolicy", map[string]any{"policy": policy})
	if validated["policy"] == nil {
		t.Fatal(validated)
	}
	updated := wireCall(t, c, 3, "Mimic.updateResourcePolicy", map[string]any{"browserContextId": context.ID, "policy": policy})
	if updated["generation"] != float64(1) {
		t.Fatal(updated)
	}
	got := wireCall(t, c, 4, "Mimic.getResourcePolicy", map[string]any{"browserContextId": context.ID})
	if got["enabled"] != true {
		t.Fatal(got)
	}
	stats := wireCall(t, c, 5, "Mimic.getResourcePolicyStats", map[string]any{"browserContextId": context.ID})
	if stats["stats"] == nil {
		t.Fatal(stats)
	}
	created := wireCall(t, c, 6, "Mimic.createContext", map[string]any{"resourcePolicy": policy})
	if created["browserContextId"] == nil {
		t.Fatal(created)
	}
}
