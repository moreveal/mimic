package browser

import (
	"context"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFragmentInsertionOwnerDocumentAndCommentText(t *testing.T) {
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
			got, err := p.Evaluate(context.Background(), `(()=>{
    const parent=document.createElement('div'),tail=document.createElement('b');parent.appendChild(tail);
    const f=document.createDocumentFragment(),a=document.createElement('span'),comment=document.createComment('hidden');
    a.textContent='a';f.appendChild(a);f.appendChild(comment);
    const returned=parent.insertBefore(f,tail);
    const moved=f.childNodes.length===0&&a.parentNode===parent&&comment.parentNode===parent;
    const f2=document.createDocumentFragment();f2.appendChild(a);parent.appendChild(f2);
    let cycle=false;try{a.appendChild(parent)}catch(e){cycle=e.name==='HierarchyRequestError'}
    return JSON.stringify({returned:returned===f,moved,order:Array.from(parent.childNodes,n=>n.nodeName),text:parent.textContent,comment:comment.textContent,owner:a.ownerDocument===document&&comment.ownerDocument===document&&f.ownerDocument===document&&document.ownerDocument===null,cycle});
   })()`)
			want := `{"returned":true,"moved":true,"order":["#comment","B","SPAN"],"text":"a","comment":"hidden","owner":true,"cycle":true}`
			if err != nil || got != want {
				t.Fatalf("got %v, %v; want %s", got, err, want)
			}
		})
	}
}

func TestV8SameWindowPostedMessageAndNativeWasm(t *testing.T) {
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := p.Evaluate(ctx, `new Promise(resolve=>{addEventListener('message',function listener(e){removeEventListener('message',listener);resolve(e.data===42&&e.source===window)});postMessage(42,'*')})`)
	if err != nil || got != true {
		t.Fatalf("same-window message: %v, %v", got, err)
	}
	// No timers and no repeated CDP evaluations to accidentally service V8 work.
	got, err = p.Evaluate(ctx, `Promise.resolve().then(()=>WebAssembly.instantiate(new Uint8Array([0,97,115,109,1,0,0,0,1,7,1,96,2,127,127,1,127,3,2,1,0,7,7,1,3,97,100,100,0,0,10,9,1,7,0,32,0,32,1,106,11]))).then(m=>m.instance.exports.add(19,23))`)
	if err != nil || got != float64(42) {
		t.Fatalf("native async compilation: %v, %v", got, err)
	}
}
