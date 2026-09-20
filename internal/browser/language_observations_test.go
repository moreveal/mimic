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
	"time"
)

func TestLanguageObservationsMatchChrome(t *testing.T) {
	parallelBrowserTest(t)
	for _, name := range []string{"realms", "network"} {
		t.Run(name, func(t *testing.T) { requestObservationOracle(t, "language", name) })
	}
}
func TestStorageAccessObservationsMatchChrome(t *testing.T) {
	parallelBrowserTest(t)
	for _, name := range []string{"access", "fetch"} {
		t.Run(name, func(t *testing.T) { requestObservationOracle(t, "storage", name) })
	}
}
func TestFetchCredentialObservationsMatchChrome(t *testing.T) {
	parallelBrowserTest(t)
	for _, worker := range []bool{false, true} {
		t.Run(strconv.FormatBool(worker), func(t *testing.T) { requestObservationOracle(t, "fetch", "credentials", worker) })
	}
}
func requestObservationOracle(t *testing.T, prefix, name string, worker ...bool) {
	source, err := os.ReadFile("testdata/" + prefix + "_" + name + "_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + prefix + "-" + name + "-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Result map[string]any `json:"result"`
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := map[string]string{}
		if prefix == "fetch" {
			if cookie := r.Header.Get("Cookie"); cookie != "" {
				out["cookie"] = cookie
			}
			if strings.HasPrefix(r.URL.Path, "/set") {
				http.SetCookie(w, &http.Cookie{Name: r.URL.Query().Get("name"), Value: "1", Path: "/"})
			}
			if r.URL.Path == "/redirect" {
				http.SetCookie(w, &http.Cookie{Name: "redirected", Value: "1", Path: "/"})
				w.Header().Set("Location", "/echo")
				w.WriteHeader(302)
				return
			}
		}
		for _, key := range []string{"accept-language", "sec-fetch-storage-access", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest"} {
			if value := r.Header.Get(key); value != "" {
				out[key] = value
			}
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if strings.HasPrefix(r.URL.Path, "/echo") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(out)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if strings.HasPrefix(r.URL.Path, "/frame") {
			encoded, _ := json.Marshal(out)
			w.Write([]byte(`<!doctype html><script>Promise.all([fetch("/echo").then(r=>r.json()),document.hasStorageAccess()]).then(([fetch,access])=>parent.postMessage({navigation:` + string(encoded) + `,fetch,access},"*"))</script>`))
			return
		}
		w.Write([]byte("<!doctype html><title>request oracle</title>"))
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		expression := "(async()=>JSON.stringify(await " + string(source) + "))()"
		if len(worker) > 0 && worker[0] {
			code := "onmessage=async()=>{try{postMessage(await " + expression + ")}catch(e){postMessage(String(e))}}"
			expression = `new Promise((resolve,reject)=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(Error(e.message));w.postMessage(null)})`
		}
		value, err := p.Evaluate(ctx, expression)
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]any
		if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, capture.Result) {
			t.Fatalf("got %v want %v", actual, capture.Result)
		}
	})
}
