package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
	"time"
)

func TestTokenToggleUsesCanonicalAttributes(t *testing.T) {
	serialBrowserTest(t)
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
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got, err := p.Evaluate(ctx, `(()=>{globalThis.tokenElement=document.createElement('div');tokenElement.id='token-test';document.body.appendChild(tokenElement);globalThis.tokens=tokenElement.classList;tokenElement.className='a a b';return tokens.toggle('a',false)===false&&tokenElement.className==='b'&&tokens.toggle('c',true)===true&&tokens.toggle('c',true)===true&&tokenElement.className==='b c'&&tokens.toggle('c')===false&&tokenElement.className==='b'})()`)
			if err != nil || got != true {
				t.Fatalf("toggle: %v %v", got, err)
			}
			document, _ := p.Document()
			node, ok := document.Find("#token-test")
			if !ok {
				t.Fatal("canonical node missing")
			}
			if err := document.SetAttribute(node.ID, "class", "\ufeffexternal\u00a0value external"); err != nil {
				t.Fatal(err)
			}
			got, err = p.Evaluate(ctx, `tokens.toggle('external',false)===false&&tokenElement.getAttribute('class')==='value'&&tokens.toggle('absent',false)===false&&tokenElement.getAttribute('class')==='value'`)
			if err != nil || got != true {
				t.Fatalf("external mutation: %v %v", got, err)
			}
		})
	}
}
