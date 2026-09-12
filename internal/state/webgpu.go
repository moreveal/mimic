package state

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
