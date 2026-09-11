package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"strings"
	"testing"
)

func TestSnapshotPreservesCanonicalShadowComposition(t *testing.T) {
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
	ctx := context.Background()
	got, err := p.Evaluate(ctx, `(()=>{
 const host=document.createElement('div');host.id='shadow-host';host.innerHTML='<span slot="name">light &amp; safe</span>';document.body.append(host);
 const root=host.attachShadow({mode:'open',delegatesFocus:true});root.innerHTML='<strong>two weeks ago</strong><slot name="name"></slot><div id="nested"></div><script>throw Error("must not replay")</script>';
 const nested=root.querySelector('#nested');nested.attachShadow({mode:'closed'}).append(document.createTextNode('closed & text'));
 const strong=root.querySelector('strong');strong.textContent='updated < &';
 return root.innerHTML.includes('updated &lt; &amp;')&&host.querySelector('strong')===null&&host.shadowRoot===root;
 })()`)
	if err != nil || got != true {
		t.Fatalf("live shadow: %v %v", got, err)
	}
	snapshot, err := p.CaptureSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	output := string(snapshot.Files["index.html"])
	for _, want := range []string{`shadowrootmode="open"`, `shadowrootdelegatesfocus`, `shadowrootmode="closed"`, `updated &lt; &amp;`, `closed &amp; text`, `slot="name"`, `<slot name="name">`} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in %s", want, output)
		}
	}
	if strings.Contains(output, "must not replay") {
		t.Error("export retained shadow script")
	}
	if strings.Count(strings.ToLower(output), "<!doctype") != 1 {
		t.Error("export has duplicate doctype")
	}
	got, err = p.Evaluate(ctx, `document.querySelector('#shadow-host').shadowRoot.querySelector('strong').textContent`)
	if err != nil || got != "updated < &" {
		t.Fatalf("snapshot mutated live state: %v %v", got, err)
	}
}

func TestShadowInnerHTMLUsesHostParserContext(t *testing.T) {
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
	got, err := p.Evaluate(context.Background(), `(()=>{
let constructions=0;customElements.define('x-parser-context',class extends HTMLElement {constructor(){super();constructions++}});
const host=document.createElement('x-parser-context'),root=host.attachShadow({mode:'open'});
root.innerHTML='<tr><td>x</td></tr>';return root.innerHTML==='x'&&constructions===1;
})()`)
	if err != nil || got != true {
		t.Fatalf("shadow parsing context: %v %v", got, err)
	}
}

func TestSnapshotDoesNotPromotePolyfilledShadowRoot(t *testing.T) {
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
	ctx := context.Background()
	got, err := p.Evaluate(ctx, `(()=>{const host=document.createElement('section'),root=document.createDocumentFragment();host.id='polyfill-host';document.body.append(host);const child=document.createElement('strong');child.textContent='polyfilled content';root.appendChild(child);Object.setPrototypeOf(root,ShadowRoot.prototype);Object.defineProperties(root,{host:{value:host},mode:{value:'open'}});globalThis.ShadyDOM={inUse:true,wrap(node){return {shadowRoot:node===host?root:null}}};return root instanceof ShadowRoot})()`)
	if err != nil || got != true {
		t.Fatalf("polyfilled root setup: %v %v", got, err)
	}
	snapshot, err := p.CaptureSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	output := string(snapshot.Files["index.html"])
	if !strings.Contains(output, `id="polyfill-host"`) {
		t.Fatal("snapshot lost canonical host")
	}
	if strings.Contains(output, "shadowrootmode") || strings.Contains(output, "polyfilled content") {
		t.Fatal("snapshot invented a native shadow attachment from an author wrapper")
	}
	got, err = p.Evaluate(ctx, `document.querySelector('#polyfill-host').shadowRoot === null`)
	if err != nil || got != true {
		t.Fatalf("snapshot mutated attachment state: %v %v", got, err)
	}
}
