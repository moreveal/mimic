package cdp

import (
	"encoding/json"
	"fmt"

	"github.com/moreveal/mimic/internal/network"
)

func (s *session) handleResourcePolicy(m message) (any, bool, error) {
	switch m.Method {
	case "Mimic.getResourcePolicySchema", "Mimic.validateResourcePolicy", "Mimic.getResourcePolicy", "Mimic.updateResourcePolicy", "Mimic.getResourcePolicyStats":
	default:
		return nil, false, nil
	}
	var params map[string]json.RawMessage
	if len(m.Params) != 0 {
		if err := json.Unmarshal(m.Params, &params); err != nil {
			return nil, true, err
		}
	}
	if m.Method == "Mimic.getResourcePolicySchema" {
		return map[string]any{"schema": network.ResourcePolicySchema()}, true, nil
	}
	if m.Method == "Mimic.validateResourcePolicy" {
		policy, err := network.ParseResourcePolicy(params["policy"])
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"policy": policy}, true, nil
	}
	var contextID string
	if err := json.Unmarshal(params["browserContextId"], &contextID); err != nil || contextID == "" {
		return nil, true, fmt.Errorf("browserContextId is required")
	}
	context, ok := s.server.Browser.Context(contextID)
	if !ok {
		return nil, true, fmt.Errorf("browser context not found")
	}
	switch m.Method {
	case "Mimic.getResourcePolicy":
		policy, enabled := context.ResourcePolicy()
		return map[string]any{"policy": policy, "enabled": enabled, "generation": context.ResourcePolicyStats().Generation}, true, nil
	case "Mimic.getResourcePolicyStats":
		return map[string]any{"stats": context.ResourcePolicyStats()}, true, nil
	default:
		policy, err := network.ParseResourcePolicy(params["policy"])
		if err != nil {
			return nil, true, err
		}
		generation, err := context.UpdateResourcePolicy(policy)
		return map[string]any{"generation": generation}, true, err
	}
}
