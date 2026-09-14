package state

import "testing"

func TestWebGPUAdapterSelectionKeepsProfilesCoherent(t *testing.T) {
	graphics := Graphics{
		MaxTextureSize: 8192,
		WebGPU:         GPUAdapter{Device: "default", Features: []string{"a"}},
		WebGPUAdapters: map[string]GPUAdapter{
			"low-power":        {Device: "integrated", Features: []string{"low"}, Limits: map[string]uint64{"maxTextureDimension2D": 4096}},
			"high-performance": {Device: "discrete", Features: []string{"high"}, Limits: map[string]uint64{"maxTextureDimension2D": 16384}},
			"fallback":         {Device: "software", IsFallbackAdapter: true},
		},
	}
	for preference, device := range map[string]string{"": "default", "low-power": "integrated", "high-performance": "discrete"} {
		projection, ok := graphics.SelectWebGPUAdapter(preference, false)
		if !ok || projection["device"] != device {
			t.Fatalf("%q selected %#v, %v", preference, projection, ok)
		}
	}
	fallback, ok := graphics.SelectWebGPUAdapter("", true)
	if !ok || fallback["device"] != "software" || fallback["isFallbackAdapter"] != true {
		t.Fatalf("fallback selected %#v, %v", fallback, ok)
	}
	low, _ := graphics.SelectWebGPUAdapter("low-power", false)
	limits := low["limits"].(map[string]uint64)
	if limits["maxTextureDimension2D"] != 4096 || low["features"].([]string)[0] != "low" {
		t.Fatalf("adapter projection mixed profiles: %#v", low)
	}
	delete(graphics.WebGPUAdapters, "fallback")
	if projection, ok := graphics.SelectWebGPUAdapter("", true); ok || projection != nil {
		t.Fatalf("missing fallback must be null: %#v, %v", projection, ok)
	}
}
