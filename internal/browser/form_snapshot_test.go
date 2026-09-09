package browser

import (
	"context"
	"github.com/moreveal/mimic/internal/dom"
	"strings"
	"testing"
)

func TestSnapshotProjectsCurrentFormStateWithoutMutatingDOM(t *testing.T) {
	p := testPage(t)
	defer p.Close()
	ctx := context.Background()
	_, err := p.Evaluate(ctx, `document.body.innerHTML='<input id="first" type="radio" name="group" checked><input id="second" type="radio" name="group"><input id="text" value="default"><textarea id="area">default</textarea><select id="select"><option id="a" value="a" selected>A</option><option id="b" value="b">B</option></select><div id="host"></div>';
 document.getElementById('second').checked=true;document.getElementById('text').value='current & value';document.getElementById('area').value='\ncurrent\ntext';document.getElementById('select').value='b';
 const shadow=document.getElementById('host').attachShadow({mode:'open'}),input=document.createElement('input');input.type='checkbox';input.id='shadow-check';input.checked=true;shadow.appendChild(input);
 globalThis.formBefore=document.body.innerHTML;globalThis.formMutations=0;new MutationObserver(records=>formMutations+=records.length).observe(document.body,{subtree:true,childList:true,attributes:true,characterData:true});`)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.CaptureSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	live, err := p.Evaluate(ctx, `document.body.innerHTML===formBefore&&formMutations===0&&!document.getElementById('first').checked&&document.getElementById('second').checked&&document.getElementById('text').value==='current & value'&&document.getElementById('area').value==='\ncurrent\ntext'&&document.getElementById('select').value==='b'`)
	if err != nil || live != true {
		t.Fatalf("snapshot mutated live state: %v %v", live, err)
	}
	source := string(snapshot.Files["index.html"])
	projection, err := dom.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	attribute := func(selector, name string) (string, bool) {
		t.Helper()
		node, ok := projection.Find(selector)
		if !ok {
			t.Fatalf("missing %s", selector)
		}
		return projection.GetAttribute(node.ID, name)
	}
	if _, ok := attribute("#first", "checked"); ok {
		t.Fatal("default radio remains checked")
	}
	if _, ok := attribute("#second", "checked"); !ok {
		t.Fatal("current radio was lost")
	}
	if value, _ := attribute("#text", "value"); value != "current & value" {
		t.Fatalf("input value %q", value)
	}
	area, _ := projection.Find("#area")
	if value := projection.TextContent(area.ID); value != "\ncurrent\ntext" {
		t.Fatalf("textarea value %q", value)
	}
	if _, ok := attribute("#a", "selected"); ok {
		t.Fatal("default option remains selected")
	}
	if _, ok := attribute("#b", "selected"); !ok {
		t.Fatal("current option was lost")
	}
	if !strings.Contains(source, `id="shadow-check" checked=""`) {
		t.Fatalf("shadow control lost current checked state: %s", source)
	}
}
