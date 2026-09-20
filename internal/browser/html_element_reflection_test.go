package browser

import (
	"context"
	"testing"
)

func TestHTMLElementHiddenAndAriaLabelReflectAttributes(t *testing.T) {
	parallelBrowserTest(t)
	page := testPage(t)
	value, err := page.Evaluate(context.Background(), `(()=>{
		const element=document.createElement('div');
		element.hidden=true;element.ariaLabel='Preview';
		const set=[element.hidden,element.hasAttribute('hidden'),element.getAttribute('aria-label')];
		element.hidden=false;element.ariaLabel=null;
		return JSON.stringify([set,element.hidden,element.hasAttribute('hidden'),element.hasAttribute('aria-label')]);
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	if value != `[[true,true,"Preview"],false,false,false]` {
		t.Fatalf("unexpected reflected state: %v", value)
	}
}
