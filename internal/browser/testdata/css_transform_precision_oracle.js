(async()=>{
 const out={},ns='http://www.w3.org/2000/svg';
 const svg=document.createElementNS(ns,'svg');svg.setAttribute('width','200');svg.setAttribute('height','100');svg.style.cssText='position:absolute;left:0;top:0';document.body.appendChild(svg);
 const text=document.createElementNS(ns,'text');text.setAttribute('x','32');text.setAttribute('y','32');text.textContent='😀';svg.appendChild(text);
 const read=()=>{const m=text.getCTM(),r=text.getExtentOfChar(0);return {specified:text.style.transform,computed:getComputedStyle(text).transform,matrix:[m.a,m.b,m.c,m.d,m.e,m.f],length:text.getComputedTextLength(),character:[r.x,r.y,r.width,r.height]}};
 for(const value of ['scale(1.000998)','scale(1.23456789)','scale(0.999876543)','matrix(1.000998,0,0,1.000998,0.123456789,0.987654321)','rotate(10.123456789deg)','rotate(1.23456789123deg)','rotate(123.456789123deg)','rotate(0.123456789123rad)','rotate(-123.456789123deg)','rotate(10.123456789deg) scale(1)','rotate(1.0123456789e1deg)','translate(0.123456789px,0.987654321px) rotate(10.123456789deg)','skewX(10.123456789deg)','scale(calc(1.0004 + 0.000598))']){text.style.transform=value;out[value]=read();}
 text.style.transform='scale(1.000998)';text.style.color='red';out.unrelatedMutation=read();
 const clone=text.cloneNode(true);svg.appendChild(clone);out.clone=[clone.style.transform,clone.getCTM().a,clone.getComputedTextLength()];clone.remove();
 text.setAttribute('style',text.getAttribute('style'));out.sameAttribute=read();
 text.style.transform=text.style.transform;out.serializedReparse=read();
 text.style.cssText='transform:scale(1.000998)';out.cssText=read();
 text.style.removeProperty('transform');out.removed=read();
 const style=document.createElement('style');style.textContent='.precision {transform:scale(1.000998)}';document.head.appendChild(style);text.setAttribute('class','precision');out.rule=read();
 style.sheet.cssRules[0].style.setProperty('transform','scale(1.23456789)');out.ruleMutation=read();
 text.style.transform='scale(0.999876543)';out.inlineWins=read();
 text.style.removeProperty('transform');out.ruleRestored=read();
 style.sheet.cssRules[0].style.transform='rotate(10.123456789deg)';out.ruleRotation=read();
 const adopted=new CSSStyleSheet();adopted.replaceSync('.precision {transform:scale(1.000998)}');document.adoptedStyleSheets=[adopted];out.adopted=read();document.adoptedStyleSheets=[];
 const d=document.createElement('div');d.style.cssText='position:absolute;left:0;top:0;width:100px;height:20px;transform-origin:0 0';document.body.appendChild(d);
 for(const value of ['scale(1.000998)','skewX(10.123456789deg)']){d.style.transform=value;const r=d.getBoundingClientRect();out['html-'+value]={specified:d.style.transform,computed:getComputedStyle(d).transform,rect:[r.x,r.y,r.width,r.height]};}
 for(const family of ['Times New Roman','Arial Black, serif','Arial, sans-serif']){d.style.fontFamily=family;out['family-'+family]=[d.style.fontFamily,getComputedStyle(d).fontFamily];}
 d.remove();style.remove();svg.remove();return out;
})()
