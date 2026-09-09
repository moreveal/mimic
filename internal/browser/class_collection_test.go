package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassCollectionsAreLiveAndScoped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><body><div id="root" class="a b"><i id="one" class="a b"></i><i id="two" class="a"></i></div><i id="outside" class="a b"></i></body>`)
	}))
	defer server.Close()
	p := testPage(t)
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(context.Background(), `(()=>{
const root=document.getElementById('root'),list=root.getElementsByClassName(' a\t b a '),ids=collection=>Array.from(collection,x=>x.id).join(',');
const initial=ids(list),all=ids(document.getElementsByClassName('a b'));
document.getElementById('two').className='b a';const changed=ids(list);
root.removeChild(document.getElementById('one'));const removed=ids(list);
const extra=document.createElement('i');extra.id='added';extra.className='a b';root.appendChild(extra);
return {tag:Object.prototype.toString.call(list),initial,all,changed,removed,added:ids(list),empty:root.getElementsByClassName(' \t ').length,named:list.namedItem('added')===extra,filter:Array.prototype.filter.call(list,x=>x.id==='added').length};
})()`)
	if err != nil {
		t.Fatal(err)
	}
	m := value.(map[string]any)
	for key, expected := range map[string]any{"tag": "[object HTMLCollection]", "initial": "one", "all": "root,one,outside", "changed": "one,two", "removed": "two", "added": "two,added", "empty": int64(0), "named": true, "filter": int64(1)} {
		if m[key] != expected {
			t.Fatalf("%s: got %#v, want %#v (result %#v)", key, m[key], expected, m)
		}
	}
}

func TestHTMLCollectionMissingIndices(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{const root=document.createElement('div');root.innerHTML='<i id="entry"></i>';const list=root.children;return list[0]===root.firstChild&&list[1]===undefined&&!(1 in list)&&0 in list&&list.item(1)===null&&list.namedItem('missing')===null&&list.namedItem('entry')===root.firstChild&&list.entry===root.firstChild})()`)
	if err != nil || value != true {
		t.Fatalf("collection indices: %v %v", value, err)
	}
}

func TestClassCollectionQuirksCaseMatching(t *testing.T) {
	for _, doctype := range []string{"", "<!doctype html>"} {
		t.Run(fmt.Sprint(doctype != ""), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, doctype+`<div class="UPPER"></div>`) }))
			defer server.Close()
			p := testPage(t)
			if err := p.Navigate(context.Background(), server.URL); err != nil {
				t.Fatal(err)
			}
			value, err := p.Evaluate(context.Background(), `document.getElementsByClassName('upper').length===1`)
			if err != nil || value != (doctype == "") {
				t.Fatalf("quirks class matching: %v %v", value, err)
			}
		})
	}
}
