package browser

import (
	"context"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFrameHTMLAllCollectionPreservesUndetectabilityAndIdentity(t *testing.T) {
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.NewContext().NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	value, err := p.Evaluate(ctx, `(()=>{
		const frame=document.createElement('iframe');document.body.appendChild(frame);
		const win=frame.contentWindow,doc=frame.contentDocument,all=doc.all;
		const node=doc.createElement('div');node.id='bridge-all-node';doc.body.appendChild(node);
		const identity=win.eval('(value)=>value===document.all');
		const echo=win.eval('(value)=>value');
		const localObject={marker:17},localFunction=value=>value;
		win.remoteObject=localObject;win.remoteFunction=localFunction;win.remoteAll=document.all;
		return {
			type:typeof all==='undefined',boolean:Boolean(all)===false,loose:all==null,strict:all!==undefined,
			tag:Object.prototype.toString.call(all)==='[object HTMLAllCollection]',
			stable:all===doc.all&&all===win.eval('document.all'),
			prototype:Object.getPrototypeOf(all)===win.eval('HTMLAllCollection.prototype'),
			passedBack:identity(all),echo:echo(all)===all,
			localEcho:echo(document.all)===document.all,
			globalObject:win.eval('remoteObject')===localObject,
			globalFunction:win.eval('remoteFunction')===localFunction,
			globalAll:win.eval('remoteAll')===document.all,
			globalCall:win.eval('remoteFunction(remoteObject)')===localObject,
			item:all.item('bridge-all-node')===node,call:all('bridge-all-node')===node,
			index:all[0]===doc.documentElement,
			hasIndex:'0' in all,hasMethod:'item' in all,hasNamed:'bridge-all-node' in all,hasAbsent:!('absent-bridge-key' in all),
			keys:Reflect.ownKeys(all).includes('0')&&Reflect.ownKeys(all).includes('bridge-all-node'),
			descriptor:(()=>{const d=Object.getOwnPropertyDescriptor(all,'0');return d.value===doc.documentElement&&d.enumerable&&d.configurable&&!d.writable})(),
			live:(()=>{const n=all.length;node.remove();return all.length===n-1})()
		};
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	for name, result := range value.(map[string]any) {
		if result != true {
			t.Errorf("%s: %v", name, result)
		}
	}
}

func TestFrameGlobalAssignmentReadsAuthoritativeState(t *testing.T) {
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.NewContext().NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	value, err := p.Evaluate(ctx, `(()=>{
		const f=document.createElement('iframe');document.body.appendChild(f);
		const w=f.contentWindow,evaluate=w.eval, object={},symbol=Symbol('roundtrip');
		w.plain=object;w[symbol]=object;
		const out={plain:w.plain===object,symbol:w[symbol]===object,readonly:Reflect.set(w,'undefined',17)===false&&w.undefined===undefined};
		const replacement=()=>42;w.eval=replacement;w.postMessage=replacement;
		out.eval=w.eval===replacement&&w.eval()===42;
		out.post=w.postMessage===replacement&&w.postMessage()===42;
		evaluate('globalThis.eval=17;globalThis.postMessage=19');
		out.sourceMutation=w.eval===17&&w.postMessage===19;
		return out;
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	for name, result := range value.(map[string]any) {
		if result != true {
			t.Errorf("%s: %v", name, result)
		}
	}
}
