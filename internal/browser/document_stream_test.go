package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDynamicExternalScriptDoesNotDestructivelyWriteDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/write.js" {
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `document.write('<p id=wrong>replaced</p>');globalThis.executed=true`)
			return
		}
		fmt.Fprint(w, `<!doctype html><body><p id=original>retained</p>`)
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		// Measured in Chrome 152.0.7977.82: a late external script runs,
		// but its document.write cannot implicitly open/replace the document.
		got, err := p.Evaluate(ctx, `new Promise(resolve=>{const s=document.createElement('script');s.src='/write.js';s.onload=()=>resolve(!!globalThis.executed&&!!document.getElementById('original')&&!document.getElementById('wrong')&&document.readyState==='complete');s.onerror=()=>resolve(false);document.head.append(s)})`)
		if err != nil || got != true {
			t.Fatalf("late external script replaced document: %v %v", got, err)
		}
	})
}

func TestDocumentStreamIdentityAndSynchronousScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body>parent</body>") }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/caller#fragment"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);globalThis.streamFrame=f;
 const w=f.contentWindow,d=f.contentDocument;
 w.eval("globalThis.keep=7;document.expando=9;globalThis.oldDocument=document;globalThis.oldWindow=window");
 if(d.open()!==d||d.readyState!=='loading'||d.documentElement!==null)return 'open';
 if(!d.URL.endsWith('/caller')||!d.baseURI.endsWith('/caller'))return 'URL';
 d.write('<div id=x>a'); const x=d.getElementById('x');
 d.write('b</div><scr'); if(x.textContent!=='ab')return 'split-text';
 d.write('ipt>window.ran=42;document.write("<i id=n>nested</i>")</scr'+'ipt><p id=tail>tail</p>');
 if(w.ran!==42||d.getElementById('x')!==x||!d.getElementById('n')||!d.getElementById('tail'))return 'script';
 if(d.close()!==undefined||d.readyState!=='complete')return 'close';
 if(w.keep!==7||d.expando!==9||!w.eval('oldDocument===document&&oldWindow===window'))return 'identity';
 if(document.body.textContent!=='parent')return 'parent-mutated';
 return true;
})()`, true)
	})
}

func TestDocumentStreamListenersAndNestedClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body>fixture</body>") }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);globalThis.streamFrame=f;
 return f.contentWindow.eval(`+"`"+`(()=>{
 globalThis.events=[];
 const old=document.createElement('button'),detached=document.createElement('button');document.body.appendChild(old);
 old.addEventListener('probe',()=>events.push('old'));detached.addEventListener('probe',()=>events.push('detached'));
 window.addEventListener('probe',()=>events.push('window'));document.addEventListener('probe',()=>events.push('document'));
 document.open();old.dispatchEvent(new Event('probe'));detached.dispatchEvent(new Event('probe'));window.dispatchEvent(new Event('probe'));document.dispatchEvent(new Event('probe'));
 if(events.join(',')!=='detached')return 'listeners:'+events;
 document.addEventListener('DOMContentLoaded',()=>events.push('dom'));window.addEventListener('load',()=>events.push('load'));
 document.write('<script>document.open();document.close();events.push(document.readyState)</scr'+'ipt><b id=tail>tail</b>');
 if(document.readyState!=='complete'||!document.getElementById('tail'))return 'nested-close';
 return events.join(',')==='detached,loading,dom,load'?true:events.join(',');
})()`+"`"+`);
})()`, true)
	})
}

func TestDocumentStreamResetsHandlerAttributesAndRegistration(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const log=[],button=document.createElement('button'),script=document.createElement('script'),image=document.createElement('img'),detached=document.createElement('button');
document.body.append(button,script,image);
const cases=[[window,'click'],[document,'click'],[button,'click'],[script,'load'],[image,'load']];
for(const [target,type] of cases){target['on'+type]=()=>log.push('old');target.addEventListener(type,()=>log.push('old-listener'))}
const retained=()=>log.push('detached');detached.onclick=retained;
let accessorCalls=0;Object.defineProperty(window,'onmessage',{get(){accessorCalls++;return null},set(){accessorCalls++},configurable:true});
document.open();
if(accessorCalls!==0||detached.onclick!==retained)throw new Error('public accessor or detached handler touched');
for(const [target,type] of cases){
 if(target['on'+type]!==null)throw new Error('stale handler value');
 target.dispatchEvent(new Event(type));
 target.addEventListener(type,()=>log.push('before'));
 target['on'+type]=()=>log.push('new');
 target.addEventListener(type,()=>log.push('after'));
 target.dispatchEvent(new Event(type));
}
detached.dispatchEvent(new Event('click'));
return log.join(',')===Array(5).fill('before,new,after').join(',')+',detached';
})()`, true)
	})
}

func TestDocumentStreamExternalScriptSuspendsParser(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/script.js" {
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				w.Header().Set("Content-Type", "text/javascript")
				fmt.Fprint(w, `events.push('external');document.write('<p id=inserted>inserted</p>')`)
				return
			}
			fmt.Fprint(w, "<!doctype html><body>parent</body>")
		}))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);globalThis.streamFrame=f;
 const d=f.contentDocument;f.contentWindow.eval('globalThis.events=[]');d.open();
 globalThis.streamDone=new Promise(resolve=>f.onload=()=>resolve(f.contentWindow.eval("events.join(',')==='external,tail'&&Array.from(document.querySelectorAll('p')).map(n=>n.id).join(',')==='inserted,first,second'")));
 d.write('<script src="/script.js"></scr'+'ipt><p id=first>first</p><script>events.push("tail")</scr'+'ipt>');
 d.write('<p id=second>second</p>');d.close();
 return d.readyState==='loading'&&d.getElementById('first')===null&&d.getElementById('second')===null;
})()`, true)
		close(release)
		historyEval(t, p, "streamDone", true)
	})
}

func TestDocumentStreamOpenCancelsPausedScript(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		started, canceled := make(chan struct{}), make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/slow.js" {
				close(started)
				<-r.Context().Done()
				close(canceled)
				return
			}
			fmt.Fprint(w, "<!doctype html><body>parent</body>")
		}))
		defer server.Close()
		defer p.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{const f=document.createElement('iframe');document.body.appendChild(f);globalThis.streamFrame=f;const d=f.contentDocument;d.open();d.write('<script src="/slow.js"></scr'+'ipt><p id=old>old</p>');d.close();return d.readyState==='loading'})()`, true)
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("stream script did not start")
		}
		historyEval(t, p, `(()=>{const d=streamFrame.contentDocument;d.open();d.write('<p id=new>new</p>');d.close();return d.readyState==='complete'&&d.getElementById('old')===null&&d.getElementById('new').textContent==='new'})()`, true)
		select {
		case <-canceled:
		case <-time.After(10 * time.Second):
			t.Fatal("superseded stream script request retained")
		}
	})
}
