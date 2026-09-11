(async()=>{
const out={},caught=f=>{try{f();return 'ok'}catch(e){return e.name}},request=r=>new Promise((resolve,reject)=>{r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)}),done=t=>new Promise((resolve,reject)=>{t.oncomplete=resolve;t.onabort=()=>reject(t.error)});
const open=indexedDB.open('lifecycle',1);open.onupgradeneeded=()=>{open.result.createObjectStore('s');open.result.createObjectStore('inline',{keyPath:'a.b',autoIncrement:true})};const db=await request(open);
let t=db.transaction('s','readwrite'),finish=done(t),s=t.objectStore('s');
out.invalidKeys=[undefined,null,true,NaN,{},[1,,2],new Date(NaN),1n].map(k=>caught(()=>s.put('x',k)));
let traps=0;const proxy=new Proxy({x:1},{ownKeys(){traps++;return ['x']}});out.cloneErrors=[caught(()=>s.put(()=>{},1)),caught(()=>s.put(Symbol(),1)),caught(()=>s.put(proxy,1)),traps];
const cyc=[];cyc.push(cyc);out.cyclicKey=caught(()=>indexedDB.cmp(cyc,0));
await request(s.put('infinity',Infinity));await request(s.put('negative',-Infinity));out.infinity=[await request(s.get(Infinity)),await request(s.get(-Infinity))];await finish;
out.completed=[caught(()=>s.get(1)),caught(()=>t.abort()),caught(()=>t.objectStore('s'))];
t=db.transaction('s','readwrite');finish=done(t);s=t.objectStore('s');const req=s.put(1,'one');const phases=[];req.addEventListener('success',()=>{phases.push('listener');Promise.resolve().then(()=>{phases.push('micro');s.put(2,'two')})});req.addEventListener('success',()=>phases.push('second'));await finish;out.microtasks=phases;
t=db.transaction('s','readwrite');s=t.objectStore('s');const tx1=t,fin1=done(t);s.put('first','serial');const tx2=db.transaction('s','readwrite'),fin2=done(tx2);tx2.objectStore('s').put('second','serial');await Promise.all([fin1,fin2]);t=db.transaction('s');finish=done(t);out.serial=await request(t.objectStore('s').get('serial'));await finish;
t=db.transaction('inline','readwrite');finish=done(t);s=t.objectStore('inline');out.inline=[await request(s.add({})),await request(s.get(1))];await finish;
const logs=[];const r=indexedDB.open('lifecycle',2);db.onversionchange=e=>{logs.push(['versionchange',e.oldVersion,e.newVersion]);};r.onblocked=e=>{logs.push(['blocked',e.oldVersion,e.newVersion]);db.close()};r.onupgradeneeded=()=>{logs.push(['upgradeneeded']);const x=r.result.createObjectStore('renamed');x.name='name2';x.createIndex('x','x').name='y';logs.push([Array.from(r.result.objectStoreNames),Array.from(x.indexNames)]);};const db2=await request(r);out.blocked=logs;
t=db2.transaction('s','readwrite');s=t.objectStore('s');const fin=done(t);out.commit=caught(()=>{s.put('commit','commit');t.commit();s.put('late','late')});await fin;
t=db2.transaction('s');finish=done(t);s=t.objectStore('s');out.cursorOrder=await new Promise((resolve,reject)=>{const a=[],q=s.openKeyCursor(null,'prev');q.onerror=()=>reject(q.error);q.onsuccess=()=>{const c=q.result;if(!c){resolve(a);return}a.push([String(c.key),caught(()=>c.delete())]);c.continue()}});await finish;
db2.close();await request(indexedDB.deleteDatabase('lifecycle'));return out;
})()
