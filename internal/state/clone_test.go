package state

import "testing"

func TestEnvironmentCloneOwnsNestedState(t *testing.T) {
	e := ChromeDesktopWindows(Product{Name: "Chrome", Version: "152", FullVersion: "152"})
	e.Capabilities.Media.RTP = RTPCatalog{"audio": {Codecs: map[string]RTPCodec{"opus": {Feedback: []string{"nack"}}}}}
	e.UserAgentOverride = &UserAgentOverride{Languages: []string{"en"}, Metadata: &UserAgentMetadata{Brands: []UserAgentBrand{{Brand: "Chrome"}}}}
	clone := e.Clone()
	clone.Locale.Languages[0] = "fr"
	clone.Capabilities.Media.RTP["audio"].Codecs["opus"].Feedback[0] = "changed"
	clone.UserAgentOverride.Metadata.Brands[0].Brand = "changed"
	if e.Locale.Languages[0] != "en-US" || e.Capabilities.Media.RTP["audio"].Codecs["opus"].Feedback[0] != "nack" || e.UserAgentOverride.Metadata.Brands[0].Brand != "Chrome" {
		t.Fatal("shared nested ownership")
	}
}
