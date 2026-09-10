package browser

import (
	"context"
	"testing"
)

// Expected observations measured against frozen Chrome 152.0.7977.82.
func TestHTMLCollectionReflectionAndCanonicalQueries(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.createElement('div');root.innerHTML='<span id="entry" name="alias" class="a b"></span><span id="length" class="a"></span>';
 const list=root.getElementsByTagName('span'),first=root.firstElementChild;
 const descriptor=(key,enumerable)=>{const d=Object.getOwnPropertyDescriptor(list,key);return !!d&&d.value===first&&!d.writable&&d.enumerable===enumerable&&d.configurable};
 const before=Reflect.ownKeys(list).join(',');
 const checks=[document.getElementsByTagName('span')===document.getElementsByTagName('span'),list===root.getElementsByTagName('span'),list!==root.getElementsByTagName('SPAN'),root.children===root.children,root.getElementsByClassName('a')===root.getElementsByClassName('a'),root.getElementsByClassName(' a ')!==root.getElementsByClassName('a'),descriptor('0',true),descriptor('entry',false),descriptor('alias',false),!Object.hasOwn(list,'length'),Object.keys(list).join(',')==='0,1',before==='0,1,entry,alias,length'];
 first.id='changed';first.removeAttribute('name');checks.push(!Object.hasOwn(list,'entry'),!Object.hasOwn(list,'alias'),Object.hasOwn(list,'changed'));
 root.removeChild(first);checks.push(list.length===1,Reflect.ownKeys(list).join(',')==='0,length');
 const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('name','svgAlias');svg.id='svgEntry';root.appendChild(svg);
 checks.push(root.children.namedItem('svgAlias')===null,root.children.namedItem('svgEntry')===svg);
 return checks.every(Boolean);
 })()`)
	if err != nil || value != true {
		t.Fatalf("collection observations: %v %v", value, err)
	}
}
