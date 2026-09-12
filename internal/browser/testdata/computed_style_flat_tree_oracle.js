(()=>{
 const out={},node=()=>{const e=document.createElement('div');e.style.cssText='display:flex;color:rgb(1, 2, 3);width:20px';return e},read=e=>{const s=getComputedStyle(e);return[s.display,s.color,s.length===0,s.item(0)==='',s[0]===undefined]};
 for(const mode of ['open','closed']){
  const h=node(),r=h.attachShadow({mode}),e=node(),child=node();e.append(child);h.append(e);document.body.append(h);const live=getComputedStyle(e);
  out[mode+'-unassigned']=[read(e),read(child),e.isConnected];
  const slot=document.createElement('slot'),fallback=node();slot.append(fallback);r.append(slot);
  out[mode+'-assigned']=[read(e),read(child),read(fallback),live.display];
  slot.name='named';out[mode+'-unmatched']=[read(e),read(child),read(fallback),live.display];
  e.slot='named';out[mode+'-attrs']=[e.getAttribute('slot'),slot.getAttribute('name'),e.slot,slot.name];out[mode+'-named']=[read(e),read(child),read(fallback),live.display];
  const duplicate=document.createElement('slot'),df=node();duplicate.name='named';duplicate.append(df);r.append(duplicate);out[mode+'-duplicate']=[read(e),read(fallback),read(df)];
  r.insertBefore(duplicate,slot);out[mode+'-reorder']=[read(e),read(fallback),read(df)];
  duplicate.remove();slot.remove();out[mode+'-slot-removed']=[read(e),live.display];
  document.body.append(e);out[mode+'-moved']=[read(e),read(child),live.display];h.remove();e.remove();
 }
 const outer=node(),root=outer.attachShadow({mode:'closed'}),inner=node(),innerRoot=inner.attachShadow({mode:'open'}),leaf=node();innerRoot.append(leaf);outer.append(inner);document.body.append(outer);out.nestedSuppressed=read(leaf);root.append(document.createElement('slot'));out.nestedAssigned=read(leaf);outer.remove();
 const frame=document.createElement('iframe');document.body.append(frame);const w=frame.contentWindow,fh=w.document.createElement('div'),fr=fh.attachShadow({mode:'closed'}),fe=w.document.createElement('div');fe.style.display='flex';fh.append(fe);w.document.body.append(fh);out.foreignUnassigned=[getComputedStyle(fe).display,w.getComputedStyle(fe).display];fr.append(w.document.createElement('slot'));out.foreignAssigned=[getComputedStyle(fe).display,w.getComputedStyle(fe).display];frame.remove();
 const host=node(),r=host.attachShadow({mode:'closed'}),light=node();host.append(light);document.body.append(host);const f=document.createElement('iframe');document.body.append(f);out.borrowedForeignGetter=f.contentWindow.getComputedStyle(light).display;f.remove();host.remove();
 return out;
})()
