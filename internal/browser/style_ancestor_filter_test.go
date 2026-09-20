package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestStyleAncestorFilterMatchesCanonicalSelectors(t *testing.T) {
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
	navigateCapabilityFixture(t, p)
	// Expectations measured in frozen Chrome 152.0.7977.82, not the runtime's
	// public matches method. Exercise positive/negative matches, sibling boundaries,
	// nested functional selectors, reparenting and attribute invalidation.
	value, err := p.Evaluate(context.Background(), `(()=>{
	 document.head.innerHTML='<style id="rules"></style>';
	 document.body.innerHTML='<section class="outer"><i class="peer"></i><div class="middle"><span class="leaf" id="probe"></span></div></section><aside></aside>';
	 const style=document.getElementById('rules'),leaf=document.getElementById('probe'),middle=leaf.parentElement,outer=middle.parentElement;
	 const selectors=['.outer .middle > .leaf','.missing .leaf','section > .middle span','.peer + .middle > .leaf','.outer .peer + .middle > .leaf','.outer :is(.middle,.other) > .leaf',':is(.outer,.absent) .leaf','.outer .middle :not(.missing)',':not(.outer) > .middle > .leaf','.outer .missing, .leaf','#probe','.OUTER .leaf','aside .leaf'];
	 const expectations=['1011111101100','0011000011110','1011111101100','0000000011101','1011111101100','0010000100100','1011111101100'];
	 let checks=0,phase=0;
	 const check=()=>{let i=0;for(const selector of selectors){style.textContent='*{z-index:1}'+selector+'{z-index:17}';const expected=expectations[phase][i++]==='1'?'17':'1',actual=getComputedStyle(leaf).zIndex;if(actual!==expected)throw Error(checks+' '+selector+': '+actual+' != '+expected);checks++}phase++};
	 check();outer.className='OUTER';check();outer.className='outer';check();
	 document.querySelector('aside').appendChild(middle);check();outer.appendChild(middle);check();
	 leaf.classList.remove('leaf');check();leaf.classList.add('leaf');check();
	 const host=document.createElement('div');document.body.append(host);const shadow=host.attachShadow({mode:'open'});
	 shadow.innerHTML='<style>*{z-index:1}.outer .leaf{z-index:17}.inside .leaf{z-index:23}</style><div class="inside"><span class="leaf"></span></div>';
	 const inner=shadow.querySelector('.leaf');if(getComputedStyle(inner).zIndex!=='23')throw Error('shadow ancestor missing');
	 shadow.querySelector('.inside').className='';if(getComputedStyle(inner).zIndex!=='1')throw Error('stale shadow ancestry');
	 const dynamicCases=[
	  ['.host .leaf',h=>h.className='other','17','1'],
	  ['[data-mode=on] .leaf',h=>h.setAttribute('data-mode','on'),'1','17'],
	  ['.peer + .leaf',h=>h.firstChild.remove(),'17','1'],
	  ['.leaf:first-child',h=>h.firstChild.remove(),'1','17'],
	  ['.host:has(.flag) .leaf',h=>h.firstChild.classList.add('flag'),'1','17'],
	  ['.leaf:focus',h=>h.lastChild.focus(),'1','17'],
	  ['.leaf:checked',h=>h.lastChild.checked=true,'1','17'],
	  [':is(.host,.other) .leaf',h=>h.className='other','17','17'],
	  [':not(.other) > .leaf',h=>h.className='other','17','1']
	 ];
	 for(const [selector,mutate,before,after] of dynamicCases){
	  document.body.innerHTML='<div class="host"><i class="peer"></i><input class="leaf" type="checkbox"></div>';
	  style.textContent='*{z-index:1}'+selector+'{z-index:17}';
	  const h=document.body.firstChild,e=h.lastChild;
	  if(getComputedStyle(e).zIndex!==before)throw Error('initial dynamic '+selector);
	  mutate(h);if(getComputedStyle(e).zIndex!==after)throw Error('stale dynamic '+selector);
	 }
	 return checks;
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	if value == nil {
		t.Fatal("missing comparisons")
	}
}
