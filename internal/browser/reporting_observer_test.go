package browser

import (
	"context"
	"testing"
)

func TestReportingObserverEmptyBackendLifecycle(t *testing.T) {
	p := testPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `(()=>{const observer=new ReportingObserver(()=>{}, {types:['deprecation'],buffered:true});observer.observe();const first=observer.takeRecords();observer.disconnect();return observer instanceof ReportingObserver&&first.length===0&&observer.takeRecords().length===0})()`)
	if err != nil {
		t.Fatal(err)
	}
	if value != true {
		t.Fatalf("unexpected observer result: %#v", value)
	}
}
