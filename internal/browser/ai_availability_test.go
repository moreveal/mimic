//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestAIAvailabilityMatchesFrozenChromePolicyDenied(t *testing.T) {
	documentAllOracle(t, "ai_availability", map[string]string{
		"Permissions-Policy": "summarizer=(), language-model=(), translator=(), language-detector=()",
	})
}

func TestAIUnavailableBackendAndFramePolicy(t *testing.T) {
	p := bootstrapSnapshotPage(t)
	const child = `(async()=>{const out=[];for(const type of ['Summarizer','LanguageModel','Translator','LanguageDetector']){const options=type==='Translator'?{sourceLanguage:'en',targetLanguage:'fr'}:{};out.push(await globalThis[type].availability(options));try{await globalThis[type].create(options);out.push('created')}catch(e){out.push(e.name)}}parent.postMessage({aiResult:out},'*')})()`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><body>")
		if r.URL.Path == "/child" {
			fmt.Fprint(w, "<script>"+child+"</script>")
		}
	}))
	defer server.Close()
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, origin, allow, want string }{
		{"same-origin-no-backend", server.URL, "", "NotSupportedError"},
		{"cross-origin-default-policy", strings.Replace(server.URL, "127.0.0.1", "localhost", 1), "", "NotAllowedError"},
		{"same-origin-container-denied", server.URL, "summarizer 'none'; language-model 'none'; translator 'none'; language-detector 'none'", "NotAllowedError"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `new Promise(resolve=>{const frame=document.createElement('iframe'),receive=e=>{if(e.source!==frame.contentWindow||!e.data.aiResult)return;removeEventListener('message',receive);frame.remove();resolve(e.data.aiResult.every((v,i)=>v===(i%2?` + strconv.Quote(tc.want) + `:'unavailable')))};addEventListener('message',receive);frame.allow=` + strconv.Quote(tc.allow) + `;frame.src=` + strconv.Quote(tc.origin+"/child") + `;document.body.append(frame)})`
			if value := bootstrapSnapshotEvaluate(t, p, source); value != true {
				t.Fatalf("AI frame policy: %#v", value)
			}
		})
	}
}
