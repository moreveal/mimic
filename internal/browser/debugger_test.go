package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDebuggerUnsafeEvalScopeMatchesChrome152(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "script-src 'nonce-test'; require-trusted-types-for 'script'")
		_, _ = w.Write([]byte(`<body>strict CSP</body>`))
	}))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	d := NewDebugger(p)
	defer d.Close()
	got := debuggerEval(t, d, `new Function('return 42')()`, DebuggerOptions{})
	if got["value"] != float64(42) {
		t.Fatal(got)
	}
	blocked, err := d.Evaluate(ctx, "", "", `new Function('return 42')()`, DebuggerOptions{RespectCSP: true})
	if err != nil || blocked["exceptionDetails"] == nil {
		t.Fatalf("explicit CSP: %v %v", blocked, err)
	}
	called, err := d.CallFunction(ctx, "", "", `function(){return new Function('return 43')()}`, map[string]any{}, DebuggerOptions{})
	if err != nil || called["exceptionDetails"] != nil || called["result"].(map[string]any)["value"] != float64(43) {
		t.Fatalf("callFunctionOn: %v %v", called, err)
	}
	got = debuggerEval(t, d, `new Promise(resolve=>setTimeout(()=>{try{new Function('return 1')();resolve('leaked')}catch(e){resolve(e.name)}},0))`, DebuggerOptions{AwaitPromise: true})
	if got["value"] != "EvalError" {
		t.Fatalf("inspector scope leaked to timer: %v", got)
	}
	author, err := p.Evaluate(ctx, `(()=>{try{new Function('return 1')();return 'leaked'}catch(e){return e.name}})()`)
	if err != nil || author != "EvalError" {
		t.Fatalf("inspector scope leaked to author: %v %v", author, err)
	}
}

func debuggerEval(t *testing.T, d *Debugger, source string, options DebuggerOptions) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := d.Evaluate(ctx, "", "", source, options)
	if err != nil {
		t.Fatalf("evaluate %s: %v", source, err)
	}
	if result["exceptionDetails"] != nil {
		t.Fatalf("evaluate %s: %#v", source, result)
	}
	return result["result"].(map[string]any)
}

func TestDebuggerRemoteValuesMatchChrome152(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		for _, test := range []struct {
			source string
			want   map[string]any
		}{
			{"undefined", map[string]any{"type": "undefined"}},
			{"null", map[string]any{"type": "object", "subtype": "null", "value": nil}},
			{"'hello'", map[string]any{"type": "string", "value": "hello"}},
			{"true", map[string]any{"type": "boolean", "value": true}},
			{"42", map[string]any{"type": "number", "value": float64(42), "description": "42"}},
			{"NaN", map[string]any{"type": "number", "unserializableValue": "NaN", "description": "NaN"}},
			{"Infinity", map[string]any{"type": "number", "unserializableValue": "Infinity", "description": "Infinity"}},
			{"-Infinity", map[string]any{"type": "number", "unserializableValue": "-Infinity", "description": "-Infinity"}},
			{"-0", map[string]any{"type": "number", "unserializableValue": "-0", "description": "-0"}},
			{"123n", map[string]any{"type": "bigint", "unserializableValue": "123n", "description": "123n"}},
		} {
			if got := debuggerEval(t, d, test.source, DebuggerOptions{}); !reflect.DeepEqual(got, test.want) {
				t.Errorf("%s: %#v want %#v", test.source, got, test.want)
			}
		}
		for _, test := range []struct{ source, subtype string }{{"[1,2]", "array"}, {"document.body", "node"}, {"Promise.resolve(3)", "promise"}, {"new Map([[1,2]])", "map"}, {"new Uint8Array([1,2])", "typedarray"}} {
			got := debuggerEval(t, d, test.source, DebuggerOptions{})
			if got["subtype"] != test.subtype || got["objectId"] == nil || got["value"] != nil {
				t.Errorf("%s: %#v", test.source, got)
			}
		}
	})
}

func TestDebuggerPreservesHandlesAndDescriptorSemantics(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		ctx := context.Background()
		object := debuggerEval(t, d, `globalThis.getterReads=0;globalThis.handleObject={number:7,get accessor(){getterReads++;return 11}};handleObject.self=handleObject;handleObject`, DebuggerOptions{ObjectGroup: "test"})
		id := object["objectId"].(string)
		properties, err := d.GetProperties(ctx, map[string]any{"objectId": id, "ownProperties": true})
		if err != nil {
			t.Fatal(err)
		}
		var selfID string
		for _, raw := range properties["result"].([]any) {
			property := raw.(map[string]any)
			if property["name"] == "self" {
				selfID = property["value"].(map[string]any)["objectId"].(string)
			}
			if property["name"] == "accessor" && property["get"].(map[string]any)["type"] != "function" {
				t.Fatalf("accessor: %#v", property)
			}
		}
		if got := debuggerEval(t, d, "getterReads", DebuggerOptions{}); got["value"] != float64(0) {
			t.Fatal(got)
		}
		params := map[string]any{"objectId": id, "arguments": []any{map[string]any{"objectId": selfID}, map[string]any{"unserializableValue": "-0"}, map[string]any{"unserializableValue": "123n"}, map[string]any{}}}
		result, err := d.CallFunction(ctx, "", "", `function(other,negative,big,empty){return this===other && this===handleObject && Object.is(negative,-0) && big===123n && empty===undefined}`, params, DebuggerOptions{})
		if err != nil || result["result"].(map[string]any)["value"] != true {
			t.Fatalf("call: %#v %v", result, err)
		}
		if err := d.ReleaseObjectGroup(ctx, "test"); err != nil {
			t.Fatal(err)
		}
		if _, err = d.GetProperties(ctx, map[string]any{"objectId": selfID}); err == nil {
			t.Fatal("released child handle remained available")
		}
		if _, err = d.GetProperties(ctx, map[string]any{"objectId": id}); err == nil {
			t.Fatal("released root handle remained available")
		}
		if debuggerEval(t, d, "handleObject.number", DebuggerOptions{})["value"] != float64(7) {
			t.Fatal("release destroyed page object")
		}
	})
}

func TestDebuggerCallFunctionInvalidatesDOMReadSnapshotAfterMutation(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		result, err := d.CallFunction(context.Background(), "", "", `function(){
			const first=document.createElement('div'),second=document.createElement('section'),child=document.createElement('span');
			first.appendChild(child);document.body.append(first,second);child.setAttribute('data-state','before');
			const before=child.parentNode===first&&child.getAttribute('data-state')==='before';
			second.appendChild(child);child.setAttribute('data-state','after');
			return [before,child.parentNode===second,child.getAttribute('data-state')];
		}`, map[string]any{}, DebuggerOptions{ReturnByValue: true})
		if err != nil {
			t.Fatal(err)
		}
		got := result["result"].(map[string]any)["value"]
		if !reflect.DeepEqual(got, []any{true, true, "after"}) {
			t.Fatalf("post-mutation reads = %#v", got)
		}
	})
}

func TestDebuggerReturnByValueAndAwait(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		for _, source := range []string{`({a:1,toJSON(){throw Error('must not run')}})`, `new Date(0)`, `()=>{}`} {
			result := debuggerEval(t, d, source, DebuggerOptions{ReturnByValue: true})
			if result["objectId"] != nil || result["value"] == nil {
				t.Fatal(result)
			}
		}
		_, err := d.Evaluate(context.Background(), "", "", `(()=>{const x={};x.self=x;return x})()`, DebuggerOptions{ReturnByValue: true})
		if err == nil || !strings.Contains(err.Error(), "Object reference chain is too long") {
			t.Fatalf("cycle: %v", err)
		}
		got := debuggerEval(t, d, `new Promise(resolve=>setTimeout(()=>resolve(17),5))`, DebuggerOptions{AwaitPromise: true})
		if got["value"] != float64(17) {
			t.Fatal(got)
		}
		got = debuggerEval(t, d, `new Promise(resolve=>{setTimeout(()=>{throw new EvalError('unrelated task')},0);setTimeout(()=>resolve(19),1)})`, DebuggerOptions{AwaitPromise: true})
		if got["value"] != float64(19) {
			t.Fatal(got)
		}
		for _, source := range []string{`throw 42`, `Promise.reject(42)`} {
			result, err := d.Evaluate(context.Background(), "", "", source, DebuggerOptions{AwaitPromise: true})
			if err != nil || result["exceptionDetails"] == nil || result["result"].(map[string]any)["value"] != float64(42) {
				t.Fatalf("exception: %#v %v", result, err)
			}
		}
	})
}

func TestDebuggerCanonicalNodeAndSessionIsolation(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		other := NewDebugger(page)
		defer other.Close()
		ctx := context.Background()
		body := debuggerEval(t, d, "document.body", DebuggerOptions{})
		id, err := d.RequestNode(ctx, body["objectId"].(string))
		if err != nil || id == 0 {
			t.Fatalf("node %d %v", id, err)
		}
		resolved, err := d.ResolveNode(ctx, "", "", id, "")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.CallFunction(ctx, "", "", `function(){return this===document.body}`, map[string]any{"objectId": resolved["objectId"]}, DebuggerOptions{})
		if err != nil || result["result"].(map[string]any)["value"] != true {
			t.Fatalf("identity %#v %v", result, err)
		}
		if _, err := other.GetProperties(ctx, map[string]any{"objectId": body["objectId"]}); err == nil {
			t.Fatal("handle crossed protocol session")
		}
		fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<p>new document</p>")) }))
		defer fixture.Close()
		if err := page.Navigate(ctx, fixture.URL); err != nil {
			t.Fatal(err)
		}
		if _, err := d.GetProperties(ctx, map[string]any{"objectId": body["objectId"]}); err == nil {
			t.Fatal("handle survived realm replacement")
		}
	})
}

func TestDebuggerIntrinsicsArePrivateAndCapturedBeforePageScripts(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		debuggerEval(t, d, `globalThis.originalKeys=Object.keys(globalThis);globalThis.poisonReads=0;Object.getOwnPropertyDescriptor=()=>{poisonReads++;throw 7};Object.prototype.toJSON=()=>{poisonReads++;throw 8};Array.prototype.toJSON=()=>{poisonReads++;throw 9};Number.isFinite=()=>{poisonReads++;return false};globalThis.Map=function(){throw 10};globalThis.Set=function(){throw 11}`, DebuggerOptions{})
		value := debuggerEval(t, d, `({answer:42,items:[1,2]})`, DebuggerOptions{ReturnByValue: true})
		if value["value"].(map[string]any)["answer"] != float64(42) {
			t.Fatal(value)
		}
		object := debuggerEval(t, d, `({answer:42})`, DebuggerOptions{})
		if _, err := d.GetProperties(context.Background(), map[string]any{"objectId": object["objectId"], "ownProperties": true}); err != nil {
			t.Fatal(err)
		}
		if got := debuggerEval(t, d, `poisonReads`, DebuggerOptions{}); got["value"] != float64(0) {
			t.Fatal(got)
		}
		if got := debuggerEval(t, d, `Object.keys(globalThis).filter(key=>!originalKeys.includes(key)).sort().join(',')`, DebuggerOptions{}); got["value"] != "originalKeys,poisonReads" {
			t.Fatalf("debugger leaked globals: %#v", got)
		}
	})
}
