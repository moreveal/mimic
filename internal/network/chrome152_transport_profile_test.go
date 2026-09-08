package network

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/bogdanfinn/tls-client/profiles"
)

func TestChrome152TransportProfileIncludesPinnedHTTP3Fingerprint(t *testing.T) {
	for _, profile := range []profiles.ClientProfile{profiles.Chrome_152, profiles.Chrome_152_PSK} {
		if got, want := profile.GetHttp3Settings(), map[uint64]uint64{1: 65536, 7: 100}; !reflect.DeepEqual(got, want) {
			t.Fatalf("HTTP/3 settings = %#v, want %#v", got, want)
		}
		if got, want := profile.GetHttp3SettingsOrder(), []uint64{1, 0x6, 7, 0x33}; !reflect.DeepEqual(got, want) {
			t.Fatalf("HTTP/3 settings order = %#v, want %#v", got, want)
		}
		if got := profile.GetHttp3PriorityParam(); got != 984832 {
			t.Fatalf("HTTP/3 priority parameter = %d, want 984832", got)
		}
		if got, want := profile.GetHttp3PseudoHeaderOrder(), []string{":method", ":authority", ":scheme", ":path"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("HTTP/3 pseudo-header order = %#v, want %#v", got, want)
		}
		if !profile.GetHttp3SendGreaseFrames() {
			t.Fatal("Chrome 152 must send HTTP/3 GREASE frames")
		}
	}
}

func TestChrome152BrowserManagedHeaderOrder(t *testing.T) {
	headers := make(http.Header)
	for name, value := range map[string]string{
		"Sec-CH-UA": "ua", "Sec-CH-UA-Mobile": "?0", "Sec-CH-UA-Platform": "Windows",
		"Accept-Language": "ru-RU", "Upgrade-Insecure-Requests": "1", "User-Agent": "Chrome",
		"Accept": "text/html", "Sec-Fetch-Site": "none", "Sec-Fetch-Mode": "navigate",
		"Sec-Fetch-User": "?1", "Sec-Fetch-Dest": "document", "Accept-Encoding": "gzip", "Priority": "u=0, i",
	} {
		headers.Set(name, value)
	}
	want := []string{"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "accept-language", "upgrade-insecure-requests", "user-agent", "accept", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest", "accept-encoding", "priority"}
	if got := chromeHeaderOrder(headers, nil, Navigation); !reflect.DeepEqual(got, want) {
		t.Fatalf("header order = %#v, want %#v", got, want)
	}
}

func TestChrome152XHRHeaderOrderUsesPinnedBlinkHashMap(t *testing.T) {
	headers := make(http.Header)
	for name, value := range map[string]string{
		"Content-Length": "4", "X-Mimic-First": "1", "X-Mimic-Second": "2", "X-Mimic-Third": "3",
		"Sec-CH-UA-Platform": "Windows", "Accept-Language": "en-US", "Sec-CH-UA": "ua", "Sec-CH-UA-Mobile": "?0",
		"User-Agent": "Chrome", "Content-Type": "text/plain", "Accept": "*/*", "Origin": "https://example.test",
		"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty",
		"Referer": "https://example.test/", "Accept-Encoding": "gzip", "Priority": "u=1, i",
	} {
		headers.Set(name, value)
	}
	want := []string{"content-length", "x-mimic-third", "sec-ch-ua-platform", "accept-language", "sec-ch-ua", "sec-ch-ua-mobile", "x-mimic-second", "x-mimic-first", "user-agent", "content-type", "accept", "origin", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "referer", "accept-encoding", "priority"}
	got := chromeHeaderOrder(headers, []string{"x-mimic-first", "x-mimic-second", "x-mimic-third"}, XHR)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("header order = %#v, want %#v", got, want)
	}
}

func TestBlinkHeaderHashOracle(t *testing.T) {
	names := []string{"content-length", "x-mimic-third", "sec-ch-ua-platform", "accept-language", "sec-ch-ua", "sec-ch-ua-mobile", "x-mimic-second", "x-mimic-first", "user-agent", "content-type", "accept", "origin", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "referer", "accept-encoding", "priority", "cf-chl-ra", "sec-ch-ua-bitness", "sec-ch-ua-model", "sec-ch-ua-arch", "sec-ch-ua-full-version", "cf-chl", "sec-ch-ua-platform-version"}
	for _, name := range names {
		t.Logf("%02d %08x %s", blinkHeaderHash(name)&31, blinkHeaderHash(name), name)
	}
}
