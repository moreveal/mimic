package browser

import (
	"context"
	"testing"
)

// Each observation may share derived reads internally, but author mutations in
// the same JavaScript task must be visible to the very next observation.
func TestGeometryReadCachesObserveSameTaskMutations(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 document.head.innerHTML='<style>.a .item{width:20px;font-size:10px}.b .item{width:40px;font-size:20px}.item{height:1em}.item:focus{height:30px}</style>';
 document.body.innerHTML='<div class="a"><button class="item">text</button></div><div class="b"></div>';
 const a=document.body.children[0],b=document.body.children[1],item=a.firstChild,style=getComputedStyle(item);
 const read=()=>{const box=item.getBoundingClientRect();return [style.width,style.fontSize,box.height]};
 const first=read();a.className='b';const changed=read();
 a.className='a';b.append(item);const adopted=read();
 item.style.fontSize='12px';const inline=read();
 item.focus();const focused=read();
 item.blur();item.style.fontSize='';b.className='a';const reset=read();
 item.hidden=true;const hidden=item.getBoundingClientRect();item.hidden=false;
 const restored=read();
 return JSON.stringify({first,changed,adopted,inline,focused,reset,hidden:[hidden.width,hidden.height],restored,same:item===b.firstChild});
 })()`)
		want := `{"first":["20px","10px",10],"changed":["40px","20px",20],"adopted":["40px","20px",20],"inline":["40px","12px",12],"focused":["40px","12px",30],"reset":["20px","10px",10],"hidden":[0,0],"restored":["20px","10px",10],"same":true}`
		if err != nil || value != want {
			t.Fatalf("same-task invalidation: %v %v", value, err)
		}
	})
}

// Flex/grid construction and the legacy containing-block path can recursively
// request one another. A cycle fallback must not escape as ready geometry or be
// reused after a synchronous style mutation.
func TestGeometryComputationPlanDoesNotPublishCycleFallback(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 document.body.innerHTML='<div id="root" style="display:flex;width:240px"><div id="item" style="display:grid;width:50%;grid-template-columns:1fr"><div id="leaf" style="width:100%;height:12px"></div></div></div>';
 const root=document.querySelector('#root'),item=document.querySelector('#item'),leaf=document.querySelector('#leaf');
 const read=()=>{
   const a=leaf.getBoundingClientRect(),b=item.getBoundingClientRect();
   return [a.width,a.height,a.left,b.width,item.clientWidth,root.scrollWidth];
 };
 const first=read(),same=read();
 root.style.width='320px';const changed=read(),again=read();
 return JSON.stringify({first,same,changed,again,finite:[...first,...changed].every(Number.isFinite)});
 })()`)
		want := `{"first":[120,12,0,120,120,240],"same":[120,12,0,120,120,240],"changed":[160,12,0,160,160,320],"again":[160,12,0,160,160,320],"finite":true}`
		if err != nil || value != want {
			t.Fatalf("geometry computation cycle: %v %v", value, err)
		}
	})
}

func TestStylesheetAttributeIndexPreservesFullMatching(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 document.head.innerHTML='<style>div{width:1px}[data-on]{width:2px}[data-on="YES" i]{width:3px}.scope > [data-on^="Y"]{width:4px}[data-on$="S"].chosen{width:5px}:not([data-off]){height:7px}</style>';
 document.body.innerHTML='<section class="scope"><div></div></section>';
 const node=document.querySelector('div'),style=getComputedStyle(node),values=[];
 const read=()=>values.push(style.width);
 read();node.setAttribute('data-on','no');read();node.setAttribute('data-on','yes');read();node.setAttribute('data-on','YES');read();node.className='chosen';read();node.removeAttribute('data-on');read();
 return JSON.stringify([values,style.height]);
 })()`)
		if err != nil || value != `[["1px","2px","3px","4px","5px","1px"],"7px"]` {
			t.Fatalf("attribute candidates: %v %v", value, err)
		}
	})
}

func TestGeometryAncestorSnapshotsObserveFontAndFlatTreeChanges(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.documentElement;root.style.fontSize='10px';
 document.body.innerHTML='<section style="font-size:200%"><div style="font-size:2em;height:1em;width:10px"></div><div style="font-size:1rem;height:1em;width:10px"></div></section>';
 const parent=document.body.firstElementChild,a=parent.children[0],b=parent.children[1];
 const read=()=>{parent.getBoundingClientRect();return [a.offsetHeight,b.offsetHeight,getComputedStyle(a).fontSize,getComputedStyle(b).fontSize]};
 const first=read();root.style.fontSize='20px';const rootChanged=read();
 parent.style.fontSize='12px';const parentChanged=read();
 parent.style.display='none';const hidden=[a.offsetHeight,b.offsetHeight];parent.style.display='';
 const host=document.createElement('div');document.body.append(host);const shadow=host.attachShadow({mode:'open'});
 shadow.innerHTML='<slot name="chosen"><div style="width:5px;height:7px"></div></slot>';
 const fallback=shadow.firstChild.firstChild;host.append(a);
 const unassigned=[a.offsetHeight,fallback.offsetHeight];a.slot='chosen';
 const assigned=[a.offsetHeight,fallback.offsetHeight];a.remove();
 const removed=[a.offsetHeight,fallback.offsetHeight];parent.append(a);
 return JSON.stringify({first,rootChanged,parentChanged,hidden,unassigned,assigned,removed,restored:read()});
 })()`)
		want := `{"first":[40,10,"40px","10px"],"rootChanged":[80,20,"80px","20px"],"parentChanged":[24,20,"24px","20px"],"hidden":[0,0],"unassigned":[0,7],"assigned":[40,0],"removed":[0,7],"restored":[24,20,"24px","20px"]}`
		if err != nil || value != want {
			t.Fatalf("ancestor observation invalidation: %v %v", value, err)
		}
	})
}

func TestStylesheetSelectorReusePreservesConditionsAndPseudoState(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 document.head.innerHTML='<style>.item{width:10px}.scope > .item{width:15px}.scope > .item{width:20px}.item::before{content:"";display:block;height:2px}.item::after{content:"";display:block;height:3px}.item[data-on]::after{height:5px}.item:focus::before{height:4px}@media(min-width:1px){.scope > .item{width:30px}}@media(min-width:99999px){.scope > .item{width:999px}}</style>';
 document.body.innerHTML='<section class="scope"><div class="item" tabindex="0"></div><div class="item"></div></section>';
 const owner=document.body.firstChild,item=owner.firstChild,other=owner.lastChild,sheet=document.head.firstChild;
 const read=()=>{owner.getBoundingClientRect();const a=item.getBoundingClientRect(),b=other.getBoundingClientRect();return [a.width,a.height,b.width,b.height]};
 const initial=read();item.setAttribute('data-on','');const attribute=read();
 item.focus();const focus=read();sheet.textContent=sheet.textContent.replace('min-width:1px','min-width:99999px');const media=read();
 owner.className='';const ancestry=read();item.blur();item.removeAttribute('data-on');
 const reset=read();
 return JSON.stringify({initial,attribute,focus,media,ancestry,reset});
 })()`)
		want := `{"initial":[30,5,30,5],"attribute":[30,7,30,5],"focus":[30,9,30,5],"media":[20,9,20,5],"ancestry":[10,9,10,5],"reset":[10,5,10,5]}`
		if err != nil || value != want {
			t.Fatalf("selector reuse across rules/pseudos: %v %v", value, err)
		}
	})
}
