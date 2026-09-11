// Component membership/order measured against Chrome 152; see webkit-css-shorthands oracle.
const cssShorthandComponents={
 "font": ["font-style","font-variant-caps","font-variant-ligatures","font-variant-numeric","font-variant-east-asian","font-variant-alternates","font-size-adjust","font-language-override","font-kerning","font-optical-sizing","font-feature-settings","font-variation-settings","font-variant-position","font-variant-emoji","font-weight","font-stretch","font-size","line-height","font-family"],
 "animation": [
  "animation-duration",
  "animation-timing-function",
  "animation-delay",
  "animation-iteration-count",
  "animation-direction",
  "animation-fill-mode",
  "animation-play-state",
  "animation-name",
  "animation-timeline",
  "animation-range-start",
  "animation-range-end"
 ],
 "border-block-end": [
  "border-block-end-width",
  "border-block-end-style",
  "border-block-end-color"
 ],
 "border-block-start": [
  "border-block-start-width",
  "border-block-start-style",
  "border-block-start-color"
 ],
 "border-inline-end": [
  "border-inline-end-width",
  "border-inline-end-style",
  "border-inline-end-color"
 ],
 "border-radius": [
  "border-top-left-radius",
  "border-top-right-radius",
  "border-bottom-right-radius",
  "border-bottom-left-radius"
 ],
 "border-inline-start": [
  "border-inline-start-width",
  "border-inline-start-style",
  "border-inline-start-color"
 ],
 "column-rule": [
  "column-rule-width",
  "column-rule-style",
  "column-rule-color"
 ],
 "columns": [
  "column-width",
  "column-count",
  "column-height",
  "column-wrap"
 ],
 "flex": [
  "flex-grow",
  "flex-shrink",
  "flex-basis"
 ],
 "flex-flow": [
  "flex-direction",
  "flex-wrap"
 ],
 "mask": [
  "mask-image",
  "-webkit-mask-position-x",
  "-webkit-mask-position-y",
  "mask-size",
  "mask-repeat",
  "mask-origin",
  "mask-clip",
  "mask-composite",
  "mask-mode"
 ],
 "-webkit-mask-box-image": [
  "-webkit-mask-box-image-source",
  "-webkit-mask-box-image-slice",
  "-webkit-mask-box-image-width",
  "-webkit-mask-box-image-outset",
  "-webkit-mask-box-image-repeat"
 ],
 "mask-position": [
  "-webkit-mask-position-x",
  "-webkit-mask-position-y"
 ],
 "text-emphasis": [
  "text-emphasis-style",
  "text-emphasis-color"
 ],
 "-webkit-text-stroke": [
  "-webkit-text-stroke-width",
  "-webkit-text-stroke-color"
 ],
 "transition": [
  "transition-property",
  "transition-duration",
  "transition-timing-function",
  "transition-delay",
  "transition-behavior"
 ]
};
const cssWideValue=value=>['initial','inherit','unset','revert','revert-layer'].includes(value);
const expandCSSDeclaration=entry=>{
 const components=cssShorthandComponents[entry.name];if(!components)return [entry];
 if(cssWideValue(entry.value))return components.map(name=>({...entry,name}));
 if(/^var\(/.test(entry.value))return components.map(name=>({name,value:'',priority:entry.priority,pending:{name:entry.name,value:entry.value}}));
 if(entry.name==='font'){
  const values=parseCSSFont(entry.value);
  if(!values){host.semanticMissingAt('css_shorthands.js/font','CSS.fontShorthandResolution');return [entry]}
  return components.map((name,i)=>({name,value:values[i],priority:entry.priority}));
 }
 const parser=cssShorthandParsers.get(entry.name);
 if(parser){const values=parser(entry.value);return values?(cssOrdinaryShorthandOrder[entry.name]||components).map(name=>({name,value:values[components.indexOf(name)],priority:entry.priority})):[]}
 return [entry];
};
const readCSSDeclaration=(entries,name)=>{
 const direct=entries.find(e=>e.name===name);if(direct)return direct.value;
 const components=cssShorthandComponents[name];if(!components)return '';
 const selected=components.map(n=>entries.find(e=>e.name===n));if(selected.some(e=>!e)||selected.some(e=>e.priority!==selected[0].priority))return '';
 if(selected.every(e=>e.pending?.name===name&&e.pending.value===selected[0].pending.value))return selected[0].pending.value;
 if(selected.every(e=>e.value===selected[0].value)&&cssWideValue(selected[0].value))return selected[0].value;
 if(selected.some(e=>e.pending))return '';
 return serializeOrdinaryCSSShorthand(name,selected.map(e=>e.value));
};
const serializeCSSDeclarations=entries=>{
 const emitted=new Set(),out=[];
 for(const entry of entries){
  if(emitted.has(entry.name))continue;
  let name=entry.name,value=entry.value;
  for(const [candidate,components] of Object.entries(cssShorthandComponents)){
   if(!components.includes(entry.name)||components.some(n=>emitted.has(n)))continue;
   const combined=readCSSDeclaration(entries,candidate);if(!combined)continue;
   name=candidate;value=combined;for(const component of components)emitted.add(component);break;
  }
  emitted.add(entry.name);out.push(name+': '+value+(entry.priority?' !important':'')+';');
 }
 return out.join(' ');
};
