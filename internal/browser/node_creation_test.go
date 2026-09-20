package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
)

func TestFreshNodeRecordsPreserveCanonicalUnicode(t *testing.T) {
	parallelBrowserTest(t)
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}} {
		t.Run(name, func(t *testing.T) {
			b, err := New(factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			c := b.NewContext()
			defer c.Close()
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			value, err := p.Evaluate(context.Background(), `(()=>{
    const node=document.createElement('MiXeD'),unicode=document.createElement('\u00df');
    if(node.tagName!=='MIXED'||node.namespaceURI!=='http://www.w3.org/1999/xhtml'||unicode.tagName!=='\u00df')return false;
    for(const text of ['plain','\u00e9','\ud83d\ude42','\ud800']){
      const t=document.createTextNode(text),comment=document.createComment(text),box=document.createElement('div');
      box.appendChild(t);if(box.textContent!==t.data||t.nodeName!=='#text')return false;
      if(comment.data!==t.data||comment.nodeName!=='#comment')return false;
    }
    return true;
   })()`)
			if err != nil || value != true {
				t.Fatalf("fresh records: %v %v", value, err)
			}
		})
	}
}
