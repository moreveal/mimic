package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// Chrome 152.0.7977.82: an inherited blank document sends same-origin Fetch
// Metadata and cookies, but no Referer. Changing <base> changes resolution,
// without changing the client's security origin or granting CORS access.
func TestBlankFrameFetchUsesInheritedSecurityOrigin(t *testing.T) {
	echo := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"cookie": r.Header.Get("Cookie"), "origin": r.Header.Get("Origin"), "referrer": r.Header.Get("Referer"), "site": r.Header.Get("Sec-Fetch-Site")})
	}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/denied" {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		}
		echo(w, r)
	}))
	defer remote.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/echo" {
			echo(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><body>blank frame</body>")
	}))
	defer local.Close()
	historyTestPages(t, func(t *testing.T, page *Page) {
		if err := page.Navigate(context.Background(), local.URL+"/entry"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, page, `(async()=>{
document.cookie='session=token;SameSite=Strict;path=/';
const f=document.createElement('iframe');document.body.append(f);
const read=async(options)=>JSON.parse(await f.contentWindow.fetch('/echo',options).then(r=>r.text()));
const same=await read();
if(same.cookie!=='session=token'||same.origin!==''||same.referrer!==''||same.site!=='same-origin')return 'same:'+JSON.stringify(same);
const post=await read({method:'POST'});
if(post.cookie!=='session=token'||post.origin!==location.origin||post.referrer!=='')return 'post:'+JSON.stringify(post);
const base=f.contentDocument.createElement('base');base.setAttribute('href',`+strconv.Quote(remote.URL+"/")+`);f.contentDocument.head.append(base);
const cross=await read();if(cross.origin!==location.origin||cross.cookie!=='')return 'cross:'+JSON.stringify(cross);
try{await f.contentWindow.fetch('/echo',{mode:'same-origin'});return 'base changed origin'}catch{}
try{await f.contentWindow.fetch('/denied');return 'CORS was bypassed'}catch{}
return true;
})()`, true)
	})
}
