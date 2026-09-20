package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResourceTimingBufferNotificationsReleaseValues(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/data" {
			fmt.Fprint(w, "body")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><p>ready</p>")
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(ctx, `globalThis.fullNotifications=0;performance.setResourceTimingBufferSize(1);performance.onresourcetimingbufferfull=()=>{fullNotifications++}`); err != nil {
		t.Fatal(err)
	}
	const source = `(async()=>{for(let i=0;i<16;i++){const body=await(await fetch('/data')).arrayBuffer();if(body.byteLength!==4)throw new Error('body')}return true})()`
	baseline := -1
	for iteration := 0; iteration < 3; iteration++ {
		value, err := p.Evaluate(ctx, source)
		if err != nil || value != true {
			t.Fatalf("resource delivery: %v %v", value, err)
		}
		handles := persistentHandleCount(t, p.Top.Realm.runtime)
		if baseline < 0 {
			baseline = handles
		} else if handles != baseline {
			t.Fatalf("buffer-full notifications kept %d scratch roots", handles-baseline)
		}
	}
	value, err := p.Evaluate(ctx, `fullNotifications>30`)
	if err != nil || value != true {
		t.Fatalf("buffer-full callbacks did not execute: %v %v", value, err)
	}
}
