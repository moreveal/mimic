package browser

import (
	"context"
	"testing"
	"time"
)

func TestNavigatedDocumentPreservesNodeOwnershipIdentity(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		result, err := p.Evaluate(ctx, `(async()=>{
 const frame=document.createElement('iframe');document.body.appendChild(frame);
 const initial=frame.contentDocument;
 await new Promise(resolve=>{frame.onload=resolve;frame.src='about:blank?next'});
 const next=frame.contentDocument,child=next.createElement('section');
 next.body.appendChild(child);
 const local=document.createElement('b');next.body.appendChild(local);
 const checks=[next!==document,next!==initial,child.ownerDocument===next,local.ownerDocument===next,child.parentNode===next.body,initial.body.ownerDocument===initial];
 document.body.appendChild(child);checks.push(child.ownerDocument===document);
 const imported=document.importNode(local,true);checks.push(imported.ownerDocument===document,imported!==local);
 frame.remove();return checks.every(Boolean);
})()`)
		if err != nil || result != true {
			t.Fatalf("navigated document/node identity: %v, %v", result, err)
		}
	})
}
