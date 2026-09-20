package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestCatalogDataCreatesIndependentRealmObjects(t *testing.T) {
	parallelBrowserTest(t)
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}} {
		t.Run(name, func(t *testing.T) {
			bundle := surfaceTestBundle{Bundle: chrome152.New(), surface: compatibility.WebAPISurface{
				GeneratedJavaScript:  `globalThis.__catalog=JSON.parse(__mimic.catalogJSON());`,
				GeneratedCatalogJSON: `{"value":"original","nested":{"n":1}}`,
			}}
			b, err := New(factory, bundle)
			if err != nil {
				t.Fatal(err)
			}
			c := b.NewContext()
			defer c.Close()
			first, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := first.Evaluate(context.Background(), `__catalog.value='changed';__catalog.nested.n=2`); err != nil {
				t.Fatal(err)
			}
			second, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			value, err := second.Evaluate(context.Background(), `__catalog.value==='original'&&__catalog.nested.n===1`)
			if err != nil || value != true {
				t.Fatalf("catalog state crossed Pages: %v %v", value, err)
			}
		})
	}
}
