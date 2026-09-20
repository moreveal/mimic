package dom

import "testing"

func TestHTMLParsingKeepsFirstDuplicateAttribute(t *testing.T) {
	d, err := Parse(`<main><img id="first" id="second" class="one" class="two"></main>`)
	if err != nil {
		t.Fatal(err)
	}
	images := d.FindAllByTagName("img")
	if len(images) != 1 {
		t.Fatalf("image count = %d, want 1", len(images))
	}
	image := images[0]
	if got := image.Attributes["id"]; got != "first" {
		t.Fatalf("id = %q, want first", got)
	}
	if got := image.Attributes["class"]; got != "one" {
		t.Fatalf("class = %q, want one", got)
	}
	if len(image.AttributeNames) != 2 || image.AttributeNames[0] != "id" || image.AttributeNames[1] != "class" {
		t.Fatalf("attribute names = %#v, want [id class]", image.AttributeNames)
	}
}

func TestFragmentParsingKeepsFirstDuplicateAttribute(t *testing.T) {
	d, err := Parse(`<main></main>`)
	if err != nil {
		t.Fatal(err)
	}
	main := d.FindAllByTagName("main")[0]
	if err := d.SetInnerHTML(main.ID, `<img id="first" id="second">`); err != nil {
		t.Fatal(err)
	}
	images := d.FindAllByTagName("img")
	if len(images) != 1 || images[0].Attributes["id"] != "first" {
		t.Fatalf("images = %#v, want one image with first id", images)
	}
}
