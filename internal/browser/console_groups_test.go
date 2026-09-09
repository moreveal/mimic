package browser

import (
	"context"
	"fmt"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

func TestConsoleGroupsEmitOrderedEvents(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 const methods=[console.group,console.groupCollapsed,console.groupEnd];
 const returns=[console.group('outer'),console.groupCollapsed('inner'),console.log('inside'),console.groupEnd('ignored'),console.groupEnd(),console.groupEnd(),console.group(),console.groupCollapsed()];
 return returns.every(v=>v===undefined)&&methods.every(f=>f.length===0)&&methods.map(f=>f.name).join(',')==='group,groupCollapsed,groupEnd';
})()`)
	if err != nil || value != true {
		t.Fatalf("console groups: %v, %v", value, err)
	}
	var got []string
	for _, event := range p.Trace().Events() {
		if event.Kind == trace.Console {
			got = append(got, event.Name+":"+fmt.Sprint(event.Data["args"]))
		}
	}
	want := "[startGroup:[outer] startGroupCollapsed:[inner] log:[inside] endGroup:[ignored] endGroup:[console.groupEnd] endGroup:[console.groupEnd] startGroup:[console.group] startGroupCollapsed:[console.groupCollapsed]]"
	if fmt.Sprint(got) != want {
		t.Fatalf("console event sequence = %v, want %s", got, want)
	}
}
