package state

import _ "embed"

// DefaultWebGLCapabilities is a measured Chrome 152 Windows/D3D11 capability
// profile. It describes query results, not a native device or shader backend.
// Keep it immutable: independent Pages must not share writable capability maps.
//
//go:embed webgl_capabilities.json
var defaultWebGLCapabilities string

func (g Graphics) WebGLCapabilities() string {
	if g.WebGLCapabilitiesJSON != "" {
		return g.WebGLCapabilitiesJSON
	}
	return defaultWebGLCapabilities
}
