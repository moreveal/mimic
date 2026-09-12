(async()=>{
 const out={},make=(d=document)=>{const e=d.createElement('div');e.style.cssText='display:flex;color:rgb(1, 2, 3);width:23px;opacity:0.5';return e},read=s=>({display:s.display,color:s.color,width:s.width,opacity:s.opacity,empty:s.length===0,itemEmpty:s.item(0)==='',indexAbsent:s[0]===undefined,hasIndex:0 in s,value:s.getPropertyValue('display'),cssText:s.cssText,priority:s.getPropertyPriority('display')});
 const e=make(),s=getComputedStyle(e);out.detached=read(s);document.body.append(e);out.connected=read(s);e.remove();out.removed=read(s);document.body.append(e);out.reconnected=read(s);
 e.style.setProperty('display','flex','important');out.computedPriority=s.getPropertyPriority('display');out.inlinePriority=e.style.getPropertyPriority('display');
 for(const name of ['isConnected','ownerDocument'])Object.defineProperty(e,name,{configurable:true,get(){throw new Error('author getter')}});out.authorGetters=read(s);for(const name of ['isConnected','ownerDocument'])delete e[name];
 e.style.display='none';out.selfNone=read(s);e.style.display='flex';const p=document.createElement('div');p.style.display='none';document.body.append(p);p.append(e);out.ancestorNone=read(s);p.remove();out.detachedTree=read(s);
 const frag=document.createDocumentFragment();frag.append(e);out.fragment=read(s);document.body.append(frag);out.insertedFragment=read(s);
 const other=document.implementation.createHTMLDocument('inactive');other.body.append(e);out.inactiveDocument=read(s);document.body.append(e);out.readopted=read(s);
 const host=document.createElement('div'),shadow=host.attachShadow({mode:'open'}),se=make();shadow.append(se);const ss=getComputedStyle(se);out.detachedShadow=read(ss);document.body.append(host);out.connectedShadow=read(ss);host.remove();out.removedShadow=read(ss);
 const frame=document.createElement('iframe');document.body.append(frame);const w=frame.contentWindow,d=w.document,fe=make(d);d.body.append(fe);const fs=w.getComputedStyle(fe);out.frameConnected=read(fs);out.borrowedStyle=read(getComputedStyle(fe));out.reverseBorrow=read(w.getComputedStyle(e));frame.style.display='none';out.frameHidden=read(fs);frame.style.display='block';out.frameShown=read(fs);frame.style.visibility='hidden';out.frameInvisible=read(fs);frame.remove();out.frameRemoved=read(fs);
 for(const [name,args] of [['missing',[]],['null',[null]],['text',[document.createTextNode('x')]],['object',[{}]]])try{getComputedStyle(...args);out[name]='ok'}catch(e){out[name]=e.name}
 e.remove();return out;
})()
