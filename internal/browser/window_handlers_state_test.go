package browser

import (
	"testing"
)

func TestWindowHandlerAttributesUseListenerState(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const handlers=Object.getOwnPropertyNames(window).filter(n=>/^on/.test(n)&&Object.getOwnPropertyDescriptor(window,n)?.set);
if(handlers.some(n=>window[n]!==null))throw new Error('non-null default');
const log=[];window.addEventListener('click',()=>log.push('before'));
window.onclick=()=>log.push('old');window.addEventListener('click',()=>log.push('after'));
window.onclick=()=>log.push('handler');window.dispatchEvent(new Event('click'));
if(log.join(',')!=='before,handler,after')throw new Error(log);
window.onclick=42;if(window.onclick!==null)throw new Error('primitive handler retained');
const d=Object.getOwnPropertyDescriptor(window,'onclick');
try{d.get.call({});throw new Error('invalid receiver accepted')}catch(e){if(e.name!=='TypeError')throw e}
return d.get.call(null)===null;
})()`, true)
	})
}

func TestInitialEmptyDocumentHasNoDoctype(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const f=document.createElement('iframe');document.body.append(f);
const d=f.contentDocument;
return document.compatMode==='BackCompat'&&document.doctype===null&&d.compatMode==='BackCompat'&&d.doctype===null&&d.documentElement.tagName==='HTML'&&d.body.tagName==='BODY';
})()`, true)
	})
}

func TestWindowProxyWritesPreserveAccessorAndReceiver(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const f=document.createElement('iframe');document.body.append(f);const w=f.contentWindow,callback=()=>{};
if(!Reflect.set(w,'onclick',callback)||w.onclick!==callback)throw new Error('owner accessor write');
if(!Reflect.set(w,'onclick',callback,window)||window.onclick!==callback)throw new Error('borrowed Window setter');
try{Reflect.set(w,'onclick',callback,{});throw new Error('invalid receiver accepted')}catch(e){if(e.name!=='TypeError')throw e}
const other={};Object.defineProperty(w,'custom',{value:1,writable:true,configurable:true});
if(!Reflect.set(w,'custom',2,other)||other.custom!==2||w.custom!==1)throw new Error('data receiver');
let receiver;Object.defineProperty(w,'accessor',{set(value){receiver=this;this.received=value},configurable:true});
if(!Reflect.set(w,'accessor',3,other)||receiver!==other||other.received!==3)throw new Error('custom accessor receiver');
if(!Reflect.set(w,'absent',4,other)||other.absent!==4||w.absent!==undefined)throw new Error('missing property receiver');
return !Reflect.set(w,'window',null)&&!Reflect.set(w,'custom',1,null)&&!Reflect.set(w,'missing',1,null);
})()`, true)
	})
}

func TestPostMessageUsesOwningFunctionAndCallingDocument(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(async()=>{
const f=document.createElement('iframe');document.body.append(f);const w=f.contentWindow,post=w.postMessage;
if(post!==Object.getOwnPropertyDescriptor(w,'postMessage').value||Object.getPrototypeOf(post)!==w.Function.prototype||post.length!==1||Object.hasOwn(post,'prototype'))throw new Error('postMessage ownership/shape');
try{new post('unused');throw new Error('constructible postMessage')}catch(e){if(e.name!=='TypeError')throw e}
try{post();throw new Error('missing arity check')}catch(e){if(e.name!=='TypeError')throw e}
w.eval("addEventListener('message',e=>parent.postMessage({sourceMatches:e.source===parent},'*'))");
const result=new Promise(resolve=>{const listener=e=>{if(e.source!==w)return;removeEventListener('message',listener);resolve(e.data.sourceMatches)};addEventListener('message',listener)});
w.postMessage('ping','*');return await result;
})()`, true)
	})
}

func TestNodeFilterIsNonConstructibleInterface(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{if(typeof NodeFilter!=='function'||NodeFilter.length!==0||Object.hasOwn(NodeFilter,'prototype')||!Object.isExtensible(NodeFilter))return false;
const d=Object.getOwnPropertyDescriptor(NodeFilter,'SHOW_ELEMENT');if(d.value!==1||d.writable||d.configurable||!d.enumerable)return false;
try{NodeFilter();return false}catch(e){if(e.name!=='TypeError')return false}
try{new NodeFilter();return false}catch(e){if(e.name!=='TypeError')return false}
try{Reflect.construct(function(){},[],NodeFilter);return false}catch(e){return e.name==='TypeError'}
})()`, true)
	})
}
