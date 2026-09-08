package cdp

import "sort"

type protocolCoverage struct {
	Name                 string `json:"name"`
	Kind                 string `json:"kind"`
	SurfaceRegistered    bool   `json:"surfaceRegistered"`
	SemanticsImplemented bool   `json:"semanticsImplemented"`
	SemanticsVerified    bool   `json:"semanticsVerified"`
}

var implementedProtocolMethods = stringSet(
	"Browser.close", "DOM.enable", "DOM.getDocument", "DOM.getOuterHTML",
	"Emulation.setDeviceMetricsOverride", "Emulation.setTouchEmulationEnabled",
	"Fetch.continueRequest", "Fetch.continueResponse", "Fetch.disable", "Fetch.enable", "Fetch.failRequest", "Fetch.fulfillRequest",
	"Log.enable", "Network.clearBrowserCache", "Network.clearBrowserCookies", "Network.continueInterceptedRequest", "Network.deleteCookies",
	"Network.emulateNetworkConditions", "Network.enable", "Network.getAllCookies", "Network.getCookies", "Network.getResponseBody",
	"Network.setCacheDisabled", "Network.setCookie", "Network.setCookies", "Network.setExtraHTTPHeaders", "Network.setRequestInterception",
	"Page.addScriptToEvaluateOnNewDocument", "Page.enable", "Page.getFrameTree", "Page.navigate", "Page.setBypassCSP", "Page.setLifecycleEventsEnabled",
	"Performance.enable", "Performance.getMetrics", "Runtime.callFunctionOn", "Runtime.enable", "Runtime.evaluate", "Runtime.releaseObject", "Runtime.releaseObjectGroup",
	"Security.enable", "Security.setIgnoreCertificateErrors", "Storage.enable", "Storage.getCookies",
	"Target.attachToTarget", "Target.getBrowserContexts", "Target.getTargets", "Target.sendMessageToTarget", "Target.setAutoAttach", "Target.setDiscoverTargets",
)

var verifiedProtocolMethods = stringSet(
	"DOM.enable", "DOM.getDocument", "DOM.getOuterHTML", "Emulation.setDeviceMetricsOverride",
	"Fetch.continueRequest", "Fetch.enable", "Network.clearBrowserCache", "Network.continueInterceptedRequest", "Network.enable",
	"Network.getAllCookies", "Network.setCacheDisabled", "Network.setCookie", "Network.setRequestInterception",
	"Page.enable", "Page.getFrameTree", "Page.navigate", "Page.setBypassCSP", "Page.setLifecycleEventsEnabled",
	"Runtime.callFunctionOn", "Runtime.enable", "Runtime.evaluate", "Target.attachToTarget", "Target.getBrowserContexts",
	"Target.getTargets", "Target.sendMessageToTarget", "Target.setDiscoverTargets",
)

func stringSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func protocolMatrix(methods, events map[string]struct{}) []protocolCoverage {
	out := make([]protocolCoverage, 0, len(methods)+len(events))
	for name := range methods {
		_, implemented := implementedProtocolMethods[name]
		_, verified := verifiedProtocolMethods[name]
		out = append(out, protocolCoverage{name, "method", true, implemented, verified})
	}
	for name := range events {
		out = append(out, protocolCoverage{name, "event", true, false, false})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
