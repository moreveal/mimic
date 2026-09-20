package browser

import (
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/imageresource"
	"testing"
)

func TestBlitzCanonicalImageInputLifecycle(t *testing.T) {
	document, err := dom.Parse(`<html><body><img></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	id := document.FindAllByTagName("img")[0].ID
	realm := &Realm{document: document, imageLoads: map[int64]*imageLoad{id: {decoded: &imageresource.Image{Width: 80, Height: 40}, complete: false}}}
	inputs := realm.blitzImageInputs()
	if len(inputs) != 1 || inputs[0].Width != 80 || inputs[0].Height != 40 || inputs[0].Complete {
		t.Fatalf("pending replacement must retain previous image: %+v", inputs)
	}
	realm.imageLoads[id].decoded = nil
	realm.imageLoads[id].complete = true
	inputs = realm.blitzImageInputs()
	if inputs[0].Width != 0 || inputs[0].Height != 0 || !inputs[0].Complete {
		t.Fatalf("failed completion retained dimensions: %+v", inputs)
	}
}
