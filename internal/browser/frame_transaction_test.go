package browser

import (
	"context"
	"testing"
	"time"
)

func TestFrameTransactionSpecialValuesAndNestedCalls(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		got, err := p.Evaluate(ctx, `(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;
 const identity=w.eval('(function(value){return value})'),object=w.eval('({})');
 const values=[undefined,null,NaN,Infinity,-Infinity,-0,42n,'Привет\u0000🌍',Symbol('local'),Symbol.for('shared')];
 for(const value of values){if(!Object.is(identity(value),value))return 'argument';object.value=value;if(!Object.is(object.value,value))return 'property';if(!Object.is(Object.getOwnPropertyDescriptor(object,'value').value,value))return 'descriptor'}
 const key=Symbol('key');object[key]=object;if(object[key]!==object||!Reflect.ownKeys(object).includes(key))return 'symbol';
 const parent={calls:0},callback=function(value){this.calls++;return value};
 const call=w.eval('(function(cb,receiver,arg){return cb.call(receiver,arg)})');
 if(call(callback,parent,parent)!==parent||parent.calls!==1)return 'nested callback';
 const thrown={marker:true},thrower=w.eval('(function(cb){cb()})');try{thrower(()=>{throw thrown});return 'missing throw'}catch(e){if(e!==thrown)return 'throw identity'}
 const foreign=w.eval('(()=>{const error={foreign:true};return{error,fail(){throw error}}})()');try{foreign.fail();return 'missing foreign throw'}catch(e){if(e!==foreign.error)return 'foreign throw identity'}
 return true;
 })()`)
		if err != nil || got != true {
			t.Fatalf("transaction values: %v, %v", got, err)
		}
	})
}

func TestFrameTransactionPrivateCodecAndLiveTraps(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		got, err := p.Evaluate(ctx, `(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;
 const remote=w.eval('(()=>{let reads=0;const object={get value(){reads++;return reads},fn(value){return value},get reads(){return reads}};return new Proxy(object,{get(target,key,receiver){return Reflect.get(target,key,receiver)}})})()');
 const poison="Object.prototype.toJSON=Array.prototype.toJSON=function(){throw Error('private codec exposed')};JSON.parse=JSON.stringify=function(){throw Error('public JSON used')}";
 w.eval(poison);eval(poison);
 if(remote.value!==1||remote.value!==2||remote.reads!==2)return 'stale getter';
 if(remote.fn('value')!=='value')return 'call';
 Object.defineProperty(remote,'added',{value:17,writable:true,configurable:true,enumerable:true});
 if(!('added' in remote)||remote.added!==17||Object.getOwnPropertyDescriptor(remote,'added').value!==17)return 'define';
 remote.added=19;if(remote.added!==19)return 'set';
 if(!Reflect.ownKeys(remote).includes('added')||!Object.getPrototypeOf(remote))return 'reflection';
 if(!delete remote.added||'added' in remote)return 'delete';
 return true;
 })()`)
		if err != nil || got != true {
			t.Fatalf("private codec/live traps: %v, %v", got, err)
		}
	})
}
