package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
)

func TestFormControlDirtyValuesAndReset(t *testing.T) {
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
	value, err := p.Evaluate(context.Background(), `(()=>{
 const f=document.createElement('form');f.innerHTML='<input value="server"><input type="checkbox" checked><textarea>initial</textarea><select><option>A</option><option selected>B</option></select>';document.body.appendChild(f);
 const [input,check]=f.querySelectorAll('input'),area=f.querySelector('textarea'),select=f.querySelector('select');
 input.value='edit';input.defaultValue='new';check.checked=false;area.value='edit';area.defaultValue='new';select.value='A';
 const dirty=input.value==='edit'&&input.getAttribute('value')==='new'&&!check.checked&&check.defaultChecked&&area.value==='edit'&&area.defaultValue==='new'&&select.selectedIndex===0;
 f.reset();return dirty&&input.value==='new'&&check.checked&&area.value==='new'&&select.value==='B'&&select.options.selectedIndex===1;
 })()`)
	if err != nil || value != true {
		t.Fatalf("dirty form state: %v %v", value, err)
	}
}

func TestFormControlInitialValueAndDirectChildText(t *testing.T) {
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
	value, err := p.Evaluate(context.Background(), `(()=>{
 const input=document.createElement('input');let query='';if(input.value!==query)query=input.value;
 const initial=query.length===0&&input.defaultValue===''&&input.checked===false&&input.defaultChecked===false;
 const textarea=document.createElement('textarea');textarea.appendChild(document.createTextNode('direct'));const child=document.createElement('span');child.textContent='nested';textarea.appendChild(child);
 const select=document.createElement('select');select.innerHTML='<option value="a">A</option><option value="b">B</option>';select.selectedIndex=-1;const none=select.value===''&&select.selectedIndex===-1;select.remove(1);
 return initial&&textarea.defaultValue==='direct'&&textarea.value==='direct'&&none&&select.selectedIndex===0;
 })()`)
	if err != nil || value != true {
		t.Fatalf("initial/child-text controls: %v %v", value, err)
	}
}
