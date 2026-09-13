package state

import "testing"

func TestFormFactorAndWoW64Hints(t *testing.T) {
	e := ChromeDesktopWindows(Product{Name: "Chrome", Version: "152.0.0.0", FullVersion: "152.0.7977.82"})
	accepted := map[string]bool{"sec-ch-ua-form-factors": true, "sec-ch-ua-wow64": true}
	if got := e.ClientHintHeaders(nil); len(got) != 0 {
		t.Fatal(got)
	}
	if got := e.ClientHintHeaders(accepted); got["Sec-CH-UA-Form-Factors"] != `"Desktop"` || got["Sec-CH-UA-WoW64"] != "?0" {
		t.Fatal(got)
	}
	e.UserAgentOverride = &UserAgentOverride{UserAgent: "Test/1", Metadata: &UserAgentMetadata{WoW64: true}}
	for _, tc := range []struct {
		factors []string
		want    string
	}{{nil, `"Desktop"`}, {[]string{}, ""}, {[]string{"Tablet"}, `"Tablet"`}} {
		e.UserAgentOverride.Metadata.FormFactors = tc.factors
		got := e.ClientHintHeaders(accepted)
		if got["Sec-CH-UA-Form-Factors"] != tc.want || got["Sec-CH-UA-WoW64"] != "?1" {
			t.Fatal(got)
		}
		if len(e.UserAgentData().FormFactors) == 0 && tc.want != "" {
			t.Fatal("navigator projection missing factors")
		}
	}
	clone := e.Clone()
	clone.UserAgentOverride.Metadata.FormFactors[0] = "changed"
	if e.UserAgentData().FormFactors[0] != "Tablet" {
		t.Fatal("shared form-factor state")
	}
}
