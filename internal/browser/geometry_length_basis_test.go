package browser

import (
	"context"
	"testing"
)

func TestGeometryLengthsUseCanonicalLazyRootFontBasis(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.documentElement;
 root.style.fontSize='10px';
 document.body.innerHTML='<div style="width:12px;height:5px;padding:1px"></div>';
 const node=document.body.firstElementChild,style=getComputedStyle(node);
 let rootReads=0;
 Object.defineProperty(document,'documentElement',{configurable:true,get(){rootReads++;return root}});
 const px=[node.getBoundingClientRect().width,style.width];
 node.style.width='calc(1rem + 2px)';node.style.transform='translateX(1rem)';
 const first=[node.getBoundingClientRect().width,style.width,style.transform];
 root.style.fontSize='20px';
 const changed=[node.getBoundingClientRect().width,style.width,style.transform];
 node.style.width='2em';node.style.fontSize='7px';
 const em=[node.getBoundingClientRect().width,style.width];
 delete document.documentElement;
 return JSON.stringify({px,first,changed,em,rootReads});
 })()`)
		want := `{"px":[14,"12px"],"first":[14,"12px","matrix(1, 0, 0, 1, 10, 0)"],"changed":[24,"22px","matrix(1, 0, 0, 1, 20, 0)"],"em":[16,"14px"],"rootReads":0}`
		if err != nil || value != want {
			t.Fatalf("lazy canonical length basis: %v %v", value, err)
		}
	})
}
