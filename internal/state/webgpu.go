package state

// WebGPUInfo is the single adapter identity projection for Window and Worker.
func (g Graphics) WebGPUInfo() map[string]any {
	return g.WebGPUAdapterProjection(g.WebGPU)
}

// WebGPUAdapterProjection returns one coherent adapter view. Adapter-specific
// limits must travel with the adapter selected by requestAdapter rather than
// being read later from the environment's default adapter.
func (g Graphics) WebGPUAdapterProjection(a GPUAdapter) map[string]any {
	limits := WebGPUDefaultLimits()
	if g.MaxTextureSize > 0 {
		limits["maxTextureDimension2D"] = uint64(g.MaxTextureSize)
	}
	for name, value := range a.Limits {
		limits[name] = value
	}
	return map[string]any{"vendor": a.Vendor, "architecture": a.Architecture, "device": a.Device, "description": a.Description, "subgroupMinSize": a.SubgroupMinSize, "subgroupMaxSize": a.SubgroupMaxSize, "isFallbackAdapter": a.IsFallbackAdapter, "features": a.Features, "limits": limits, "maxTextureSize": g.MaxTextureSize}
}

// SelectWebGPUAdapter applies the request policy without mutating the shared
// environment profile. Every request receives a fresh JS adapter wrapper.
func (g Graphics) SelectWebGPUAdapter(preference string, forceFallback bool) (map[string]any, bool) {
	key := preference
	if forceFallback {
		key = "fallback"
	}
	adapter, ok := g.WebGPUAdapters[key]
	if !ok {
		if forceFallback {
			return nil, false
		}
		adapter = g.WebGPU
	}
	return g.WebGPUAdapterProjection(adapter), true
}

// WebGPUDefaultLimits are the browser's baseline device contract, distinct
// from the adapter limits selected by an Environment. Return fresh state.
func WebGPUDefaultLimits() map[string]uint64 {
	return map[string]uint64{
		"maxTextureDimension1D": 8192, "maxTextureDimension2D": 8192, "maxTextureDimension3D": 2048, "maxTextureArrayLayers": 256,
		"maxBindGroups": 4, "maxBindGroupsPlusVertexBuffers": 24, "maxBindingsPerBindGroup": 1000, "maxDynamicUniformBuffersPerPipelineLayout": 8, "maxDynamicStorageBuffersPerPipelineLayout": 4,
		"maxSampledTexturesPerShaderStage": 16, "maxSamplersPerShaderStage": 16, "maxStorageBuffersPerShaderStage": 8, "maxStorageTexturesPerShaderStage": 4, "maxUniformBuffersPerShaderStage": 12,
		"maxUniformBufferBindingSize": 65536, "maxStorageBufferBindingSize": 134217728, "minUniformBufferOffsetAlignment": 256, "minStorageBufferOffsetAlignment": 256,
		"maxVertexBuffers": 8, "maxBufferSize": 268435456, "maxVertexAttributes": 16, "maxVertexBufferArrayStride": 2048, "maxInterStageShaderVariables": 16,
		"maxColorAttachments": 8, "maxColorAttachmentBytesPerSample": 32, "maxComputeWorkgroupStorageSize": 16384, "maxComputeInvocationsPerWorkgroup": 256,
		"maxComputeWorkgroupSizeX": 256, "maxComputeWorkgroupSizeY": 256, "maxComputeWorkgroupSizeZ": 64, "maxComputeWorkgroupsPerDimension": 65535,
		"maxImmediateSize": 64, "maxStorageBuffersInFragmentStage": 8, "maxStorageBuffersInVertexStage": 8, "maxStorageTexturesInFragmentStage": 4, "maxStorageTexturesInVertexStage": 4,
	}
}

func (g Graphics) WebGPUProjection() map[string]any {
	limits := WebGPUDefaultLimits()
	if g.MaxTextureSize > 0 {
		limits["maxTextureDimension2D"] = uint64(g.MaxTextureSize)
	}
	for name, value := range g.WebGPU.Limits {
		limits[name] = value
	}
	return map[string]any{"defaults": WebGPUDefaultLimits(), "limits": limits, "wgsl": g.WebGPU.WGSLLanguageFeatures}
}
