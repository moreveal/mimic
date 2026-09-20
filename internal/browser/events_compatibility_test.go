package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
)

func TestClickPropagationChrome152(t *testing.T) {
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
	value, err := p.Evaluate(context.Background(), `(()=>{
 const root=document.createElement('div'),child=document.createElement('button');root.appendChild(child);document.body.appendChild(root);
 const trace=[];let sample;
 for(const [node,name] of [[root,'root'],[child,'child']])for(const capture of [true,false])node.addEventListener('click',e=>{trace.push([name,capture,e.target===child,e.currentTarget===node,e.eventPhase]);sample=[e instanceof PointerEvent,e instanceof MouseEvent,e instanceof UIEvent,!e.isTrusted,e.bubbles,e.cancelable,e.composed,e.pointerId===-1,e.pointerType==='',e.view===window,e.detail===0,e.buttons===0,e.which===1]},{capture});
 child.click();
 if(JSON.stringify(trace)!=='[["root",true,true,true,1],["child",true,true,true,2],["child",false,true,true,2],["root",false,true,true,3]]'||!sample.every(Boolean))return JSON.stringify({trace,sample});
 const e=new Event('custom',{bubbles:true,cancelable:true});let calls=0;const f=()=>calls++;child.addEventListener('custom',f);child.addEventListener('custom',f);child.dispatchEvent(e);child.removeEventListener('custom',f);child.dispatchEvent(e);
 if(calls!==1||e.target!==child||e.currentTarget!==null||e.eventPhase!==0||e.composedPath().length)return 'post dispatch';
 const loadTrace=[];window.addEventListener('load',()=>loadTrace.push('window'),true);document.addEventListener('load',()=>loadTrace.push('document'),true);child.addEventListener('load',()=>loadTrace.push('child'),true);child.dispatchEvent(new Event('load'));if(JSON.stringify(loadTrace)!=='["document","child"]')return JSON.stringify(loadTrace);
 let once=0;child.addEventListener('once',()=>once++,{once:true});child.dispatchEvent(new Event('once'));child.dispatchEvent(new Event('once'));if(once!==1)return 'once';
 child.addEventListener('passive',e=>e.preventDefault(),{passive:true});if(!child.dispatchEvent(new Event('passive',{cancelable:true})))return 'passive';
 const controller=new AbortController();child.addEventListener('abortable',()=>calls++,{signal:controller.signal});controller.abort();child.dispatchEvent(new Event('abortable'));if(calls!==1)return 'signal';
 child.disabled=true;trace.length=0;child.click();if(trace.length)return 'disabled';
 const host=document.createElement('div'),shadow=host.attachShadow({mode:'closed'}),inner=document.createElement('span');shadow.append(inner);document.body.append(host);const shadowLog=[];
 for(const [node,name] of [[host,'host'],[shadow,'shadow'],[inner,'child']])for(const capture of [true,false])node.addEventListener('probe',e=>shadowLog.push([name,capture,e.eventPhase,e.target===host?'host':'child',e.composedPath().includes(shadow)]),capture);
 const composed=new Event('probe',{composed:true,bubbles:false});inner.dispatchEvent(composed);
 if(JSON.stringify(shadowLog)!=='[["host",true,2,"host",false],["shadow",true,1,"child",true],["child",true,2,"child",true],["child",false,2,"child",true],["host",false,2,"host",false]]'||composed.target!==host)return JSON.stringify(shadowLog);
 return 'ok';
})()`)
	if err != nil || value != "ok" {
		t.Fatalf("event semantics: %v %v", value, err)
	}
}
