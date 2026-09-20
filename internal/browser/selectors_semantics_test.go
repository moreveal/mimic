package browser

import (
	"context"
	"testing"
)

func TestSelectorsUseLibraryGrammarAndCanonicalState(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.createElement('section');root.id='selector-root';
 root.innerHTML='<ul><li id="a" class="item" data-label="one two"></li><!-- gap --><li id="b" class="other"></li><li id="escaped:id" class="item"></li></ul>';
 document.body.appendChild(root);globalThis.selectorRoot=root;
 const check=(selector,ids)=>Array.from(root.querySelectorAll(selector),n=>n.id).join(',')===ids;
 if(!check('li:nth-child(2n+1)','a,escaped:id'))return 'nth';
 if(!check(':scope > ul > li[data-label~="two"]','a'))return 'scope';
 if(!check('li:has(+ .other)','a'))return 'has';
 if(!check('li:is(.item,:unknown)','a,escaped:id'))return 'forgiving';
 if(document.getElementById('escaped:id')!==root.firstChild.lastChild)return 'getElementById literal';
 if(!check('#escaped\\:id','escaped:id'))return 'escape';
 if(!check('li::before',''))return 'pseudo-element';
 if(root.querySelector('#b')!==root.firstChild.children[1])return 'identity';
 const odd=document.createElement('span');odd.className='left\u00a0right';root.appendChild(odd);if(root.querySelector('.left')!==null)return 'non-CSS whitespace';
 const list=root.querySelectorAll('.item');root.firstChild.firstChild.className='gone';
 if(list.length!==2||root.querySelectorAll('.item').length!==1)return 'snapshot/freshness';
 for(const selector of ['[','li,','> li',':contains("x")','[a!=b]',':unknown']) {
  try {root.querySelectorAll(selector);return 'accepted '+selector;}catch(error){if(error.name!=='SyntaxError')return error.name;}
 }
 return true;
})()`)
	if err != nil || value != true {
		t.Fatalf("selector semantics: %v %v", value, err)
	}
	d, _ := p.Document()
	n, ok := d.Find("#b")
	if !ok {
		t.Fatal("missing canonical node")
	}
	if err := d.SetAttribute(n.ID, "class", "from-host"); err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(context.Background(), `selectorRoot.querySelector('.from-host').id==='b'`)
	if err != nil || value != true {
		t.Fatalf("host mutation freshness: %v %v", value, err)
	}
}

func TestSelectorsFragmentAndClosestScope(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 const fragment=document.createDocumentFragment(),parent=document.createElement('div'),child=document.createElement('i');
 parent.id='parent';child.id='child';parent.appendChild(child);fragment.appendChild(parent);
 if(fragment.querySelector('div > i')!==child||fragment.querySelectorAll('*').length!==2)return 'fragment';
 if(child.closest(':scope')!==child||child.closest('div')!==parent)return 'closest';
 const list=fragment.querySelectorAll('i');parent.removeChild(child);
 return list[0]===child&&fragment.querySelectorAll('i').length===0&&child.matches(':scope');
})()`)
	if err != nil || value != true {
		t.Fatalf("fragment selectors: %v %v", value, err)
	}
}

func TestSelectorNativeLeafPreservesAttributeCasePolicy(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.createElement('div');root.innerHTML='<input type="TEXT" data-value="Mixed"><span data-value="a:b"></span>';document.body.appendChild(root);
 return root.querySelector('[type="text"]')===root.firstChild&&root.querySelector('[data-value="mixed"]')===null&&root.querySelector('[data-value="Mixed"]')===root.firstChild&&root.querySelector('[data-value="a:b"]')===root.lastChild&&root.querySelectorAll('[data-value]').length===2;
})()`)
	if err != nil || value != true {
		t.Fatalf("attribute policy: %v %v", value, err)
	}
}

func TestSelectorCandidatesPreserveOrderScopeAndFallback(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	v, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.createElement('section');root.className='outside';
 root.innerHTML='<div class="group"><a id="a" data-k="MiXeD"></a><span id="b"></span><a id="c"></a></div><a id="d"></a>';
 document.body.appendChild(root);
 const ids=s=>Array.from(root.querySelectorAll(s),n=>n.id).join(',');
 if(ids('.group a, .group > span, a#a')!=='a,b,c')return 'union order and dedup';
 if(ids(':scope > a')!=='d'||ids('.outside > .group > a')!=='a,c')return 'scope';
 if(ids('a[data-k="mixed" i]')!=='a'||ids(':is(a,span):not(#c)')!=='a,b,d')return 'fallback';
 if(ids('a:has(+ span)')!=='a'||ids('.group > :last-child')!=='c')return 'structural';
 if(root.querySelector('.group a')!==document.getElementById('a'))return 'identity';
 const list=root.querySelectorAll('.group a');root.firstChild.lastChild.remove();
 return list.length===2&&ids('.group a')==='a'&&root.querySelector('article a')===null;
})()`)
	if err != nil || v != true {
		t.Fatalf("candidate filtering: %v %v", v, err)
	}
}
