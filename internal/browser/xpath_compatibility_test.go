package browser

import (
	"context"
	"reflect"
	"testing"
)

func TestXPathAutomationResultsUseCanonicalNodes(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 document.body.innerHTML='<main><button id="login"> Log   in </button><button data-action="login">Other</button></main>';
 const first=document.evaluate('//button[normalize-space(.)="Log in"]',document,null,XPathResult.FIRST_ORDERED_NODE_TYPE,null);
 const snapshot=document.evaluate('//*[@data-action="login"]',document,null,XPathResult.ORDERED_NODE_SNAPSHOT_TYPE,null);
 const iterator=document.evaluate('//button[contains(., "Log")]',document,null,XPathResult.ORDERED_NODE_ITERATOR_TYPE,null);
 const iterFirst=iterator.iterateNext(),iterSecond=iterator.iterateNext();
 return [first instanceof XPathResult,first.resultType,first.singleNodeValue===document.getElementById('login'),snapshot.snapshotLength,snapshot.snapshotItem(0)===document.querySelector('[data-action="login"]'),iterFirst===document.getElementById('login'),iterSecond===null];
})()`)
	want := []any{true, int64(9), true, int64(1), true, true, true}
	if err != nil || !reflect.DeepEqual(value, want) {
		t.Fatalf("xpath automation semantics: %v %v", value, err)
	}
}

func TestXPathMissingNodeReturnsResultInsteadOfUndefined(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{const r=document.evaluate('//button[@id="missing"]',document,null,XPathResult.FIRST_ORDERED_NODE_TYPE,null);return r.singleNodeValue===null})()`)
	if err != nil || value != true {
		t.Fatalf("xpath missing result: %v %v", value, err)
	}
}
