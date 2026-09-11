package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestNavigationBasicOracle(t *testing.T)     { navigationOracle(t, "basic") }
func TestNavigationInterceptOracle(t *testing.T) { navigationOracle(t, "intercept") }
func navigationOracle(t *testing.T, name string) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>fixture")) }))
	defer server.Close()
	source, _ := os.ReadFile("testdata/navigation_" + name + "_oracle.js")
	expected, _ := os.ReadFile("testdata/navigation_" + name + "_chrome152.json")
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/plain"); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(context.Background(), "Promise.resolve("+string(source)+").then(JSON.stringify)")
		if err != nil {
			t.Fatal(err)
		}
		data := []byte(value.(string))
		var got, want any
		json.Unmarshal(data, &got)
		json.Unmarshal(expected, &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s\nwant %s", data, expected)
		}
	})
}
func TestNavigationChildCrossDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>fixture")) }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/plain"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, loadHistoryChild, true)
		historyEval(t, p, `new Promise(resolve=>{childFrame.onload=()=>resolve(true);childFrame.contentWindow.navigation.navigate('/other',{history:'push',state:{v:8}})})`, true)
		historyEval(t, p, `childFrame.contentWindow.location.pathname`, "/other")
		historyEval(t, p, `childFrame.contentWindow.navigation.currentEntry.getState()===undefined`, true)
		historyEval(t, p, `new Promise(resolve=>{const n=childFrame.contentWindow.navigation,key=n.currentEntry.key,id=n.currentEntry.id;childFrame.onload=()=>resolve(childFrame.contentWindow.navigation.currentEntry.key===key&&childFrame.contentWindow.navigation.currentEntry.id===id);n.reload()})`, true)
		historyEval(t, p, `new Promise(resolve=>{childFrame.onload=()=>resolve(true);childFrame.contentWindow.navigation.back()})`, true)
		historyEval(t, p, `childFrame.contentWindow.location.pathname`, "/child/original")
	})
}
func TestNavigationGraphStateAndRealm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>fixture")) }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/plain"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{const x={a:-0,date:new Date(NaN),big:12n,map:new Map(),set:new Set([1]),buffer:new Uint8Array([3,4])};x.self=x;x.map.set(x,x.buffer);navigation.updateCurrentEntry({state:x});x.buffer[0]=9;const y=navigation.currentEntry.getState(),z=navigation.currentEntry.getState();return y.self===y&&y!==z&&y.map.get(y)===y.buffer&&y.buffer[0]===3&&Object.is(y.a,-0)&&Number.isNaN(y.date.getTime())&&y.big===12n&&y.set.has(1)})()`, true)
		historyEval(t, p, loadHistoryChild, true)
		historyEval(t, p, `(()=>{const input={v:7};childFrame.contentWindow.history.pushState(input,'','#saved');input.v=9;return childFrame.contentWindow.history.state.v===7})()`, true)
		historyEval(t, p, `new Promise(resolve=>{childFrame.onload=()=>resolve(true);childFrame.contentWindow.navigation.navigate('/next',{history:'push'})})`, true)
		historyEval(t, p, `new Promise(resolve=>{childFrame.onload=()=>resolve(true);childFrame.contentWindow.navigation.back()})`, true)
		historyEval(t, p, `childFrame.contentWindow.history.state.v===7`, true)
	})
}

func TestNavigationHashChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<!doctype html><body><div id='target'>target</div>"))
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/plain"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `new Promise(resolve=>{addEventListener('hashchange',e=>resolve(e.oldURL.endsWith('/plain')&&e.newURL.endsWith('/plain#target')&&document.querySelector(':target').id==='target'),{once:true});navigation.navigate('#target')})`, true)
	})
}
