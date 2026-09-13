package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFormFactorHintsRespectOptInAndRedirects(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		echo := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode([]string{r.Header.Get("Sec-CH-UA-Form-Factors"), r.Header.Get("Sec-CH-UA-WoW64")})
		}
		other := httptest.NewServer(http.HandlerFunc(echo))
		defer other.Close()
		top := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/probe":
				echo(w, r)
			case "/redirect":
				http.Redirect(w, r, other.URL, http.StatusFound)
			default:
				w.Header().Set("Accept-CH", "Sec-CH-UA-Form-Factors, Sec-CH-UA-WoW64")
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<!doctype html><body>")
			}
		}))
		defer top.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, top.URL); err != nil {
			t.Fatal(err)
		}
		got, err := p.Evaluate(ctx, `(async()=>{const a=await(await fetch('/probe')).json();const b=await(await fetch('/redirect')).json();const c=await navigator.userAgentData.getHighEntropyValues(['formFactors','wow64']);c.formFactors[0]='changed';const d=await navigator.userAgentData.getHighEntropyValues(['formFactors','wow64']);return JSON.stringify([a,b,d.formFactors,d.wow64])})()`)
		if err != nil || got != `[["\"Desktop\"","?0"],["",""],["Desktop"],false]` {
			t.Fatalf("%v, %v", got, err)
		}
	})
}

// Frozen Chrome 152: parent opt-in plus effective delegation is required.
// A child's Accept-CH alone cannot opt its subrequests in.
func TestClientHintsFramePolicy(t *testing.T) {
	for _, mode := range []string{"default", "delegate", "allow", "delegate-allow", "deny-allow", "noaccept-allow", "child-noaccept-allow"} {
		t.Run(mode, func(t *testing.T) {
			parallelOracle(t)
			historyTestPages(t, func(t *testing.T, p *Page) {
				child := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !strings.Contains(mode, "child-noaccept") {
						w.Header().Set("Accept-CH", "Sec-CH-UA-Platform-Version")
					}
					if r.URL.Path == "/child" {
						w.Header().Set("Content-Type", "text/html")
						fmt.Fprint(w, `<script>onmessage=async()=>{const a=await(await fetch('/fetch')).text();const b=await new Promise(resolve=>{const x=new XMLHttpRequest();x.open('GET','/xhr');x.onload=()=>resolve(x.responseText);x.send()});parent.postMessage([a,b,document.featurePolicy.allowsFeature('ch-ua-platform-version')],'*')}</script>`)
					} else {
						fmt.Fprint(w, r.Header.Get("Sec-CH-UA-Platform-Version"))
					}
				}))
				defer child.Close()
				top := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !strings.HasPrefix(mode, "noaccept") {
						w.Header().Set("Accept-CH", "Sec-CH-UA-Platform-Version")
					}
					if strings.Contains(mode, "deny") {
						w.Header().Set("Permissions-Policy", "ch-ua-platform-version=()")
					}
					if strings.Contains(mode, "delegate") {
						w.Header().Set("Permissions-Policy", `ch-ua-platform-version=(self "`+child.URL+`")`)
					}
					w.Header().Set("Content-Type", "text/html")
					if r.URL.Path == "/probe" {
						fmt.Fprint(w, r.Header.Get("Sec-CH-UA-Platform-Version"))
					} else {
						fmt.Fprint(w, "<!doctype html><body>")
					}
				}))
				defer top.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := p.Navigate(ctx, top.URL); err != nil {
					t.Fatal(err)
				}
				allow := ""
				if strings.Contains(mode, "allow") {
					allow = "ch-ua-platform-version"
				}
				script := `(async()=>{const own=await(await fetch('/probe')).text();const f=document.createElement('iframe');f.allow=` + strconv.Quote(allow) + `;if(f.getAttribute('allow')!==` + strconv.Quote(allow) + `)throw Error('allow reflection');f.src=` + strconv.Quote(child.URL+"/child") + `;await new Promise(resolve=>{f.onload=resolve;document.body.appendChild(f)});const values=await new Promise(resolve=>{onmessage=e=>resolve(e.data);f.contentWindow.postMessage('go','*')});return JSON.stringify([!!own,...values.map(Boolean)])})()`
				got, err := p.Evaluate(ctx, script)
				topHint := !strings.Contains(mode, "deny") && !strings.HasPrefix(mode, "noaccept")
				childHint := mode != "default" && mode != "deny-allow" && mode != "noaccept-allow"
				want, _ := json.Marshal([]bool{topHint, childHint, childHint, mode != "default" && mode != "deny-allow"})
				if err != nil || got != string(want) {
					t.Fatalf("got %v, %v; want %s", got, err, want)
				}
			})
		})
	}
}

func TestClientHintsWorkerRequests(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Accept-CH", "Sec-CH-UA-Platform-Version")
			if r.URL.Path == "/worker.js" || r.URL.Path == "/probe" {
				for name := range r.Header {
					if strings.HasPrefix(strings.ToLower(name), "sec-ch-") {
						t.Errorf("worker %s sent %s", r.URL.Path, name)
					}
				}
			}
			if r.URL.Path == "/worker.js" && (r.Header.Get("Sec-Fetch-Dest") != "worker" || r.Header.Get("Sec-Fetch-Mode") != "same-origin" || r.Header.Get("Accept") != "*/*") {
				t.Errorf("worker metadata: %v", r.Header)
			}
			switch r.URL.Path {
			case "/worker.js":
				w.Header().Set("Content-Type", "text/javascript")
				fmt.Fprint(w, `fetch('/probe').then(r=>r.text()).then(postMessage)`)
			case "/probe":
				fmt.Fprint(w, "ok")
			default:
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<!doctype html><body>")
			}
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		for _, blob := range []bool{false, true} {
			urlExpression := strconv.Quote(server.URL + "/worker.js")
			if blob {
				urlExpression = `URL.createObjectURL(new Blob([` + strconv.Quote("fetch("+strconv.Quote(server.URL+"/probe")+").then(r=>r.text()).then(postMessage)") + `]))`
			}
			value, err := p.Evaluate(ctx, `new Promise((resolve,reject)=>{const u=`+urlExpression+`;const w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(e.message)})`)
			if err != nil || value != "ok" {
				t.Fatalf("blob=%v: %v, %v", blob, value, err)
			}
		}
	})
}
