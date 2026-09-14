package state

import "strings"

// NavigatorProjection is shared by Window and Worker bindings.
func (e Environment) NavigatorProjection(offline, webdriver bool) map[string]any {
	n := e.Navigator()
	metadata := e.UserAgentData()
	brands := make([]map[string]any, 0, len(e.Product.UserAgentBrands))
	for _, brand := range metadata.Brands {
		brands = append(brands, map[string]any{"brand": brand.Brand, "version": brand.Version, "fullVersion": brand.FullVersion})
	}
	values := map[string]any{"userAgent": n.UserAgent, "appVersion": strings.TrimPrefix(n.UserAgent, "Mozilla/"), "platform": n.Platform, "languages": n.Languages, "language": n.Languages[0], "hardwareConcurrency": n.HardwareConcurrency, "deviceMemory": n.DeviceMemory, "onLine": n.Online && !offline, "cookieEnabled": n.CookieEnabled, "vendor": "Google Inc.", "product": "Gecko", "appName": "Netscape", "maxTouchPoints": 0, "webdriver": webdriver, "pdfViewerEnabled": true, "uaBrands": brands, "uaFullVersion": e.Product.FullVersion, "architecture": "x86", "bitness": "64", "model": "", "platformVersion": e.Platform.OSVersion}
	values["appVersion"] = e.AppVersion()
	values["uaPlatform"] = metadata.Platform
	values["uaMobile"] = metadata.Mobile
	values["uaFullVersion"] = metadata.FullVersion
	values["platformVersion"] = metadata.PlatformVersion
	values["architecture"] = metadata.Architecture
	values["model"] = metadata.Model
	values["bitness"] = metadata.Bitness
	values["wow64"] = metadata.WoW64
	values["formFactors"] = append([]string{}, metadata.FormFactors...)
	fullBrands := []map[string]any{}
	for _, b := range metadata.FullVersionList {
		v := b.FullVersion
		if v == "" {
			v = b.Version
		}
		fullBrands = append(fullBrands, map[string]any{"brand": b.Brand, "version": v})
	}
	values["uaFullVersionList"] = fullBrands
	if e.Hardware.CPUPerformanceKnown {
		values["cpuPerformance"] = e.Hardware.CPUPerformance
	}
	return values
}
