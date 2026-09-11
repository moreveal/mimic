package browser

import (
	"testing"
)

func TestWindowHandlerAttributesUseListenerState(t *testing.T) {
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
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const f=document.createElement('iframe');document.body.append(f);
const d=f.contentDocument;
return document.compatMode==='BackCompat'&&document.doctype===null&&d.compatMode==='BackCompat'&&d.doctype===null&&d.documentElement.tagName==='HTML'&&d.body.tagName==='BODY';
})()`, true)
	})
}

