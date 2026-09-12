package csp

import "strings"

// TrustedTypesState is a projection of the document's enforced policies. The
// factory and its callbacks belong to the realm, not to this immutable policy.
type TrustedTypesState struct {
	Enforced    bool       `json:"enforced"`
	Required    bool       `json:"required"`
	Rules       [][]string `json:"rules"`
	EvalBlocked string     `json:"evalBlocked"`
}

func (s TrustedTypesState) Projection() map[string]any {
	rules := make([]any, len(s.Rules))
	for i, names := range s.Rules {
		rules[i] = names
	}
	return map[string]any{"required": s.Required, "enforced": s.Enforced, "rules": rules, "evalBlocked": s.EvalBlocked}
}

func (set PolicySet) TrustedTypes() TrustedTypesState {
	state := TrustedTypesState{Rules: [][]string{}}
	for _, policy := range set {
		if contains(policy.directives["require-trusted-types-for"], "'script'") {
			state.Required = true
			if !policy.reportOnly {
				state.Enforced = true
			}
		}
		if policy.reportOnly {
			continue
		}
		if names, ok := policy.directives["trusted-types"]; ok {
			state.Rules = append(state.Rules, append([]string{}, names...))
		}
		sources, ok := policy.directives["script-src"]
		directive := "script-src"
		if !ok {
			sources, ok = policy.directives["default-src"]
			directive = "default-src"
		}
		if ok && !contains(sources, "'unsafe-eval'") && state.EvalBlocked == "" {
			state.EvalBlocked = directive + " " + strings.Join(sources, " ")
		}
	}
	return state
}
