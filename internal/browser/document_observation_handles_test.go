package browser

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDocumentObservationsDoNotRetainInvocationValues(t *testing.T) {
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := p.Top.Realm
	element, ok := r.document.FirstElementChild(r.document.Root().ID)
	if !ok {
		t.Fatal("fixture root element unavailable")
	}
	host := map[string]any{}
	r.installDocumentCompatibility(host)
	if err := r.runtime.Set("observationProbe", host); err != nil {
		t.Fatal(err)
	}
	diagnostic, ok := r.runtime.(interface{ Diagnostics() (any, error) })
	if !ok {
		t.Fatal("V8 diagnostics unavailable")
	}
	count := func() int {
		t.Helper()
		value, err := diagnostic.Diagnostics()
		if err != nil {
			t.Fatal(err)
		}
		return value.(map[string]any)["persistent_handles"].(int)
	}
	// Exercise both the exact packed signature and existing nonnumeric fallback.
	// Private node IDs are numbers; the fallback preserves its zero-ID result.
	// These private hosts return values; they never capture a callback or receiver.
	source := fmt.Sprintf(`(()=>{let valid=true;const width=innerWidth;for(let i=0;i<1000;i++){
	const id=i%%2 ? %d : String(%d);
	valid=valid&&observationProbe.nodeOwnerDocument(id)===(i%%2 ? %d : 0);
	valid=valid&&observationProbe.computedStyleAvailable(id)===(i%%2!==0);
	valid=valid&&observationProbe.foreignComputedStyleFlatTree(id,'value','color')===(i%%2 ? null : '');
	valid=valid&&observationProbe.stylesheetResource('https://example.invalid/missing.css','')===undefined;
	valid=valid&&innerWidth===width;
	}return valid})()`, element.ID, element.ID, r.document.Root().ID)
	for iteration := 0; iteration < 2; iteration++ {
		before := count()
		value, err := p.Evaluate(ctx, source)
		if err != nil || value != true {
			t.Fatalf("observation values: %v %v", value, err)
		}
		if growth := count() - before; growth > 8 {
			t.Fatalf("observation loop retained %d invocation handles", growth)
		}
	}
}

func TestDOMMutationsDoNotRetainInvocationValues(t *testing.T) {
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	diagnostic := p.Top.Realm.runtime.(interface{ Diagnostics() (any, error) })
	count := func() int {
		t.Helper()
		value, err := diagnostic.Diagnostics()
		if err != nil {
			t.Fatal(err)
		}
		return value.(map[string]any)["persistent_handles"].(int)
	}
	const source = `(()=>{const root=document.body,fragment=document.createDocumentFragment();
 root.innerHTML='';
 for(let i=0;i<16;i++)document.getElementById('missing');
 for(let i=0;i<256;i++){const node=document.createElement('div');node.setAttribute('data-index',String(i));node.classList.toggle('item',true);node.appendChild(document.createTextNode('text'));node.appendChild(document.createComment('comment'));fragment.appendChild(node)}
 const extra=document.createElement('section');extra.innerHTML='<b>end</b><!--tail-->text';fragment.appendChild(extra);
 if(fragment.querySelectorAll('.item').length!==256||fragment.querySelector('[data-index="255"]').textContent!=='text')throw new Error('DOM observation mismatch');
 extra.firstChild.textContent='changed';extra.insertAdjacentHTML('beforeend','<i>last</i>');extra.lastChild.outerHTML='<em>last</em>';
 if(!extra.innerHTML.includes('changed')||!extra.outerHTML.includes('<em>last</em>'))throw new Error('markup observation mismatch');
 root.appendChild(fragment);while(root.lastChild)root.removeChild(root.lastChild);
 return fragment.childNodes.length===0&&root.childNodes.length===0})()`
	// Warm API-access observation and wrapper installation before checking that
	// successive turns release their arguments rather than retain them per call.
	for iteration := 0; iteration < 4; iteration++ {
		before := count()
		value, err := p.Evaluate(ctx, source)
		if err != nil || value != true {
			t.Fatalf("mutation result: %v %v", value, err)
		}
		if growth := count() - before; iteration != 0 && growth != 0 {
			t.Fatalf("mutation loop retained %d invocation handles", growth)
		}
	}
}
