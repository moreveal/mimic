package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

func TestRedirectMethodAndBody(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, method := range []string{"POST", "PUT", "HEAD"} {
			t.Run(fmt.Sprintf("%d/%s", status, method), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/start" {
						w.Header().Set("Location", "/end")
						w.WriteHeader(status)
						return
					}
					body, _ := io.ReadAll(r.Body)
					w.Header().Set("X-Observed", r.Method+"|"+string(body)+"|"+r.Header.Get("Content-Type"))
				}))
				defer server.Close()
				u, _ := url.Parse(server.URL + "/start")
				loader := NewLoader(testEnvironment, NewCookieStore(), trace.New())
				res, err := loader.Load(context.Background(), Request{URL: u, Method: method, Body: []byte("payload"), Headers: http.Header{"Content-Type": {"text/plain"}}, Initiator: Fetch})
				if err != nil {
					t.Fatal(err)
				}
				want := method + "|payload|text/plain"
				if (status == 301 || status == 302) && method == "POST" || status == 303 && method != "HEAD" {
					want = "GET||"
				}
				if got := res.Headers.Get("X-Observed"); got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
				if !res.Redirected || res.URL.Path != "/end" {
					t.Fatalf("redirect metadata: %+v", res)
				}
			})
		}
	}
}

func TestRedirectLimitModesAndNonRedirectStatus(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Location", "/loop")
		status := 302
		if r.URL.Path == "/missing" {
			w.Header().Del("Location")
		}
		if r.URL.Path == "/empty" {
			w.Header().Set("Location", "")
		}
		if r.URL.Path == "/credentials" {
			w.Header().Set("Location", "http://user:password@"+r.Host+"/target")
		}
		if r.URL.Path == "/304" {
			status = 304
		}
		if strings.HasPrefix(r.URL.Path, "/count/") {
			n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/count/"))
			if n == 0 {
				w.Header().Del("Location")
				status = 200
			} else {
				w.Header().Set("Location", fmt.Sprintf("/count/%d", n-1))
			}
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	for _, tc := range []struct {
		path, mode    string
		count, status int
		fail          bool
	}{
		{"/loop", "follow", 21, 0, true},
		{"/count/20", "follow", 21, 200, false},
		{"/loop", "error", 1, 0, true},
		{"/loop", "manual", 1, 0, false},
		{"/304", "follow", 1, 304, false},
		{"/missing", "follow", 1, 302, false},
		{"/missing", "error", 1, 302, false},
		{"/missing", "manual", 1, 0, false},
		{"/empty", "error", 1, 302, false},
		{"/empty", "manual", 1, 0, false},
		{"/credentials", "follow", 1, 0, true},
		{"/credentials", "manual", 1, 0, false},
	} {
		requests = 0
		u, _ := url.Parse(server.URL + tc.path)
		res, err := NewLoader(testEnvironment, NewCookieStore(), trace.New()).Load(context.Background(), Request{URL: u, Redirect: tc.mode, Initiator: Fetch})
		if (err != nil) != tc.fail || requests != tc.count || err == nil && res.Status != tc.status {
			t.Fatalf("%+v: requests=%d response=%+v err=%v", tc, requests, res, err)
		}
		if tc.mode == "manual" && (res.Type != "opaqueredirect" || len(res.Headers) != 0 || len(res.Body) != 0 || res.Redirected) {
			t.Fatalf("manual response exposed redirect: %+v", res)
		}
	}
}

func TestRedirectRecomputesCredentialsAndPreservesFragment(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Expose-Headers", "X-Authorization, X-Cookie, X-Site")
		w.Header().Set("X-Authorization", r.Header.Get("Authorization"))
		w.Header().Set("X-Cookie", r.Header.Get("Cookie"))
		w.Header().Set("X-Site", r.Header.Get("Sec-Fetch-Site"))
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", strings.Replace(target.URL, "127.0.0.1", "localhost", 1)+"/end")
		w.WriteHeader(301)
	}))
	defer source.Close()
	u, _ := url.Parse(source.URL + "/start#kept")
	cookies := NewCookieStore()
	cookies.SetFromResponse(u, http.Header{"Set-Cookie": {"secret=source; Path=/"}})
	res, err := NewLoader(testEnvironment, cookies, trace.New()).Load(context.Background(), Request{URL: u, SourceURL: u, Initiator: Fetch, Headers: http.Header{"Authorization": {"Bearer secret"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Headers.Get("X-Authorization") != "" || res.Headers.Get("X-Cookie") != "" || res.Headers.Get("X-Site") != "cross-site" || res.URL.Fragment != "kept" {
		t.Fatalf("stale redirect state: %+v", res)
	}
}
