(()=>{
const out={};
for(const name of ['translate','draggable','spellcheck']){
 const rows=[];out[name]=rows;
 for(const tag of ['div','a','img','input','textarea'])for(const parentValue of [null,'true','false','yes','no']){
  const parent=document.createElement('div'),e=document.createElement(tag);parent.appendChild(e);if(parentValue!==null)parent.setAttribute(name,parentValue);
  for(const value of [null,'','true','false','yes','no','TRUE','FALSE','invalid',' true ']){if(value===null)e.removeAttribute(name);else e.setAttribute(name,value);rows.push([tag,parentValue,value,e[name]])}
 }
 const e=document.createElement('div');out[name+'Set']=[];
 for(const value of [true,false,null,undefined,0,1,'false','',{}]){e[name]=value;out[name+'Set'].push([e[name],e.getAttribute(name)])}
 const d=Object.getOwnPropertyDescriptor(HTMLElement.prototype,name);let illegal;try{d.get.call({})}catch(e){illegal=e.name}out[name+'Descriptor']=[d.get.name,d.set.name,d.enumerable,d.configurable,illegal];
}
const a=document.createElement('a');out.link=[a.draggable];a.setAttribute('href','');out.link.push(a.draggable);a.setAttribute('draggable','invalid');out.link.push(a.draggable);
out.inputTypes=[];for(const type of ['text','password','email','search','number','url']){const e=document.createElement('input');e.type=type;out.inputTypes.push([type,e.spellcheck]);e.setAttribute('spellcheck','true');out.inputTypes.push([type,'explicit',e.spellcheck]);}
out.inheritance=[];for(const name of ['translate','spellcheck']){const host=document.createElement('div');host.setAttribute(name,name==='translate'?'no':'false');const child=document.createElement('span');const root=host.attachShadow({mode:'open'});root.appendChild(child);out.inheritance.push([name,child[name]]);host.appendChild(child);out.inheritance.push([name,child[name]]);child.remove();out.inheritance.push([name,child[name]]);}
return out})()
