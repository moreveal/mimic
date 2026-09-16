package cdp

import "testing"

func TestPageSetFontFamiliesUpdatesGenericDefaults(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	reply := flatCall(t, c, sid, 2, "Page.setFontFamilies", map[string]any{
		"fontFamilies": map[string]any{
			"standard":  "Georgia",
			"fixed":     "Consolas",
			"sansSerif": "Arial",
		},
		"forScripts": []any{map[string]any{
			"script":       "cyrl",
			"fontFamilies": map[string]any{"serif": "Times New Roman"},
		}},
	})
	if reply["error"] != nil {
		t.Fatalf("setFontFamilies failed: %#v", reply["error"])
	}
	fonts := s.Page.Environment().Fonts
	if fonts.Serif != "Georgia" || fonts.SansSerif != "Arial" || fonts.Monospace != "Consolas" {
		t.Fatalf("font defaults = %#v", fonts)
	}
}
