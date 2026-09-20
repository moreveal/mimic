package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"testing"
)

func TestCanonicalIntrinsicImageUpdates(t *testing.T) {
	source, err := dom.Parse(`<html><body><img style="display:block"></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := FromSnapshot(source.DerivedSnapshot(), 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	id := uint64(source.FindAllByTagName("img")[0].ID)
	image := ImageIntrinsic{ID: id, Width: 240, Height: 120, Complete: true}
	if err := owner.ImageIntrinsic(image); err != nil {
		t.Fatal(err)
	}
	generation, err := owner.Resolve(0)
	if err != nil {
		t.Fatal(err)
	}
	rect, err := owner.Rect(id)
	if err != nil || rect.Width != 240 || rect.Height != 120 {
		t.Fatalf("loaded: %+v %v", rect, err)
	}
	if err := owner.ImageIntrinsic(image); err != nil {
		t.Fatal(err)
	}
	reused, err := owner.Resolve(0)
	if err != nil || reused != generation {
		t.Fatalf("unchanged resource rebuilt: %d %d %v", generation, reused, err)
	}
	image.Width, image.Height = 80, 40
	if err := owner.ImageIntrinsic(image); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	rect, err = owner.Rect(id)
	if err != nil || rect.Width != 80 || rect.Height != 40 {
		t.Fatalf("replacement: %+v %v", rect, err)
	}
	image.Width, image.Height = 0, 0
	if err := owner.ImageIntrinsic(image); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	rect, err = owner.Rect(id)
	if err != nil || rect.Width != 0 || rect.Height != 0 {
		t.Fatalf("failed resource retained size: %+v %v", rect, err)
	}
}
