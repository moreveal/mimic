//go:build (windows || linux) && amd64

package v8

import (
	"reflect"
	"strings"
	"testing"
)

func TestSpikeLifecycleHostRealmMicrotasksAndExceptions(t *testing.T) {
	runtime, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Dispose()

	first, err := runtime.NewRealm()
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.NewRealm()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Dispose()
	defer second.Dispose()

	err = first.SetHostObject("host", map[string]any{
		"prefix": "v8",
		"join": HostFunction(func(args []string) (any, error) {
			return strings.Join(args, ":"), nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := first.Eval(`({host:host.join(host.prefix, 42), global:globalThis===this})`, "host.js")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"host": "v8:42", "global": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	_, err = first.Eval(`globalThis.realmMarker=1`, "first.js")
	if err != nil {
		t.Fatal(err)
	}
	isolated, err := second.Eval(`typeof realmMarker`, "second.js")
	if err != nil || isolated != "undefined" {
		t.Fatalf("realm isolation = %#v, %v", isolated, err)
	}

	_, err = first.Eval(`globalThis.order=[];Promise.resolve().then(()=>order.push('microtask'));order.push('sync')`, "promise.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	order, err := first.Eval(`order`, "order.js")
	if err != nil || !reflect.DeepEqual(order, []any{"sync", "microtask"}) {
		t.Fatalf("microtask order = %#v, %v", order, err)
	}

	if _, err := first.Eval(`(()=>{throw new TypeError('boom')})()`, "exception.js"); err == nil || !strings.Contains(err.Error(), "TypeError: boom") {
		t.Fatalf("exception conversion = %v", err)
	}
}
