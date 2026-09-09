package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNavigationRequestContextMatchesChrome(t *testing.T) {
	type observation struct {
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
	}
	var oracle struct {
		Requests         []observation   `json:"requests"`
		IframeReflection json.RawMessage `json:"iframeReflection"`
	}
	raw, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/navigation-request-context-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		received := make(chan observation, 16)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/favicon.ico" {
				headers := map[string]string{}
				for _, key := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User", "Referer"} {
					if value := r.Header.Get(key); value != "" {
						headers[strings.ToLower(key)] = value
					}
				}
				received <- observation{Path: r.URL.RequestURI(), Headers: headers}
			}
			w.Header().Set("Content-Type", "text/html")
			if strings.HasPrefix(r.URL.Path, "/policy") {
				w.Header().Set("Referrer-Policy", "same-origin")
			}
			w.Write([]byte("<!doctype html><body>fixture"))
		}))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL+"/start?source=1"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, "location.reload();true", true)
		historyEval(t, p, "location.href='/next';true", true)
		cross := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
		historyEval(t, p, `new Promise(resolve=>{const f=document.createElement('iframe');f.onload=()=>resolve(true);f.src=`+strconv.Quote(cross+"/frame")+`;document.body.appendChild(f)})`, true)
		historyEval(t, p, `new Promise(resolve=>{const f=document.createElement('iframe');f.onload=()=>resolve(true);f.referrerPolicy='no-referrer';f.src='/private-frame';document.body.appendChild(f)})`, true)
		historyEval(t, p, `(()=>{const f=document.createElement('iframe'),actual=['NO-REFERRER','bad',' origin ','unsafe-url'].map(v=>{f.referrerPolicy=v;return [f.getAttribute('referrerpolicy'),f.referrerPolicy]});return JSON.stringify(actual)===JSON.stringify(`+string(oracle.IframeReflection)+`)})()`, true)
		if err := p.Navigate(context.Background(), server.URL+"/policy?source=1"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `new Promise(resolve=>{const f=document.createElement('iframe');f.onload=()=>resolve(true);f.src=`+strconv.Quote(cross+"/policy-frame")+`;document.body.appendChild(f)})`, true)
		historyEval(t, p, `new Promise(resolve=>{const f=document.createElement('iframe');f.onload=()=>resolve(true);f.referrerPolicy='unsafe-url';f.src=`+strconv.Quote(cross+"/unsafe-frame")+`;document.body.appendChild(f)})`, true)
		for index, expected := range oracle.Requests {
			select {
			case actual := <-received:
				for k, v := range actual.Headers {
					actual.Headers[k] = strings.ReplaceAll(strings.ReplaceAll(v, server.URL, "ORIGIN"), cross, "CROSS_ORIGIN")
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("request%d got%#v want%#v", index, actual, expected)
				}
			default:
				t.Fatalf("missing request%d", index)
			}
		}
	})
}
