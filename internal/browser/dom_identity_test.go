package browser

import (
	"context"
	"testing"
)

func TestQueryMembershipPreservesIdentityAndCanonicalAttributes(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	ctx := context.Background()
	value, err := p.Evaluate(ctx, `(()=>{
 document.body.innerHTML='<div data-v="old"></div><div></div>';
 globalThis.members=document.querySelectorAll('div');
 globalThis.first=members[0];
 first.setAttribute('data-v','new');
 first.remove();
 return members.length===2&&members[0]===first&&members[0].getAttribute('data-v')==='new'&&document.querySelectorAll('div').length===1;
})()`)
	if err != nil || value != true {
		t.Fatalf("membership and identity: %v %v", value, err)
	}
	document, ok := p.Document()
	if !ok {
		t.Fatal("missing document")
	}
	node, ok := document.Find("[data-v]")
	if !ok {
		t.Fatal("missing detached canonical node")
	}
	if err := document.SetAttribute(node.ID, "data-external", "updated"); err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(ctx, `first.getAttribute('data-external')==='updated'&&members[0].getAttributeNames().includes('data-external')&&first.attributes.getNamedItem('data-external').value==='updated'`)
	if err != nil || value != true {
		t.Fatalf("canonical attributes: %v %v", value, err)
	}
}
