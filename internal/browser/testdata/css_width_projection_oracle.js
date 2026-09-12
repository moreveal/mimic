(()=>{
 const out={},body=document.body;body.replaceChildren();body.style.cssText='margin:0;width:800px';
 const read=e=>[e.offsetWidth,e.clientWidth,getComputedStyle(e).width,e.getBoundingClientRect().width];
 const add=(name,style,parent=body,tag='div')=>{const e=document.createElement(tag);e.style.cssText=style;parent.append(e);out[name]=read(e);return e};
 add('content','width:100px;padding:3px;border:2px solid');
 add('border','box-sizing:border-box;width:100px;padding:3px;border:2px solid');
 add('fractional-content','width:100.5px;padding:0.25px;border:1px solid');
 const parent=add('parent','width:200px;padding:5px;border:2px solid');
 const child=add('percent','width:50%;padding:3px;border:2px solid',parent);
 child.style.width='25%';out.mutation=read(child);parent.style.width='400px';out.parentMutation=read(child);
 add('minimum','width:10px;min-width:40px;max-width:80px');
 add('maximum','width:200px;min-width:40px;max-width:80px');
 const hidden=add('hidden','display:none;width:100px;padding:3px;border:2px solid');
 add('hidden-child','width:50px',hidden);
 const detached=document.createElement('div');detached.style.width='80px';out.detached=read(detached);
 const inline=add('empty-inline','display:inline');out['empty-inline']=[inline.offsetWidth,inline.clientWidth,inline.getBoundingClientRect().width];add('inline-block','display:inline-block;width:75px;padding:3px;border:1px solid');
 const table=add('table','width:200px;border-spacing:0',body,'table');
 table.innerHTML='<tbody><tr><td style="width:80px;padding:0">a</td><td style="padding:0">b</td></tr></tbody>';out.cell=read(table.querySelector('td'));
 add('replaced','width:60px;height:20px;padding:2px;border:1px solid',body,'img');
 const host=add('shadow-host','width:150px');const shadow=host.attachShadow({mode:'open'});shadow.innerHTML='<div style="width:80px"><slot></slot></div>';
 const slotted=add('slotted','width:40px',host);out.shadow=read(shadow.firstChild);
 slotted.slot='absent';out.unassigned=read(slotted);
 return out;
})()
