(()=>{
 const out={},body=document.body;body.replaceChildren();body.style.cssText='margin:0;width:800px';
 const read=e=>[e.offsetHeight,e.clientHeight,getComputedStyle(e).height,e.getBoundingClientRect().height];
 const add=(name,style,parent=body,tag='div')=>{const e=document.createElement(tag);e.style.cssText=style;parent.append(e);out[name]=read(e);return e};
 add('content','height:100px;padding:3px;border:2px solid');
 add('border','box-sizing:border-box;height:100px;padding:3px;border:2px solid');
 add('fractional-content','height:100.5px;padding:0.25px;border:1px solid');
 const parent=add('parent','height:200px;width:200px;position:relative');
 const child=add('percent','height:50%;padding:3px;border:2px solid',parent);
 child.style.height='25%';out.mutation=read(child);parent.style.height='400px';out.parentMutation=read(child);
 add('minimum','height:10px;min-height:40px;max-height:80px');
 add('maximum','height:200px;min-height:40px;max-height:80px');
 const auto=add('auto-empty','width:100px;padding:3px;border:2px solid');
 auto.innerHTML='<div style="height:14px"></div>';out.autoChild=read(auto);
 auto.firstChild.style.height='24px';out.autoChildMutation=read(auto);
 add('absolute','position:absolute;top:10px;bottom:20px;width:100px',parent);
 const hidden=add('hidden','display:none;height:100px;padding:3px;border:2px solid');
 add('hidden-child','height:50px',hidden);
 const detached=document.createElement('div');detached.style.height='80px';out.detached=read(detached);
 add('inline-block','display:inline-block;height:75px;width:40px;padding:3px;border:1px solid');
 const table=add('table','width:200px;border-spacing:0',body,'table');
 table.innerHTML='<tbody><tr><td style="height:20px;padding:0">a</td><td style="height:30px;padding:0">b</td></tr></tbody>';out.cell=read(table.querySelector('td'));
 add('replaced','width:60px;height:20px;padding:2px;border:1px solid',body,'img');
 // Pin author box sizing: the existing UA default button model is a separate
 // compatibility boundary, unchanged by this dimension projection.
 add('button','box-sizing:content-box;height:20px;padding:2px;border:1px solid',body,'button');
 const host=add('shadow-host','height:150px');const shadow=host.attachShadow({mode:'open'});shadow.innerHTML='<div style="height:80px"><slot></slot></div>';
 const slotted=add('slotted','height:40px',host);out.shadow=read(shadow.firstChild);
 slotted.slot='absent';out.unassigned=read(slotted);
 const frame=document.createElement('iframe');frame.style.cssText='height:200px;width:220px';body.append(frame);
 const own=add('foreign-target','height:30px;padding:3px;border:2px solid');
 const foreignGet=Object.getOwnPropertyDescriptor(frame.contentWindow.HTMLElement.prototype,'offsetHeight').get;
 out.foreign=foreignGet.call(own);own.style.height='40px';out.foreignMutation=foreignGet.call(own);
 const adopted=document.createElement('div');adopted.style.cssText='height:50px;padding:2px;border:1px solid';body.append(adopted);
 frame.contentDocument.body.append(adopted);out.adopted=read(adopted);frame.remove();out.removedOwner=read(adopted);
 return out;
})()
