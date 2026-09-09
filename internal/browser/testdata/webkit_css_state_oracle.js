(()=>{
 const element=document.createElement('div'),s=element.style,out={};
 const prepare=()=>{s.webkitFlex='var(--choice)';s.flexGrow='2'};
 const read=style=>[style.cssText,style.length,Array.from({length:style.length},(_,i)=>style.item(i))];
 prepare();out.before=read(s);out.clone=read(element.cloneNode().style);
 element.setAttribute('style',element.getAttribute('style'));out.attribute=read(s);
 prepare();s.cssText=s.cssText;out.cssText=read(s);
 prepare();element.setAttributeNS(null,'style',element.getAttribute('style'));out.namespace=read(s);
 prepare();element.getAttributeNode('style').value=element.getAttribute('style');out.attr=read(s);
 prepare();element.removeAttribute('style');out.removed=read(s);
 class StyleProbe extends HTMLElement{static get observedAttributes(){return ['style']}attributeChangedCallback(){out.reaction=read(this.style)}}
 customElements.define('webkit-style-state-probe',StyleProbe);const probe=document.createElement('webkit-style-state-probe');probe.style.webkitFlex='var(--choice)';probe.style.flexGrow='2';out.custom=read(probe.style);
 return out;
})()
