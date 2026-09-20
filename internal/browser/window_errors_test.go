package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/trace"
)

// Measured with headful Chrome 152.0.7977.82. Window's error handler receives
// five arguments; true (or preventDefault) suppresses the uncaught exception.
func TestWindowTimerErrorReportingAndCancellation(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<!doctype html><body>timer errors</body>")
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, page *Page) {
		if err := page.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"false", "true", "prevent", "nested"} {
			t.Run(mode, func(t *testing.T) {
				page.Trace().Clear()
				handler := "return false"
				prevent := ""
				if mode == "true" {
					handler = "return true"
				} else if mode == "prevent" {
					prevent = "event.preventDefault()"
				} else if mode == "nested" {
					handler = "throw new RangeError('nested-probe')"
				}
				script := `new Promise(resolve=>{
					const rows=[], original=new TypeError('timer-probe');
					window.onerror=function(message,filename,line,column,error){
						rows.push(['handler',arguments.length,message,this===window,error===original,window.event instanceof ErrorEvent]);HANDLER;
					};
					const listener=event=>{rows.push(['listener',event.message,event.error===original,event.isTrusted,event.cancelable,event.bubbles,event.defaultPrevented,window.event===event]);PREVENT};
					addEventListener('error',listener);
					setTimeout(()=>{throw original},0);
					setTimeout(()=>{removeEventListener('error',listener);window.onerror=null;resolve(JSON.stringify(rows))},15);
				})`
				script = strings.ReplaceAll(strings.ReplaceAll(script, "HANDLER", handler), "PREVENT", prevent)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				value, err := page.Evaluate(ctx, script)
				if err != nil {
					t.Fatal(err)
				}
				want := `[["handler",5,"Uncaught TypeError: timer-probe",true,true,true],["listener","Uncaught TypeError: timer-probe",true,true,true,false,DEFAULT,true]]`
				want = strings.ReplaceAll(want, "DEFAULT", map[bool]string{true: "true", false: "false"}[mode == "true"])
				if value != want {
					t.Fatalf("error dispatch=%s, want %s", value, want)
				}
				var uncaught []string
				for _, event := range page.Trace().Events() {
					if event.Kind == trace.Exception {
						uncaught = append(uncaught, event.Data["error"].(string))
					}
				}
				wantExceptions := "null"
				if mode == "false" {
					wantExceptions = `["Uncaught TypeError: timer-probe"]`
				} else if mode == "nested" {
					wantExceptions = `["Uncaught TypeError: timer-probe","Uncaught RangeError: nested-probe"]`
				}
				encoded, _ := json.Marshal(uncaught)
				if string(encoded) != wantExceptions {
					t.Fatalf("uncaught=%s want %s", encoded, wantExceptions)
				}
			})
		}
	})
}

func TestWindowIntervalContinuesAfterErrorAndKeepsCancelableID(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<!doctype html><body>interval errors</body>")
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, page *Page) {
		if err := page.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		page.Trace().Clear()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		value, err := page.Evaluate(ctx, `new Promise(resolve=>{
			let ticks=0,other=false;window.onerror=()=>true;
			const id=setInterval(()=>{
				ticks++;if(ticks===1)throw new Error('interval-probe');
				clearInterval(id);
				setTimeout(()=>{window.onerror=null;resolve(JSON.stringify({ticks,other}))},20);
			},5);
			setTimeout(()=>other=true,0);
		})`)
		if err != nil || value != `{"ticks":2,"other":true}` {
			t.Fatalf("interval=%v error=%v", value, err)
		}
		if len(page.Top.Realm.timers) != 0 {
			t.Fatalf("retained timer registrations: %d", len(page.Top.Realm.timers))
		}
		for _, event := range page.Trace().Events() {
			if event.Kind == trace.Exception || event.Kind == trace.Error {
				t.Fatalf("handled interval error leaked: %+v", event)
			}
		}
	})
}
