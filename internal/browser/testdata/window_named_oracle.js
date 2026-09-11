(() => {
 const out={},cap=f=>{try{return {value:f()}}catch(e){return {error:e.name}}},tag=v=>v===undefined?'undefined':v===null?'null':v===window?'window':v?.nodeType?'node:'+v.localName:Object.prototype.toString.call(v);
 const container=document.createElement('div');document.body.appendChild(container);
 for(const name of ['form','img','embed','object','div','input','iframe']){
  const e=document.createElement(name);e.id='probe_id_'+name;e.setAttribute('name','probe_name_'+name);container.appendChild(e);
  out[name]={windowId:tag(window[e.id]),windowName:tag(window['probe_name_'+name])};
 }
 const p=Object.getPrototypeOf(Window.prototype);out.layer={tag:tag(p),parent:tag(Object.getPrototypeOf(p)),keys:Reflect.ownKeys(p).map(String)};
 const desc=(o,k)=>{const d=Object.getOwnPropertyDescriptor(o,k);return d?{value:tag(d.value),get:typeof d.get,set:typeof d.set,w:d.writable,e:d.enumerable,c:d.configurable}:null};
 out.descriptors={window:desc(window,'probe_name_form'),layer:desc(p,'probe_name_form')};
 const a=document.createElement('form'),b=document.createElement('form');a.name=b.name='probe_dupe';container.append(a,b);
 out.dupe={window:tag(window.probe_dupe),stableWindow:window.probe_dupe===window.probe_dupe};
 out.shadow={set:Reflect.set(window,'probe_dupe',7),value:window.probe_dupe,del:Reflect.deleteProperty(window,'probe_dupe')};out.restored=tag(window.probe_dupe);container.remove();out.removed={window:tag(window.probe_dupe)};
 return out;
})()
