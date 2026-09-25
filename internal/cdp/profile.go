package cdp

import (
	"encoding/json"

	"github.com/moreveal/mimic/internal/profile"
)

// Custom commands have their own strict envelope. They do not modify the frozen
// Chrome schema and never acquire the connection's incidental control Page.
func (s *session) handleProfile(m message) (any, bool, error) {
	if result, handled, err := s.handlePortableProfile(m); handled {
		return result, true, err
	}
	allowed := map[string]string{}
	switch m.Method {
	case "Mimic.getProfileSchema":
	case "Mimic.getProfile":
		allowed["browserContextId"] = "string"
		allowed["targetId"] = "string"
	case "Mimic.updateProfile":
		allowed["targetId"] = "string"
		allowed["patch"] = "object"
	case "Mimic.resetProfileOverrides":
		allowed["targetId"] = "string"
	default:
		return nil, false, nil
	}
	raw := m.Params
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	params, err := profile.Decode(raw)
	if err != nil {
		return nil, true, err
	}
	bad := func(path, why string) (any, bool, error) {
		return nil, true, &profile.Error{Path: path, Reason: "invalidParameter", Message: why}
	}
	for key, value := range params {
		typ, ok := allowed[key]
		if !ok {
			return bad(key, "unknown command parameter")
		}
		valid := false
		switch typ {
		case "object":
			_, valid = value.(map[string]any)
		case "string":
			v, ok := value.(string)
			valid = ok && v != ""
		case "boolean":
			_, valid = value.(bool)
		}
		if !valid {
			return bad(key, "expected "+typ)
		}
	}
	b := s.server.Browser
	if m.Method == "Mimic.getProfileSchema" {
		base := b.Environment()
		return map[string]any{"schema": profile.ManualSchema(base), "baseProfiles": []string{base.ProfileID}, "limitations": profile.Limitations()}, true, nil
	}
	target, _ := params["targetId"].(string)
	contextID, _ := params["browserContextId"].(string)
	if m.Method == "Mimic.getProfile" && contextID != "" {
		if target != "" {
			return bad("targetId", "choose targetId or browserContextId")
		}
		for _, c := range b.Contexts() {
			if c.ID == contextID {
				return map[string]any{"profile": c.Profile().Public(), "limitations": profile.Limitations()}, true, nil
			}
		}
		return bad("browserContextId", "context not found")
	}
	if target == "" {
		return bad("targetId", "required")
	}
	page, _, ok := s.server.target(target)
	if !ok {
		return bad("targetId", "target not found")
	}
	page.LockCommands()
	defer page.UnlockCommands()
	switch m.Method {
	case "Mimic.getProfile":
		return map[string]any{"profile": page.Profile().Public(), "limitations": profile.Limitations()}, true, nil
	case "Mimic.resetProfileOverrides":
		return map[string]any{"profile": page.ResetProfileOverrides().Public()}, true, nil
	default:
		value, ok := params["patch"]
		if !ok {
			return bad("patch", "required")
		}
		raw, _ := json.Marshal(value)
		d, err := page.UpdateProfile(raw)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"profile": d.Public()}, true, nil
	}
}
