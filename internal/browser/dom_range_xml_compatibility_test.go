package browser

import (
	"context"
	"testing"
)

func TestTraversalIncludesCharacterDataAndRemainsLive(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
const root=document.createElement('div');root.innerHTML='a<span>b<!--c--></span>';
const iterator=document.createNodeIterator(root,NodeFilter.SHOW_TEXT|NodeFilter.SHOW_COMMENT),seen=[];
for(let node;(node=iterator.nextNode());)seen.push(node.nodeType+':'+node.data);
root.appendChild(document.createTextNode('d'));
const live=iterator.previousNode().data==='c'&&iterator.nextNode().data==='c'&&iterator.nextNode().data==='d';
const rejected=document.createElement('section');rejected.innerHTML='<i>x</i>';root.append(rejected);
const descendants=document.createNodeIterator(root,NodeFilter.SHOW_ELEMENT,n=>n===rejected?NodeFilter.FILTER_REJECT:NodeFilter.FILTER_ACCEPT),names=[];
for(let node;(node=descendants.nextNode());)names.push(node.localName);
return [seen.join(','),live,names.join(',')];
})()`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.([]any)
	if got[0] != "3:a,3:b,8:c" || got[1] != true || got[2] != "div,span,i" {
		t.Fatalf("live traversal mismatch: %#v", got)
	}
}

func TestRangeUsesBoundaryPointsForTextAndContents(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
const root=document.createElement('div');root.innerHTML='<b>hello</b><i> world</i>';document.body.append(root);
const first=root.firstChild.firstChild,last=root.lastChild.firstChild,r=document.createRange();
r.setStart(first,1);r.setEnd(last,3);
const clone=r.cloneContents(),before=[String(r),clone.firstChild.localName,clone.textContent,r.commonAncestorContainer===root];
const extracted=r.extractContents(),after=[extracted.textContent,root.textContent,r.collapsed];
const insertion=document.createRange(),text=root.firstChild.firstChild;insertion.setStart(text,1);insertion.collapse(true);
const mark=document.createElement('u');mark.textContent='!';insertion.insertNode(mark);
return [before,after,root.textContent,root.querySelector('u')===mark];
})()`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.([]any)
	before, after := got[0].([]any), got[1].([]any)
	if before[0] != "ello wo" || before[1] != "b" || before[2] != "ello wo" || before[3] != true || after[0] != "ello wo" || after[1] != "hrld" || after[2] != true || got[2] != "h!rld" || got[3] != true {
		t.Fatalf("range boundary mismatch: %#v", got)
	}
}

func TestXMLSerializerRoundTripNamespacesAndEscaping(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `(()=>{
const parser=new DOMParser(),doc=parser.parseFromString('<r xmlns="urn:r" xmlns:p="urn:p"><p:c p:a="&quot;&amp;">x&lt;y</p:c></r>','application/xml');
const serializer=new XMLSerializer(),text=serializer.serializeToString(doc),round=parser.parseFromString(text,'application/xml'),child=round.documentElement.firstChild;
const made=parser.parseFromString('<root/>','application/xml');made.documentElement.setAttributeNS('urn:attr','value','a&b');
const generated=serializer.serializeToString(made);
return [Object.prototype.toString.call(serializer),text,round.querySelector('parsererror')===null,child.namespaceURI,child.getAttributeNS('urn:p','a'),child.textContent,generated];
})()`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.([]any)
	if got[0] != "[object XMLSerializer]" || got[1] != `<r xmlns="urn:r" xmlns:p="urn:p"><p:c p:a="&quot;&amp;">x&lt;y</p:c></r>` || got[2] != true || got[3] != "urn:p" || got[4] != "\"&" || got[5] != "x<y" {
		t.Fatalf("XML serialization mismatch: %#v", got)
	}
}
