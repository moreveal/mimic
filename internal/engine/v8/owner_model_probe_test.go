//go:build windows && amd64

package v8

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestOwnerModelProbe(t *testing.T) {
	clockDLL := windows.NewLazySystemDLL("kernel32.dll")
	counter := clockDLL.NewProc("QueryPerformanceCounter")
	frequency := clockDLL.NewProc("QueryPerformanceFrequency")
	var frequencyValue int64
	frequency.Call(uintptr(unsafe.Pointer(&frequencyValue)))
	tick := func() float64 {
		var value int64
		counter.Call(uintptr(unsafe.Pointer(&value)))
		return float64(value) * 1000 / float64(frequencyValue)
	}
	dir := os.Getenv("MIMIC_OWNER_MODEL_PROBE")
	if dir == "" {
		t.Skip("diagnostic")
	}
	os.MkdirAll(dir, 0700)
	rows := []map[string]any{}
	for round := 0; round < 6; round++ {
		start := tick()
		r, e := NewRuntime()
		if e != nil {
			t.Fatal(e)
		}
		rows = append(rows, map[string]any{"phase": "new-isolate", "round": round, "ms": tick() - start})
		start = tick()
		a, e := r.NewRealm()
		if e != nil {
			t.Fatal(e)
		}
		b, e := r.NewRealm()
		if e != nil {
			t.Fatal(e)
		}
		rows = append(rows, map[string]any{"phase": "two-contexts", "round": round, "ms": tick() - start})
		_, e = r.execute(func(s *state) response {
			ca, cb := s.realms[a.id], s.realms[b.id]
			_, e := evalText(s.isolate, cb, `globalThis.chain=Object.create(Object.create(Object.create(null)));globalThis.fn=x=>x+1;globalThis.obj={x:7};true`, "child")
			if e != nil {
				return response{err: e}
			}
			scope, e := s.isolate.NewScope()
			if e != nil {
				return response{err: e}
			}
			defer scope.Close()
			token, e := scope.NewString("https://same-origin.test")
			if e != nil {
				return response{err: e}
			}
			if e = ca.SetSecurityToken(scope, token); e != nil {
				return response{err: e}
			}
			if e = cb.SetSecurityToken(scope, token); e != nil {
				return response{err: e}
			}
			parent, e := ca.GlobalObject(scope)
			if e != nil {
				return response{err: e}
			}
			child, e := cb.GlobalObject(scope)
			if e != nil {
				return response{err: e}
			}
			_, e = parent.SetByName(scope, ca, "w", child.Value)
			return response{err: e}
		})
		if e != nil {
			t.Fatal(e)
		}
		identity, e := a.Eval("w.Array!==Array && w.Object!==Object && Object.getPrototypeOf(w.obj)===w.Object.prototype", "realm-identity")
		if e != nil || identity != true {
			t.Fatalf("realm identity %v %v", identity, e)
		}
		for _, n := range []int{100, 100000} {
			start = tick()
			value, e := a.Eval(fmt.Sprintf(`(()=>{let checks=0;for(let i=0;i<%d;i++){let p=w.chain;while(p!==null){checks++;p=Object.getPrototypeOf(p)}checks+=w.obj.x;checks+=w.fn(i)}return checks})()`, n), "shared-contexts")
			if e != nil {
				t.Fatal(e)
			}
			want := float64(10*n + n*(n+1)/2)
			if value != want {
				t.Fatalf("checksum %v want %v", value, want)
			}
			rows = append(rows, map[string]any{"phase": "shared-contexts", "iterations": n, "round": round, "ms": tick() - start, "result": value})
		}
		start = tick()
		for i := 0; i < 10000; i++ {
			_, e = r.execute(func(*state) response { return response{value: 7} })
			if e != nil {
				t.Fatal(e)
			}
		}
		rows = append(rows, map[string]any{"phase": "actor-10000", "round": round, "ms": tick() - start})
		start = tick()
		r.Dispose()
		rows = append(rows, map[string]any{"phase": "dispose", "round": round, "ms": tick() - start})
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	os.WriteFile(filepath.Join(dir, "model.json"), data, 0600)
}
