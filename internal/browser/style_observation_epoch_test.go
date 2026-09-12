package browser

import (
	"context"
	"testing"
)

func TestCheckedSelectorsUseDirtyStateWithinSameJob(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		got, err := p.Evaluate(context.Background(), `(()=>{
 document.head.innerHTML='<style>input{width:10px}input:checked{width:20px}section:has(input:checked){height:7px}</style>';
 document.body.innerHTML='<section><input type="checkbox" checked data-literal=":checked"><input type="radio" name="r"><input type="radio" name="r"></section><select><option>A</option><option>B</option></select>';
 const [box,a,b]=document.querySelectorAll('input'),select=document.querySelector('select'),style=getComputedStyle(box);
 const read=()=>[box.matches(':checked'),box.matches(':is(:checked)'),box.matches(':not(:checked)'),style.width,document.querySelectorAll('input:checked').length];
 const initial=read();box.checked=false;const cleared=read();box.checked=true;const set=read();
 a.checked=true;b.checked=true;const radio=[a.matches(':checked'),b.matches(':checked')];
 const options=()=>Array.from(select.options,o=>o.matches(':checked'));
 const first=options();select.selectedIndex=1;const second=options();select.selectedIndex=-1;const none=options();
 let hiddenAlias=false;try{box.matches(':mimic-internal-checked')}catch(e){hiddenAlias=e.name==='SyntaxError'}
 let getters=0;Object.defineProperty(box,'checked',{get(){getters++;return false}});
 return JSON.stringify({initial,cleared,set,radio,first,second,none,literal:document.querySelector('[data-literal=":checked"]')===box,hiddenAlias,canonical:box.matches(':checked'),getters});
 })()`)
		want := `{"initial":[true,true,false,"20px",1],"cleared":[false,false,true,"10px",0],"set":[true,true,false,"20px",1],"radio":[false,true],"first":[true,false],"second":[false,true],"none":[false,false],"literal":true,"hiddenAlias":true,"canonical":true,"getters":0}`
		if err != nil || got != want {
			t.Fatalf("checked observation: %v %v", got, err)
		}
	})
}

func TestStyleObservationEpochCoversCSSOMShadowAndReentrantConversion(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		got, err := p.Evaluate(context.Background(), `(()=>{
 document.head.innerHTML='<style>.box{width:20px;height:5px}.other{width:30px}</style>';
 document.body.innerHTML='<div class="box"></div><input type="button" value="a"><div id="shadow-host"><span style="height:7px;display:block"></span></div>';
 const node=document.querySelector('.box'),owner=document.head.firstChild,sheet=owner.sheet,style=getComputedStyle(node),button=document.querySelector('input'),shadowHost=document.getElementById('shadow-host');
 const widths=[];const read=()=>widths.push(style.width);read();sheet.cssRules[0].style.width='40px';read();
 sheet.insertRule('.box{width:50px}',sheet.cssRules.length);read();sheet.deleteRule(2);read();
 const adopted=new CSSStyleSheet();adopted.replaceSync('.box{width:60px}');document.adoptedStyleSheets=[adopted];read();
 adopted.replaceSync('.box{width:70px}');read();document.adoptedStyleSheets=[];read();
 let ownerReads=0;Object.defineProperty(owner,'textContent',{get(){ownerReads++;return '.box{width:999px}'}});node.className='box other';read();
 const light=shadowHost.firstChild,prior=light.offsetHeight,shadow=shadowHost.attachShadow({mode:'open'});const absent=light.offsetHeight;
 shadow.innerHTML='<slot name="visible"></slot>';light.slot='visible';const assigned=light.offsetHeight;light.slot='missing';const removed=light.offsetHeight;
 const before=button.offsetWidth;let nested;
 button.value={toString(){nested=button.offsetWidth;node.style.width='80px';return 'a much longer button label'}};
 const after=button.offsetWidth;read();
 let valueReads=0;Object.defineProperty(button,'value',{get(){valueReads++;return 'fake'}});const canonical=button.offsetWidth;
 return JSON.stringify({widths,ownerReads,shadow:[prior,absent,assigned,removed],nestedOld:nested===before,grew:after>before,canonical:canonical===after,valueReads});
 })()`)
		want := `{"widths":["20px","40px","50px","40px","60px","70px","40px","30px","80px"],"ownerReads":0,"shadow":[7,0,7,0],"nestedOld":true,"grew":true,"canonical":true,"valueReads":0}`
		if err != nil || got != want {
			t.Fatalf("style epoch invalidation: %v %v", got, err)
		}
	})
}
