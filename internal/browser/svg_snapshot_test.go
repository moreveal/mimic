package browser

import (
	"context"
	"strings"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestSVGFragmentNamespacesAndSnapshot(t *testing.T) {
	parallelBrowserTest(t)
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
	value, err := p.Evaluate(context.Background(), `(()=>{
 const ns='http://www.w3.org/2000/svg',xl='http://www.w3.org/1999/xlink',svg=document.createElementNS(ns,'svg');
 svg.innerHTML='<symbol id="mark" viewBox="0 0 20 20"><path d="M0 0"/><path d="M1 1"/><linearGradient id="gradient"><stop offset="0"/><stop offset="1"/></linearGradient></symbol><foreignObject><div>HTML</div></foreignObject>';
 const symbol=svg.firstElementChild,use=document.createElementNS(ns,'use');use.setAttributeNS(xl,'xlink:href','#mark');svg.appendChild(use);document.body.appendChild(svg);
 const checks=[symbol.children.length===3,symbol.children[1].localName==='path',symbol.children[2].localName==='linearGradient',symbol.children[2].children.length===2,symbol.getAttribute('viewBox')==='0 0 20 20',symbol.getAttribute('viewbox')===null,svg.children[1].firstElementChild.namespaceURI==='http://www.w3.org/1999/xhtml',use.getAttributeNS(xl,'href')==='#mark',use.getAttribute('xlink:href')==='#mark'];
 const attr=use.getAttributeNodeNS(xl,'href');checks.push(attr.namespaceURI===xl,attr.prefix==='xlink',attr.localName==='href',use.cloneNode().getAttributeNS(xl,'href')==='#mark');
 use.setAttributeNS(xl,'other:href','#updated');checks.push(use.getAttributeNS(xl,'href')==='#updated',use.getAttributeNodeNS(xl,'href')===attr,attr.name==='xlink:href');use.removeAttributeNS(xl,'href');checks.push(!use.hasAttributeNS(xl,'href'),attr.value==='#updated',attr.ownerElement===null,attr.namespaceURI===xl);use.setAttributeNS(xl,'xlink:href','#mark');
 return checks.every(Boolean)?'ok':JSON.stringify(checks);
 })()`)
	if err != nil || value != "ok" {
		t.Fatalf("SVG DOM: %v %v", value, err)
	}
	snapshot, err := p.CaptureSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	output := string(snapshot.Files["index.html"])
	for _, want := range []string{`<path d="M0 0"></path><path d="M1 1"></path>`, `<linearGradient`, `viewBox="0 0 20 20"`, `<use xlink:href="#mark"></use>`, `<foreignObject><div>HTML</div></foreignObject>`} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %s in %s", want, output)
		}
	}
}
