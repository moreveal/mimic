//go:build (windows || linux) && amd64

package browser

import (
	"encoding/json"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestRTPCapabilitiesMatchesFrozenChrome(t *testing.T) { documentAllOracle(t, "rtp") }
func TestRTPCatalogSDPMatchesFrozenChrome(t *testing.T) {
	raw, err := os.ReadFile("testdata/rtp_sdp_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string][]string
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	c := chrome152.New().Environment().State.Capabilities.Media.RTP
	for kind, lines := range expected {
		m := c.Media(kind)
		payloads := []string{}
		seen := map[int]bool{}
		for _, p := range m.Payloads {
			if p < 0 || p > 127 || seen[p] {
				t.Fatalf("invalid preferred payload assignment %s %d", kind, p)
			}
			seen[p] = true
			payloads = append(payloads, strconv.Itoa(p))
		}
		actual := []string{"m=" + kind + " 9 UDP/TLS/RTP/SAVPF " + strings.Join(payloads, " ")}
		actual = append(actual, m.Extensions...)
		actual = append(actual, m.Attributes...)
		if !reflect.DeepEqual(actual, lines) {
			t.Fatalf("%s SDP metadata differs from Chrome\ngot %v\nwant %v", kind, actual, lines)
		}
	}
}

func TestRTPDirectionSDPMatchesFrozenChrome(t *testing.T) {
	raw, err := os.ReadFile("testdata/rtp_direction_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string][]string
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	c := chrome152.New().Environment().State.Capabilities.Media.RTP
	for key, lines := range expected {
		parts := strings.Split(key, "|")
		direction := parts[1]
		if direction == "legacy" {
			direction = "recvonly"
		}
		m := c.Media(parts[0], direction)
		payloads := []string{}
		for _, p := range m.Payloads {
			payloads = append(payloads, strconv.Itoa(p))
		}
		actual := []string{"m=" + parts[0] + " 9 UDP/TLS/RTP/SAVPF " + strings.Join(payloads, " ")}
		actual = append(actual, m.Extensions...)
		actual = append(actual, m.Attributes...)
		if !reflect.DeepEqual(actual, lines) {
			t.Fatalf("%s direction projection differs", key)
		}
	}
}

func TestMediaSupportMatchesFrozenChrome(t *testing.T) { documentAllOracle(t, "media_support") }

func TestMediaCompletionMatchesFrozenChrome(t *testing.T) { documentAllOracle(t, "media_completion") }
