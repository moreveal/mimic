(()=>{
 const element=document.createElement('div');document.body.append(element);
 const style=element.style,computed=getComputedStyle(element);
 const names=Object.getOwnPropertyNames(style).filter(n=>/webkit/i.test(n)).sort();
 for(let p=Object.getPrototypeOf(style);p;p=Object.getPrototypeOf(p))for(const n of Object.getOwnPropertyNames(p))if(/webkit/i.test(n)&&!names.includes(n))names.push(n);
 const properties={};
 for(const name of names.sort()){
  style.cssText='';style[name]='initial';
  properties[name]={cssText:style.cssText,length:style.length,item:style.item(0),value:style[name],computed:computed[name],supported:style.length?CSS.supports(style.item(0),'initial'):false};
 }
 element.remove();return {properties};
})()
