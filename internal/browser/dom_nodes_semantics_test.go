package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestCharacterDataUsesCanonicalMutableText(t *testing.T) {
	parallelBrowserTest(t)
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
	ctx := context.Background()
	value, err := p.Evaluate(ctx, `(()=>{
 const root=document.createElement('div');root.id='character-root';document.body.appendChild(root);
 const text=document.createTextNode('initial');root.appendChild(text);globalThis.characterText=text;
 text.data='\ud83c\udf20 test';text.replaceData(1,2,'--');
 if(text.data!=='\ud83c--test'||text.nodeValue!==text.data)return false;
 text.data='updated';return text.length===7&&root.textContent==='updated';
})()`)
	if err != nil || value != true {
		detail, _ := p.Evaluate(ctx, `JSON.stringify({data:characterText.data,value:characterText.nodeValue,text:characterText.textContent,len:characterText.length,parent:characterText.parentNode.textContent})`)
		t.Log(detail)
		t.Fatalf("character data: %v %v", value, err)
	}
	d, _ := p.Document()
	root, _ := d.Find("#character-root")
	node, _ := d.FirstChild(root.ID)
	if err := d.SetTextContent(node.ID, "host mutation"); err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(ctx, `characterText.data==='host mutation'&&characterText.nodeValue==='host mutation'`)
	if err != nil || value != true {
		t.Fatalf("canonical host mutation: %v %v", value, err)
	}
}

func TestNodeTraversalAndLiveChildList(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.createElement('div');root.innerHTML='a<!--marker--><span>b</span>c';
 const list=root.childNodes, text=root.firstChild, comment=text.nextSibling, element=comment.nextSibling;
 if(list!==root.childNodes||list.length!==4||comment.nodeType!==8||comment.data!=='marker'||element.previousSibling!==comment||root.lastChild.data!=='c')return false;
 root.removeChild(comment);if(list.length!==3||text.nextSibling!==element)return false;
 const fragment=document.createDocumentFragment();fragment.append(text,element);
 if(text.parentNode!==fragment||text.nextSibling!==element||element.previousSibling!==text)return false;
 const copy=fragment.cloneNode(true);return copy!==fragment&&copy.firstChild!==text&&copy.textContent==='ab';
})()`)
	if err != nil || value != true {
		t.Fatalf("traversal: %v %v", value, err)
	}
}

func TestCharacterDataErrorsUseDOMExceptionCodes(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{const n=document.createTextNode('one');try{n.deleteData(4,1)}catch(e){return e instanceof DOMException&&e.name==='IndexSizeError'&&e.code===1&&n.data==='one'}return false})()`)
	if err != nil || value != true {
		t.Fatalf("DOM exception: %v %v", value, err)
	}
}
