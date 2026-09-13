package state

import (
	"fmt"
	"strings"
)

type ScreenOrientation struct {
	Type  string
	Angle int
}

// CDP identity is explicit Page state. Legacy UA, navigator and Client Hint
// projections consume this same record; transport fingerprints stay unchanged.
type UserAgentOverride struct {
	UserAgent, Platform string
	Languages           []string
	Metadata            *UserAgentMetadata
}
type UserAgentMetadata struct {
	Brands, FullVersionList                                              []UserAgentBrand
	FormFactors                                                          []string
	FullVersion, Platform, PlatformVersion, Architecture, Model, Bitness string
	Mobile, WoW64                                                        bool
}

func (e Environment) AppVersion() string {
	ua := e.Navigator().UserAgent
	if _, suffix, ok := strings.Cut(ua, "/"); ok {
		return suffix
	}
	return ua
}
func (e Environment) UserAgentData() UserAgentMetadata {
	if o := e.UserAgentOverride; o != nil && o.UserAgent != "" {
		if o.Metadata != nil {
			m := *o.Metadata
			if m.FormFactors == nil {
				m.FormFactors = []string{"Desktop"}
			}
			return m
		}
		return UserAgentMetadata{Brands: []UserAgentBrand{}, FullVersionList: []UserAgentBrand{}}
	}
	arch, bitness := "x86", "64"
	if strings.Contains(e.Platform.Architecture, "arm") {
		arch = "arm"
	}
	if !strings.Contains(e.Platform.Architecture, "64") {
		bitness = "32"
	}
	return UserAgentMetadata{Brands: e.Product.UserAgentBrands, FullVersionList: e.Product.UserAgentBrands, FormFactors: []string{"Desktop"}, FullVersion: e.Product.FullVersion, Platform: "Windows", PlatformVersion: e.Platform.OSVersion, Architecture: arch, Bitness: bitness}
}
func overrideHintHeaders(m *UserAgentMetadata, accepted map[string]bool) map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	for key, value := range map[string]string{"Sec-CH-UA-Arch": m.Architecture, "Sec-CH-UA-Bitness": m.Bitness, "Sec-CH-UA-Full-Version": m.FullVersion, "Sec-CH-UA-Model": m.Model, "Sec-CH-UA-Platform-Version": m.PlatformVersion} {
		if accepted[strings.ToLower(key)] {
			out[key] = fmt.Sprintf("%q", value)
		}
	}
	if accepted["sec-ch-ua-full-version-list"] {
		parts := []string{}
		for _, b := range m.FullVersionList {
			v := b.FullVersion
			if v == "" {
				v = b.Version
			}
			parts = append(parts, fmt.Sprintf(`%q;v=%q`, b.Brand, v))
		}
		out["Sec-CH-UA-Full-Version-List"] = strings.Join(parts, ", ")
	}
	if accepted["sec-ch-ua-wow64"] {
		out["Sec-CH-UA-WoW64"] = "?0"
		if m.WoW64 {
			out["Sec-CH-UA-WoW64"] = "?1"
		}
	}
	if accepted["sec-ch-ua-form-factors"] {
		factors := m.FormFactors
		if factors == nil {
			factors = []string{"Desktop"}
		}
		parts := make([]string, 0, len(factors))
		for _, factor := range factors {
			parts = append(parts, fmt.Sprintf("%q", factor))
		}
		out["Sec-CH-UA-Form-Factors"] = strings.Join(parts, ", ")
	}
	return out
}
