package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// The empty HTMLDocument layer, constructor chain and illegal-constructor
// behavior were measured against headful Chrome 152.0.7977.82.
func TestHTMLDocumentInterfaceIdentityChrome152(t *testing.T) {
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
 const checks=[];
 for(const d of [document,document.implementation.createHTMLDocument(''),new DOMParser().parseFromString('<p>hello</p>','text/html')]) {
   checks.push(Object.getPrototypeOf(d)===HTMLDocument.prototype,d.constructor===HTMLDocument,Object.prototype.toString.call(d)==='[object HTMLDocument]',d instanceof Document,d.documentElement.ownerDocument===d);
 }
 for(const d of [document.implementation.createDocument(null,'root'),new DOMParser().parseFromString('<r/>','text/xml')]) {
   checks.push(Object.getPrototypeOf(d)===XMLDocument.prototype,d.constructor===XMLDocument,Object.prototype.toString.call(d)==='[object XMLDocument]',!(d instanceof HTMLDocument),d instanceof Document);
 }
 checks.push(Object.getPrototypeOf(HTMLDocument.prototype)===Document.prototype,Object.getPrototypeOf(HTMLDocument)===Document,Reflect.ownKeys(HTMLDocument.prototype).map(String).join(',')==='constructor,Symbol(Symbol.toStringTag)',String(HTMLDocument)==='function HTMLDocument() { [native code] }',HTMLDocument.length===0);
 try{new HTMLDocument();checks.push(false)}catch(e){checks.push(e.name==='TypeError',e.message==="Failed to construct 'HTMLDocument': Illegal constructor")}
 const frame=document.createElement('iframe');document.body.appendChild(frame);
 checks.push(frame.contentWindow.eval('Object.getPrototypeOf(document)===HTMLDocument.prototype && document.constructor===HTMLDocument && document.documentElement.ownerDocument===document'));
 return checks.every(Boolean)?'ok':JSON.stringify(checks);
 })()`)
	if err != nil || value != "ok" {
		t.Fatalf("HTMLDocument interface identity: %v %v", value, err)
	}
}
