package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestOPFSOriginOwnershipAndLifecycle(t *testing.T) {
	c := &Context{}
	s := c.opfs("https://one.test")
	if s != c.opfs("https://one.test") || s == c.opfs("https://two.test") || s == (&Context{}).opfs("https://one.test") {
		t.Fatal("origin/context ownership")
	}
	owner := &opfsOwner{stores: map[*opfsStore]bool{s: true}}
	other := &opfsOwner{stores: map[*opfsStore]bool{s: true}}
	file := s.call(owner, "child", 1, map[string]any{"name": "data", "kind": "file", "create": true}).(map[string]any)
	id := file["id"].(int)
	access := s.call(owner, "open", id, map[string]any{"mode": "readwrite"}).(int)
	if got := s.call(other, "write", access, map[string]any{"data": []any{1.0}}).(map[string]any)["error"]; got != "InvalidStateError" {
		t.Fatal(got)
	}
	s.call(owner, "write", access, map[string]any{"at": 2.0, "data": []any{1.0, 2.0}})
	owner.close()
	next := s.call(other, "open", id, map[string]any{"mode": "readwrite"}).(int)
	if got := s.call(other, "size", next, nil); got != 4 {
		t.Fatal(got)
	}
	s.call(other, "close", next, nil)
	staged := s.call(other, "openWritable", id, map[string]any{"mode": "siloed"}).(int)
	s.call(other, "write", staged, map[string]any{"data": []any{9.0}})
	other.close()
	data := s.call(owner, "file", id, nil).(map[string]any)["data"].([]int)
	if len(data) != 4 || data[2] != 1 {
		t.Fatal("teardown committed abandoned staging", data)
	}
	foreign := c.opfs("https://two.test")
	if got := foreign.call(owner, "restore", id, map[string]any{"store": file["store"]}).(map[string]any)["error"]; got != "DataCloneError" {
		t.Fatal("foreign store accepted", got)
	}
	s.call(owner, "remove", 1, map[string]any{"name": "data"})
	if s.nodes[id].data != nil {
		t.Fatal("deleted bytes retained")
	}
	if got := s.call(owner, "file", id, nil).(map[string]any)["error"]; got != "NotFoundError" {
		t.Fatal(got)
	}
	// A serialized handle retains its deleted entry identity; it cannot resurrect it.
	if got := s.call(owner, "restore", id, map[string]any{"store": file["store"]}).(map[string]any)["id"]; got != id {
		t.Fatal(got)
	}
}

func TestOPFSConcurrentAccessLockAndWrites(t *testing.T) {
	s := newOPFSStore()
	rootOwner := &opfsOwner{}
	id := s.call(rootOwner, "child", 1, map[string]any{"name": "concurrent", "kind": "file", "create": true}).(map[string]any)["id"].(int)
	const count = 12
	var wg sync.WaitGroup
	results := make(chan any, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o := &opfsOwner{}
			results <- s.call(o, "open", id, map[string]any{"mode": "readwrite"})
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for result := range results {
		if _, ok := result.(int); ok {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("exclusive open winners=%d", winners)
	}
	s.access = map[int]*opfsAccess{}
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o := &opfsOwner{stores: map[*opfsStore]bool{s: true}}
			defer o.close()
			a := s.call(o, "open", id, map[string]any{"mode": "readwrite-unsafe"}).(int)
			s.call(o, "write", a, map[string]any{"at": float64(i), "data": []any{float64(i + 1)}})
		}(i)
	}
	wg.Wait()
	data := s.call(rootOwner, "file", id, nil).(map[string]any)["data"].([]int)
	for i, b := range data {
		if b != i+1 {
			t.Fatalf("lost overlapping-store write %d=%d", i, b)
		}
	}
	if len(s.access) != 0 {
		t.Fatal("access locks retained")
	}
}

func TestOPFSPagesShareOnlyTheirOriginContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html>")) }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(async()=>{const f=await (await navigator.storage.getDirectory()).getFileHandle('shared.txt',{create:true});const w=await f.createWritable();await w.write('shared');await w.close();const c=structuredClone([f,f]);return c[0]===c[1]&&await c[0].isSameEntry(f)})()`, true)
		other, err := p.ctx.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		defer other.Close()
		if err = other.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, other, `(async()=>{const f=await (await navigator.storage.getDirectory()).getFileHandle('shared.txt');return (await f.getFile()).text()})()`, "shared")
		isolated := p.ctx.browser.NewContext()
		defer isolated.Close()
		third, err := isolated.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		if err = third.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, third, `(async()=>{try{await (await navigator.storage.getDirectory()).getFileHandle('shared.txt');return false}catch(e){return e.name==='NotFoundError'}})()`, true)
	})
}
