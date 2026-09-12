// Shared computed font-size resolution. No layout, rasterization or persistent
// element cache: mutations and adoption are observed on the next query.
const cssFontParent=n=>{const parent=syntheticParents.get(n)||(elementSlot(n)?wrap(host.parentNode(elementSlot(n).nodeId)):null);return shadowSlots.get(parent)?.host||parent||shadowSlots.get(n)?.host||null};
const cssResolveLength=(input,context)=>{
 const value=String(input).trim().toLowerCase();
 if(cssNumberRegex.test(value))return Number(value)===0||context.unitless?Number(value):null;
 const term=cssLengthTerm(value);
 if(term){const scales={px:1,pt:96/72,pc:16,in:96,cm:96/2.54,mm:96/25.4,q:96/101.6,em:context.em,rem:context.rem,'%':context.percent/100};const scale=scales[term.unit];return Number.isFinite(scale)?term.number*scale:null}
 const calc=/^calc\(\s*(\S+)\s+([+-])\s+(\S+)\s*\)$/.exec(value);
 if(calc){const a=cssResolveLength(calc[1],context),b=cssResolveLength(calc[3],context);return a===null||b===null?null:a+(calc[2]==='-'?-b:b)}
 return null;
};
const cssComputedFontSize=element=>{
 const chain=[];for(let n=element;elementSlot(n)?.type==='element';n=cssFontParent(n)){if(chain.length>=256){host.semanticMissingAt('css_font_metrics.js:14','CSS.fontSizeResolution',JSON.stringify({reason:'ancestor-limit'}));return null;}chain.push(n)}
 let size=16,rootSize=16;
 for(let i=chain.length-1;i>=0;i--){const n=chain[i],entries=computedCSSDeclarations(n);let raw=entries.find(e=>e.name==='font-size')?.value;const source=raw==null?'presentation-or-inherited':'css';
  if(raw==null&&elementSlot(n).namespaceURI==='http://www.w3.org/2000/svg')raw=host.getAttribute(elementSlot(n).nodeId,'font-size');
  const specified=String(raw??'inherit');raw=specified.trim().toLowerCase();
  if(raw==='inherit'||raw==='unset'||raw===''){}else if(raw==='initial')size=16;
  else {const keywords={'xx-small':9,'x-small':10,small:13,medium:16,large:18,'x-large':24,'xx-large':32,'xxx-large':48};
   const next=Object.prototype.hasOwnProperty.call(keywords,raw)?keywords[raw]:raw==='smaller'?size/1.2:raw==='larger'?size*1.2:cssResolveLength(raw,{em:size,rem:i===chain.length-1?16:rootSize,percent:size,unitless:elementSlot(n).namespaceURI==='http://www.w3.org/2000/svg'});
   if(next===null||!Number.isFinite(next)||next<0){host.semanticMissingAt('css_font_metrics.js:22','CSS.fontSizeResolution',JSON.stringify({reason:next===null?'unsupported-expression':!Number.isFinite(next)?'nonfinite-result':'negative-result',value:raw.slice(0,256),specified:specified.slice(0,256),source,rootBasis:i===chain.length-1?16:rootSize,ancestorIndex:chain.length-1-i,targetNodeId:elementSlot(element).nodeId,nodeId:elementSlot(n).nodeId,tag:elementSlot(n).qualifiedName||elementSlot(n).tagName,parentSize:size,rootSize}));return null;}size=next;
  }
  if(i===chain.length-1)rootSize=size;
 }
 return size;
};
