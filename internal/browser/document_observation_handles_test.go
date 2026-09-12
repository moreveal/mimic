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
