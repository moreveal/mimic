package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/network"
)

func TestResourcePolicyContextIsolationAndHotUpdate(t *testing.T) {
	parallelBrowserTest(t)
	first := testPage(t)
	second := testPage(t)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	blocked := false
	policy := network.ResourcePolicy{Rules: []network.ResourceRule{{ID: "images", Match: network.ResourceMatch{Kinds: []string{"image"}}, Work: network.ResourceWork{Network: &blocked}}}}
	firstGeneration, err := first.ctx.UpdateResourcePolicy(policy)
	if err != nil || firstGeneration != 1 {
		t.Fatalf("update: %d %v", firstGeneration, err)
	}
	request := network.Request{URL: target, Initiator: network.Image}
	if _, err := first.Loader().Load(context.Background(), request); err == nil {
		t.Fatal("first context did not block")
	}
	if _, err := second.Loader().Load(context.Background(), request); err != nil {
		t.Fatalf("second context: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("network requests: %d", requests.Load())
	}
	allowed := true
	policy.Rules[0].Work.Network = &allowed
	if _, err := first.ctx.UpdateResourcePolicy(policy); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Loader().Load(context.Background(), request); err != nil {
		t.Fatalf("updated context: %v", err)
	}
	if requests.Load() != 2 || first.ctx.ResourcePolicyStats().Generation != 2 {
		t.Fatalf("requests %d stats %+v", requests.Load(), first.ctx.ResourcePolicyStats())
	}
}

func TestResourcePolicyDecodedBudgetAppliesToLatePixels(t *testing.T) {
	serialBrowserTest(t)
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAYAAAD0In+KAAAADklEQVR4nGP4z8DwHwQBEPgD/U6VwW8AAAAASUVORK5CYII=")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
			return
		}
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()
	p := testPage(t)
	if _, err := p.ctx.UpdateResourcePolicy(network.ResourcePolicy{Budgets: network.ResourceBudgets{MaxDecodedBytes: 8}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(async()=>{let image=new Image();image.src='/image';await image.decode();return image.naturalWidth})()`)
	if err != nil || fmt.Sprint(value) != "2" {
		t.Fatalf("image metadata: %v %v", value, err)
	}
	if got := p.ctx.ResourcePolicyStats().DecodedPixelWorkBytes; got != 8 {
		t.Fatalf("validation work bytes = %d", got)
	}
	for _, entry := range p.Top.Realm.availableImages.entries {
		if _, err := entry.Value.(availableImageEntry).image.resource.RequireDecodedImage(); err == nil {
			t.Fatal("late pixels bypassed decoded budget")
		}
	}
}
