package browser

import (
	"context"
	"errors"
	"fmt"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestV8EvaluateDeadlineCoversPromiseCheckpoint(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = p.Evaluate(ctx, `Promise.resolve().then(function loop(){Promise.resolve().then(loop)})`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("checkpoint deadline: %v", err)
	}
}

func TestRemovingFrameCancelsPendingDocumentTransport(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/child" {
			close(started)
			<-req.Context().Done()
			close(cancelled)
			return
		}
		fmt.Fprint(w, "<body></body>")
	}))
	defer server.Close()
	b, _ := New(v8engine.Factory{}, chrome152.New())
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(ctx, `globalThis.pendingFrame=document.createElement('iframe');pendingFrame.src='/child';document.body.appendChild(pendingFrame);void 0`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("child transport did not start")
	}
	if _, err := p.Evaluate(ctx, `pendingFrame.remove();void 0`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-ctx.Done():
		t.Fatal("removed frame transport was not cancelled")
	}
}
