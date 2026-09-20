package browser

import (
	"context"
	"testing"
	"time"
)

func TestPerformanceObserverDistinguishesZeroDurationFinalization(t *testing.T) {
	serialBrowserTest(t)
	p := testPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Keep timing at zero while independently exercising the load-state change.
	p.Top.Realm.navigationLoadEnd = time.Time{}
	_, err := p.Evaluate(ctx, `void(globalThis.zeroNavigationDone=new Promise(resolve=>new PerformanceObserver(list=>{const entries=list.getEntriesByType('navigation');resolve(entries.length===1&&entries[0].duration===0)}).observe({entryTypes:['navigation']})))`)
	if err != nil {
		t.Fatal(err)
	}
	p.Top.Realm.navigationLoadEnd = p.Top.Realm.performanceOrigin
	p.Top.Realm.notifyPerformanceObservers(ctx)
	value, err := p.Evaluate(ctx, `zeroNavigationDone`)
	if err != nil || value != true {
		t.Fatalf("zero-duration finalization: %v %v", value, err)
	}
}
