package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// This independent fixture was measured against frozen Chrome 152. In
// particular, inert document descendants are connected despite no Window,
// and removal preserves ownership while insertion adopts without cloning.
func TestInertHTMLDocumentOwnershipChrome152(t *testing.T) {
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
 const d=document.implementation.createHTMLDocument('  A \n B '),empty=document.implementation.createHTMLDocument();
 d.body.innerHTML='<form></form><form></form><div id="local"><span>text</span></div>';
 const n=d.getElementById('local'),child=n.firstChild;
 const checks=[d!==document,d instanceof Document,n.ownerDocument===d,child.ownerDocument===d,d.documentElement.parentNode===d,n.getRootNode()===d,n.isConnected,d.defaultView===null,d.URL==='about:blank',d.title==='A B',d.compatMode==='CSS1Compat',d.doctype.nodeType===10,empty.head.childNodes.length===0,d.body.children.length===3,d.implementation===d.implementation];
 checks.push(document.implementation.createHTMLDocument(undefined).head.childNodes.length===0,d.contentType==='text/html',d.characterSet==='UTF-8',d.doctype instanceof DocumentType,d.doctype.name==='html',d.textContent===null);
 const tags=d.body.getElementsByTagName('form');checks.push(tags.length===2,tags[2]===undefined,tags.item(2)===null);d.body.appendChild(d.createElement('form'));checks.push(tags.length===3);
 n.remove();checks.push(n.ownerDocument===d,!n.isConnected);
 const copy=n.cloneNode(true);checks.push(copy.ownerDocument===d,copy.firstChild.ownerDocument===d);
 document.body.appendChild(n);checks.push(n.ownerDocument===document,child.ownerDocument===document,n.firstChild===child);
 const imported=d.importNode(n,true);checks.push(imported!==n,imported.ownerDocument===d,imported.firstChild.ownerDocument===d);
 d.body.appendChild(imported);imported.textContent='replacement';checks.push(imported.firstChild.ownerDocument===d);
 let hierarchy='';try{d.appendChild(d.createElement('div'))}catch(e){hierarchy=e.name}checks.push(hierarchy==='HierarchyRequestError');
 return checks.every(Boolean)?'ok':JSON.stringify(checks);
})()`)
	if err != nil || value != "ok" {
		t.Fatalf("inert document semantics: %v %v", value, err)
	}
}
