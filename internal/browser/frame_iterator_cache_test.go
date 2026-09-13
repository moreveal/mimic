//go:build (windows || linux) && amd64

package browser

import "testing"

// Exercise the alias boundary of fresh native iterator result caching. Custom
// next functions and results exposed back to their owner must remain live.
func TestCrossFrameIteratorResultCache(t *testing.T) {
	for _, snapshot := range []bool{false, true} {
		name := "ordinary"
		if snapshot {
			name = "snapshot"
		}
		t.Run(name, func(t *testing.T) {
			if snapshot {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "")
			} else {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
			}
			p := bootstrapSnapshotPage(t)
			if snapshot {
				bootstrapSnapshotWarm(t, p)
			}
			value := bootstrapSnapshotEvaluate(t, p, `(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;
 const check=(ok,message)=>{if(!ok)throw new Error(message)};
 const step=()=>w.eval('[7][Symbol.iterator]()').next();
 let r=step();check(r.value===7&&!r.done,'initial fields');
 const d=Object.getOwnPropertyDescriptor(r,'value');check(d.value===7&&d.writable&&d.enumerable&&d.configurable,'descriptor');
 check(Object.getPrototypeOf(r)===w.Object.prototype,'prototype');
 r.value=8;r.done=true;check(r.value===8&&r.done,'direct mutation');
 r=step();w.eval('(r)=>{r.value=9;r.done=true}')(r);check(r.value===9&&r.done,'argument escape');
 r=step();const holder={result:r};w.eval('(holder)=>{holder.result.value=10}')(holder);check(r.value===10,'transitive escape');
 r=step();w.eval('(r)=>{Object.defineProperty(r,"value",{get:()=>11});delete r.done}')(r);check(r.value===11&&r.done===undefined,'owner descriptor mutation');
 const custom=w.eval('(()=>{const result={value:1,done:false};globalThis.savedIteratorResult=result;return {next(){return result}}})()');
 r=custom.next();w.eval('savedIteratorResult.value=12');check(r.value===12,'custom reused result');
 const wrapped=w.eval('(()=>{const it=[13][Symbol.iterator](),next=it.next;it.next=function(){const r=next.call(this);globalThis.savedIteratorResult=r;return r};return it})()');
 r=wrapped.next();w.eval('savedIteratorResult.value=14');check(r.value===14,'wrapped native next');
 globalThis.iteratorCalls=0;globalThis.iteratorReentry=()=>++iteratorCalls;
 const getters=w.eval('(()=>{let a=[0,2];Object.defineProperty(a,0,{get(){parent.iteratorReentry();a.length=1;return 15}});return a[Symbol.iterator]()})()');
 r=getters.next();check(iteratorCalls===1&&r.value===15&&!r.done,'getter reentry');check(getters.next().done&&iteratorCalls===1,'no lookahead');
 const throwing=w.eval('(()=>{const a=[];Object.defineProperty(a,0,{get(){throw new Error("iterator getter")}});return a[Symbol.iterator]()})()');
 let threw=false;try{throwing.next()}catch(e){threw=true}check(threw,'getter exception');
 r=step();const same=w.eval('(x)=>x')(r);check(same===r,'canonical identity');
 const objects=w.eval('(()=>{globalThis.iteratorObject={};return [iteratorObject,undefined,null,true,"text",NaN,-0,19n][Symbol.iterator]()})()');
 check(objects.next().value===w.iteratorObject,'object-valued identity');
 check(objects.next().value===undefined&&objects.next().value===null&&objects.next().value===true&&objects.next().value==='text','primitive values');
 check(Number.isNaN(objects.next().value)&&Object.is(objects.next().value,-0)&&objects.next().value===19n,'special primitive values');
 r=step();const originalHasOwn=Object.prototype.hasOwnProperty;
 try{Object.prototype.hasOwnProperty=()=>{throw new Error('user hasOwn')};check(r.value===7&&!r.done,'captured hasOwn')}finally{Object.prototype.hasOwnProperty=originalHasOwn}
 const poisoned=w.eval('(()=>{Object.prototype.toJSON=function(){throw new Error("user toJSON")};return [21][Symbol.iterator]()})()');
 try{r=poisoned.next();check(r.value===21&&!r.done,'private iterator metadata')}finally{w.eval('delete Object.prototype.toJSON')}
 return true;
})()`)
			if value != true {
				t.Fatalf("iterator corpus: %v", value)
			}
			if snapshot {
				bootstrapSnapshotAssertRestored(t, p, 1)
			}
		})
	}
}
