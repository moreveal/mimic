package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestAwaitParentPromiseFromDelayedChildFetch(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/data" {
				time.Sleep(40 * time.Millisecond)
				fmt.Fprint(w, "fetched")
				return
			}
			fmt.Fprint(w, "<!doctype html><body>fixture</body>")
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL+"/top"); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Evaluate(ctx, loadHistoryChild); err != nil {
			t.Fatal(err)
		}
		ctx, cancelFetch := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFetch()
		child := `fetch('/data').then(r=>r.text()).then(text=>parent.postMessage(text,'*'));true`
		value, err := p.Evaluate(ctx, `new Promise(resolve=>{addEventListener('message',e=>resolve(e.data),{once:true});childFrame.contentWindow.eval(`+strconv.Quote(child)+`)})`)
		if err != nil || value != "fetched" {
			t.Fatalf("delayed child result=%v err=%v", value, err)
		}
	})
}

func TestAwaitChildWorkCancellationAndPageTeardown(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		started, canceled := make(chan struct{}), make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/pending" {
				close(started)
				<-r.Context().Done()
				close(canceled)
				return
			}
			fmt.Fprint(w, "<!doctype html><body>fixture</body>")
		}))
		defer server.Close()
		// Close before the HTTP server even if the assertions fail.
		defer p.Close()
		if err := p.Navigate(context.Background(), server.URL+"/top"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, loadHistoryChild, true)
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()
		_, err := p.Evaluate(ctx, `new Promise(resolve=>{childFrame.contentWindow.eval("fetch('/pending');true")})`)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("await cancellation: %v", err)
		}
		select {
		case <-started:
		default:
			t.Fatal("child request never started")
		}
		// Canceling a debugger evaluation leaves the Page usable.
		historyEval(t, p, "1+1===2", true)
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("Page teardown did not cancel child request")
		}
	})
}
