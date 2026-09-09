package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
)

// Chrome 152 preserves Attr and NamedNodeMap identities while sanitizers remove
// the first attribute repeatedly. A shape-only removeAttributeNode never
// shrinks this live collection and leaves those ordinary loops running forever.
func TestCanonicalAttributeRemovalChrome152(t *testing.T) {
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
 const e=document.createElement('div');e.setAttribute('z','one');e.setAttribute('a','two');const map=e.attributes,a=map[0];
 const checks=[map===e.attributes,a===map.item(0),a===e.getAttributeNode('Z'),a===map.getNamedItem('z'),a instanceof Attr,map instanceof NamedNodeMap,a.nodeType===2,a.ownerElement===e,a.ownerDocument===document,JSON.stringify(Array.from(map,x=>x.name))==='["z","a"]',map[2]===undefined,map.item(2)===null,JSON.stringify(Object.keys(map))==='["0","1"]'];
 e.setAttribute('z','updated');checks.push(a.value==='updated');a.value='written';checks.push(e.getAttribute('z')==='written');
 const removed=e.removeAttributeNode(a);checks.push(removed===a,a.ownerElement===null,a.value==='written',map.length===1);
 let error='';try{e.removeAttributeNode(a)}catch(ex){error=ex.name}checks.push(error==='NotFoundError');
 const other=document.createElement('span');other.setAttribute('q','v');try{e.removeAttributeNode(other.attributes[0]);checks.push(false)}catch(ex){checks.push(ex.name==='NotFoundError')}
 a.value='detached';checks.push(e.setAttributeNode(a)===null,e.attributes[1]===a,a.ownerElement===e,e.getAttribute('z')==='detached');e.removeAttribute('z');checks.push(a.ownerElement===null,a.value==='detached');
 const fresh=document.createAttribute('TITLE');fresh.value='title';e.setAttributeNode(fresh);checks.push(fresh.name==='title',e.title==='title');
 const loop=[];for(let i=0;map.length&&i<10;i++)loop.push(e.removeAttributeNode(map[0]).name);checks.push(map.length===0,JSON.stringify(loop)==='["a","title"]');
 return checks.every(Boolean)?'ok':JSON.stringify(checks);
})()`)
	if err != nil || value != "ok" {
		t.Fatalf("attribute semantics: %v %v", value, err)
	}
}
