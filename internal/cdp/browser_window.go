package cdp

import (
	"fmt"

	"github.com/moreveal/mimic/internal/browser"
)

const primaryWindowID = 1

func windowBounds(page *browser.Page) map[string]any {
	w := page.Environment().Window
	return map[string]any{
		"left":        w.X,
		"top":         w.Y,
		"width":       w.OuterWidth,
		"height":      w.OuterHeight,
		"windowState": "normal",
	}
}

func optionalInt(value any) *int {
	if value == nil {
		return nil
	}
	v := intValue(value, 0)
	return &v
}

func (s *session) handleBrowserWindow(method string, p map[string]any) (any, bool, error) {
	switch method {
	case "Browser.getWindowForTarget":
		page := s.page
		if targetID := stringValue(p["targetId"]); targetID != "" {
			var ok bool
			page, _, ok = s.server.target(targetID)
			if !ok {
				return nil, true, fmt.Errorf("No target with given id found")
			}
		}
		if page == nil {
			return nil, true, fmt.Errorf("No target with given id found")
		}
		return map[string]any{"windowId": primaryWindowID, "bounds": windowBounds(page)}, true, nil
	case "Browser.getWindowBounds":
		if intValue(p["windowId"], 0) != primaryWindowID {
			return nil, true, fmt.Errorf("Browser window not found")
		}
		page := s.page
		if page == nil {
			page = s.server.Page
		}
		return map[string]any{"bounds": windowBounds(page)}, true, nil
	case "Browser.setWindowBounds":
		if intValue(p["windowId"], 0) != primaryWindowID {
			return nil, true, fmt.Errorf("Browser window not found")
		}
		bounds, _ := p["bounds"].(map[string]any)
		if state := stringValue(bounds["windowState"]); state != "" && state != "normal" {
			return nil, true, fmt.Errorf("Window state %s is not supported", state)
		}
		for _, page := range s.server.pages() {
			if err := page.SetWindowBounds(optionalInt(bounds["left"]), optionalInt(bounds["top"]), optionalInt(bounds["width"]), optionalInt(bounds["height"])); err != nil {
				return nil, true, err
			}
		}
		return map[string]any{}, true, nil
	}
	return nil, false, nil
}
