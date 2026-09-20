package browser

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

func TestConsoleCountLabelsErrorsAndRealmIsolation(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		result, err := p.Evaluate(context.Background(), `(()=>{
 if(console.count.length!==0||console.countReset.length!==0)return 'length';
 if(console.count()!==undefined||console.count(undefined)!==undefined)return 'return';
 console.countReset();console.count();console.countReset('missing');
 const count=console.count;count.call(null,'borrowed');count.call({},'borrowed');
 let reads=0;const label={toString(){reads++;return 'label'}};console.count(label);console.countReset(label);console.count(label);if(reads!==3)return 'conversion';
 let failure={},thrown=false;try{console.count({toString(){throw failure}})}catch(e){thrown=e===failure}if(!thrown)return 'error identity';
 thrown=false;try{console.countReset(Symbol())}catch(e){thrown=e.name==='TypeError'}if(!thrown)return 'symbol';
 console.count();
 const frame=document.createElement('iframe');document.body.appendChild(frame);frame.contentWindow.console.count('label');console.count('label');
 console.countReset('label');console.countReset('label');return true;
 })()`)
		if err != nil || result != true {
			t.Fatalf("count: %v %v", result, err)
		}
		var rows []string
		for _, event := range p.Trace().Events() {
			if event.Kind == trace.Console {
				rows = append(rows, event.Name+":"+fmt.Sprint(event.Data["args"]))
			}
		}
		want := "count:[default: 1]|count:[default: 2]|count:[default: 1]|warning:[Count for 'missing' does not exist]|count:[borrowed: 1]|count:[borrowed: 2]|count:[label: 1]|count:[label: 1]|count:[default: 2]|count:[default: 1]|count:[label: 1]|count:[label: 2]|warning:[Count for 'label' does not exist]"
		if got := strings.Join(rows, "|"); got != want {
			t.Fatalf("trace:\n%s\nwant:\n%s", got, want)
		}
	})
}
