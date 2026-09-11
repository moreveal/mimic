package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIndexedDBChromeOracles(t *testing.T) {
	for _, name := range []string{"indexeddb_storage", "indexeddb_lifecycle", "indexeddb_realms"} {
		t.Run(name, func(t *testing.T) {
			historyTestPages(t, func(t *testing.T, p *Page) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body>") }))
				defer server.Close()
				if err := p.Navigate(ctx, server.URL); err != nil {
					t.Fatal(err)
				}
				if name == "indexeddb_lifecycle" {
					// Navigation without script can retain deferredRuntime. Realize
					// the engine before inspecting its checkpoint capability.
					if _, err := p.Evaluate(ctx, "void 0"); err != nil {
						t.Fatal(err)
					}
					if _, ok := p.Top.Realm.runtime.(interface{ NativeCallbackCheckpoint() error }); !ok {
						if strings.HasSuffix(t.Name(), "/goja") {
							t.Skip("Goja cannot perform a native callback microtask checkpoint between event listeners")
						}
						t.Fatal("realized engine is missing the native callback checkpoint capability")
					}
				}
				source, err := os.ReadFile("testdata/" + name + "_oracle.js")
				if err != nil {
					t.Fatal(err)
				}
				reference, err := os.ReadFile("testdata/" + name + "_chrome152.json")
				if err != nil {
					t.Fatal(err)
				}
				value, err := p.Evaluate(ctx, "(async()=>JSON.stringify(await ("+string(source)+")))()")
				if err != nil {
					for _, e := range p.trace.Events() {
						if e.Name == "error" {
							t.Logf("event: %+v", e)
						}
					}
					t.Fatal(err)
				}
				var got, want any
				if err = json.Unmarshal([]byte(fmt.Sprint(value)), &got); err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(reference, &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("got %s; want %s", value, reference)
				}
			})
		})
	}
}

func TestIndexedDBContextOwnershipAndRealmTeardown(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body>") }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		eval := func(page *Page, script string) any {
			t.Helper()
			v, err := page.Evaluate(ctx, script)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		eval(p, `new Promise((resolve,reject)=>{const r=indexedDB.open('shared',1);r.onupgradeneeded=()=>r.result.createObjectStore('s');r.onerror=()=>reject(r.error);r.onsuccess=()=>{globalThis.db=r.result;const t=db.transaction('s','readwrite');t.objectStore('s').put({x:12},'key');t.oncomplete=()=>resolve()}})`)
		other, err := p.ctx.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		defer other.Close()
		if err = other.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		got := eval(other, `new Promise((resolve,reject)=>{const r=indexedDB.open('shared');r.onerror=()=>reject(r.error);r.onsuccess=()=>{const db=r.result;const q=db.transaction('s').objectStore('s').get('key');q.onsuccess=()=>{db.close();resolve(q.result.x)}}})`)
		if got != int64(12) && got != float64(12) {
			t.Fatalf("shared value: %T %v", got, got)
		}
		// Leave the original connection open; navigation must release it.
		// A closed document must not retain a connection and block another realm's
		// version upgrade. Storage survives the document that created it.
		if err = p.Navigate(ctx, server.URL+"/next"); err != nil {
			t.Fatal(err)
		}
		got = eval(other, `new Promise((resolve,reject)=>{const r=indexedDB.open('shared',2);r.onupgradeneeded=()=>r.result.createObjectStore('new');r.onerror=()=>reject(r.error);r.onsuccess=()=>{const db=r.result;const result=db.version;db.close();resolve(result)}})`)
		if got != int64(2) && got != float64(2) {
			t.Fatalf("upgrade after teardown: %T %v", got, got)
		}
		// A fresh Context owns a separate catalog even for the same URL.
		isolated := p.ctx.browser.NewContext()
		defer isolated.Close()
		ip, err := isolated.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		if err = ip.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		if got := eval(ip, `indexedDB.databases().then(x=>x.length)`); got != int64(0) && got != float64(0) {
			t.Fatalf("Context leaked catalog: %v", got)
		}
	})
}

func TestIndexedDBConcurrentPageTransactions(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body>") }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		_, err := p.Evaluate(ctx, `new Promise((resolve,reject)=>{const r=indexedDB.open('parallel',1);r.onupgradeneeded=()=>r.result.createObjectStore('s').put(0,'counter');r.onerror=()=>reject(r.error);r.onsuccess=()=>{r.result.close();resolve()}})`)
		if err != nil {
			t.Fatal(err)
		}
		pages := []*Page{p}
		for i := 0; i < 3; i++ {
			q, err := p.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer q.Close()
			if err = q.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			pages = append(pages, q)
		}
		errors := make(chan error, len(pages))
		for _, page := range pages {
			go func(q *Page) {
				_, err := q.Evaluate(ctx, `(async()=>{const db=await new Promise((resolve,reject)=>{const r=indexedDB.open('parallel');r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});for(let i=0;i<10;i++){await new Promise((resolve,reject)=>{const t=db.transaction('s','readwrite'),s=t.objectStore('s'),r=s.get('counter');r.onsuccess=()=>s.put(r.result+1,'counter');t.oncomplete=resolve;t.onabort=()=>reject(t.error)})}db.close()})()`)
				errors <- err
			}(page)
		}
		for range pages {
			if err := <-errors; err != nil {
				t.Fatal(err)
			}
		}
		value, err := p.Evaluate(ctx, `new Promise((resolve,reject)=>{const r=indexedDB.open('parallel');r.onsuccess=()=>{const db=r.result,q=db.transaction('s').objectStore('s').get('counter');q.onsuccess=()=>{db.close();resolve(q.result)}};r.onerror=()=>reject(r.error)})`)
		if err != nil || value != int64(40) && value != float64(40) {
			t.Fatalf("lost transaction update: %v, %v", value, err)
		}
	})
}
