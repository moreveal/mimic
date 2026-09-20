package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

func TestCanvasImplementedReflectionMatchesChrome(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/canvas_reflection_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/canvas-reflection-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result       json.RawMessage `json:"result"`
		WorkerResult json.RawMessage `json:"workerResult"`
	}
	if err = json.Unmarshal(capture, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		workerSource := `onmessage=()=>{try{const actual=` + string(source) + `,expected=` + string(oracle.WorkerResult) + `;for(const [name,value]of Object.entries(actual)){for(const key of Object.keys(value)){if(key==='members'){for(const [member,descriptor]of Object.entries(value.members))if(JSON.stringify(descriptor)!==JSON.stringify(expected[name].members[member]))throw Error(name+'.'+member+JSON.stringify(descriptor)+' expected '+JSON.stringify(expected[name].members[member]))}else if(value[key]!==expected[name][key])throw Error(name+'.'+key)}}postMessage(true)}catch(error){postMessage(String(error))}}`
		historyEval(t, p, `new Promise((resolve,reject)=>{const url=URL.createObjectURL(new Blob([`+strconv.Quote(workerSource)+`])),worker=new Worker(url);worker.onerror=e=>reject(Error(e.message));worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data)};worker.postMessage(null)})`, true)
		historyEval(t, p, `(()=>{const actual=`+string(source)+`,expected=`+string(oracle.Result)+`;const differences=[];for(const [name,value]of Object.entries(actual)){const native=expected[name];for(const key of Object.keys(value)){if(key==='members'){for(const [member,descriptor]of Object.entries(value.members)){if(JSON.stringify(descriptor)!==JSON.stringify(native.members[member]))differences.push(name+'.'+member+':'+JSON.stringify(descriptor)+' expected '+JSON.stringify(native.members[member]))}}else if(value[key]!==native[key])differences.push(name+'.'+key+':'+value[key]+' expected '+native[key])}}if(differences.length)throw Error(differences.join(';'));return true})()`, true)
	})
}
