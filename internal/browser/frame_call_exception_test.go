package browser

import "testing"

func TestBorrowedCallsPreserveThrownValuesAndRealm(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const f=document.createElement('iframe');document.body.append(f);const w=f.contentWindow;
w.eval("window.sentinel={marker:1};window.throwSentinel=()=>{throw sentinel};window.throwPrimitive=()=>{throw 17}");
try{w.throwSentinel();return false}catch(e){if(e!==w.sentinel)return false}
try{w.throwPrimitive();return false}catch(e){if(e!==17)return false}
const getter=Object.getOwnPropertyDescriptor(w,'onclick').get;
try{getter.call({});return false}catch(e){if(e.name!=='TypeError'||!(e instanceof w.TypeError))return false}
const next=w.eval('Object.getPrototypeOf([][Symbol.iterator]()).next');
try{next.call({});return false}catch(e){return e.name==='TypeError'&&e instanceof w.TypeError}
})()`, true)
	})
}
