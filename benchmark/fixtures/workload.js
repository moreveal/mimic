/* Deterministic browser workloads. Timings never participate in correctness. */
'use strict';
window.__bench = {done:false, result:null, error:null};
window.__benchRun = async function () {
  const kind = document.body.getAttribute('data-workload');
  const start = performance.now();
  try {
    if (localStorage.getItem('benchmark-used') !== null) throw Error('storage leaked');
    localStorage.setItem('benchmark-used', 'yes');
    let result;
    if (kind === 'static') {
      result = {text:document.querySelector('#root').textContent, count:document.querySelectorAll('#root').length};
    } else if (kind === 'cpu') {
      let sum=0;
      for(let round=0;round<30;round++) {
        const a=Array.from({length:4000},(_,i)=>({id:i,value:'item-'+i,n:(i*17)%997}));
        const b=JSON.parse(JSON.stringify(a));
        const m=new Map(b.map(x=>[x.id,x]));
        for(const x of m.values()) if(/^item-\d+$/.test(x.value)) sum=(sum+x.n)>>>0;
        await Promise.resolve();
      }
      const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(String(sum)));
      result={sum,digest:Array.from(new Uint8Array(digest),x=>x.toString(16).padStart(2,'0')).join('')};
    } else if (kind === 'dom') {
      const root=document.querySelector('#root'); root.innerHTML='';
      const frag=document.createDocumentFragment();
      for(let i=0;i<3000;i++) {
        const e=document.createElement('div'); e.setAttribute('data-i',String(i));
        e.className='item'; e.appendChild(document.createTextNode('node-'+i));
        e.appendChild(document.createComment('c')); frag.appendChild(e);
      }
      root.appendChild(frag);
      for(let round=0;round<4;round++) for(const e of root.querySelectorAll('.item')) {
        e.classList.toggle('active',round%2===1); e.setAttribute('data-round',String(round));
      }
      const extra=document.createElement('section'); extra.innerHTML='<b>end</b><!--tail-->text';root.appendChild(extra);
      result={count:root.querySelectorAll('.item').length,active:root.querySelectorAll('.active').length,
        last:root.querySelector('[data-i="2999"]').textContent,children:extra.childNodes.length,
        round:root.querySelector('.item').getAttribute('data-round'),textLength:root.textContent.length};
    } else if (kind === 'async') {
      window.__bench.phase='promises';
      let chain=0; for(let i=0;i<100;i++) await Promise.resolve().then(()=>chain++);
      window.__bench.phase='microtask';
      await new Promise(r=>queueMicrotask(()=>{chain++;r()}));
      window.__bench.phase='timer';
      const timer=await new Promise(r=>setTimeout(()=>r(17),10));
      window.__bench.phase='channel';
      const channel=await new Promise(r=>{const c=new MessageChannel();c.port1.onmessage=e=>{r(e.data);c.port1.close();c.port2.close()};c.port2.postMessage(23)});
      window.__bench.phase='worker';
      const worker=await new Promise((r,j)=>{const w=new Worker('/worker.js');w.onmessage=e=>{r(e.data);w.terminate()};w.onerror=()=>j(Error('worker error'));w.postMessage(31)});
      window.__bench.phase='postMessage';
      const posted=await new Promise(r=>{function f(e){if(e.data==='bench-message'){removeEventListener('message',f);r(e.data)}}addEventListener('message',f);postMessage('bench-message','*')});
      window.__bench.phase='fetch';
      const fetched=await (await fetch('/data.json')).json();
      window.__bench.phase='xhr';
      const xhr=await new Promise((r,j)=>{const x=new XMLHttpRequest();x.open('GET','/data.json');x.onload=()=>r(JSON.parse(x.responseText));x.onerror=j;x.send()});
      result={chain,timer,channel,worker,posted,fetch:fetched.value,xhr:xhr.value};
    } else if (kind === 'react') {
      window.__bench.phase='fetch';
      const data=await (await fetch('/data.json')).json();
      window.__bench.phase='render';
      const h=React.createElement;
      function App(){const [phase,setPhase]=React.useState(0);React.useEffect(()=>{if(phase<3)Promise.resolve().then(()=>setPhase(phase+1));else window.__reactReady=true},[phase]);
        return h('main',{'data-phase':String(phase)},Array.from({length:200},(_,i)=>h('article',{key:i,className:i%2?'odd':'even'},h('strong',null,'Task '+i),h('span',null,String(i+phase+data.value)))));}
      ReactDOM.createRoot(document.querySelector('#root')).render(h(App));
      await new Promise(r=>{function check(){if(window.__reactReady)r();else setTimeout(check,1)}check()});
      result={count:document.querySelectorAll('article').length,phase:document.querySelector('main').getAttribute('data-phase'),first:document.querySelector('article').textContent,last:document.querySelectorAll('article')[199].textContent};
    } else if (kind === 'wasm') {
      const bytes=new Uint8Array([0,97,115,109,1,0,0,0,1,7,1,96,2,127,127,1,127,3,2,1,0,7,7,1,3,97,100,100,0,0,10,9,1,7,0,32,0,32,1,106,11]);
      const m=await WebAssembly.instantiate(bytes);let sum=0;for(let i=0;i<100000;i++)sum=m.instance.exports.add(sum,i);result={sum};
    } else throw Error('unknown workload '+kind);
    window.__bench={done:true,result,error:null,js_ms:performance.now()-start};
  } catch(e) { window.__bench={done:true,result:null,error:String(e),js_ms:performance.now()-start}; }
};
