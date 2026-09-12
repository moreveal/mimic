package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Verify bytes received by an independent server, not the request projection.
func TestFormSubmitNavigation(t *testing.T) {
	for _, method := range []string{"get", "post"} {
		t.Run(method, func(t *testing.T) {
			historyTestPages(t, func(t *testing.T, p *Page) {
				type received struct{ method, query, body, contentType, origin string }
				requests := make(chan received, 4)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/result" {
						body, _ := io.ReadAll(r.Body)
						requests <- received{r.Method, r.URL.RawQuery, string(body), r.Header.Get("Content-Type"), r.Header.Get("Origin")}
					}
					fmt.Fprint(w, "<!doctype html><body></body>")
				}))
				defer server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := p.Navigate(ctx, server.URL); err != nil {
					t.Fatal(err)
				}
				_, err := p.Evaluate(ctx, `(function(){
				const f=document.createElement('form'); f.method='`+method+`';f.action='/result?old=1';
				f.innerHTML='<input name="a" value="old"><input name="off" disabled value="x"><input type="checkbox" name="check" checked><textarea name="text">line1\nline2</textarea><button name="button" value="x">submit</button>';
				document.body.appendChild(f);f.elements[0].value='new value';
				f.addEventListener('submit',()=>{throw new Error('submit() must not dispatch submit')});
				f.submit();return true})()`)
				if err != nil {
					t.Fatal(err)
				}
				select {
				case got := <-requests:
					const encoded = "a=new+value&check=on&text=line1%0D%0Aline2"
					if method == "get" {
						if got.method != "GET" || got.query != encoded || got.body != "" || got.origin != "" {
							t.Fatalf("received %+v", got)
						}
					} else if got.method != "POST" || got.query != "old=1" || got.body != encoded || got.contentType != "application/x-www-form-urlencoded" || got.origin != server.URL {
						t.Fatalf("received %+v", got)
					}
				case <-ctx.Done():
					t.Fatal("form navigation did not reach server")
				}
			})
		})
	}
}

func TestSubmitButtonActivationHonorsCancellationAndSubmitter(t *testing.T) {
	for _, button := range []string{`<button id="send" name="send" value="yes"><span>go</span></button>`, `<input id="send" type="submit" name="send" value="yes">`} {
		historyTestPages(t, func(t *testing.T, p *Page) {
			requests := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/result" {
					body, _ := io.ReadAll(r.Body)
					requests <- r.Method + ":" + string(body)
				}
				fmt.Fprint(w, "<!doctype html><body></body>")
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			_, err := p.Evaluate(ctx, `(()=>{
document.body.innerHTML='<form action="/wrong" method="get"><input name="value" value="dirty"><input name="events"><button name="other" value="no">other</button>`+button+`</form>';
const f=document.querySelector('form'),b=document.getElementById('send');b.setAttribute('formaction','/result');b.setAttribute('formmethod','post');
let cancelClick=true,cancelSubmit=true,events=[];
b.addEventListener('click',e=>{if(cancelClick)e.preventDefault()});
f.addEventListener('submit',e=>{events.push([e.submitter===b,e.isTrusted,e.bubbles,e.cancelable].join(','));if(cancelSubmit)e.preventDefault();f.querySelector('[name=events]').value=events.join('|')});
const target=b.firstElementChild||b;target.click();cancelClick=false;target.click();cancelSubmit=false;
f.submit=()=>{throw Error('author override must not be called')};target.click();
})()`)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-requests:
				want := "POST:value=dirty&events=true%2Ctrue%2Ctrue%2Ctrue%7Ctrue%2Ctrue%2Ctrue%2Ctrue&send=yes"
				if got != want {
					t.Fatalf("submit activation: %s want %s", got, want)
				}
			case <-ctx.Done():
				t.Fatal("submit-button activation did not reach server")
			}
			select {
			case got := <-requests:
				t.Fatalf("canceled activation submitted: %s", got)
			default:
			}
		})
	}
}
