(() => {
 const rules=['@keyframes a{from{transform:scale3d(1,1,1)}to{transform:translate(1px,2px)}}','@media (width > 1px){.a{color:red}.b{color:blue}}','@supports (display:grid){.a{display:grid}}','@layer a{.a{color:red}}','@keyframes empty{}'];
 const transforms=['matrix(1,0,0,1,2,3)','translate(1px,2px)','translateX(2px)','translate3d(1px,2px,3px)','scale(1,2)','scale3d(1,2,3)','rotate3d(1,0,0,20deg)','skew(1deg,2deg)','var(--a,translate(1px,2px))','translate(calc(1px + 2px),4px)','perspective(200px) rotate(20deg)'];
 const out={rules:rules.map(text=>{const s=new CSSStyleSheet();s.replaceSync(text);return Array.from(s.cssRules,r=>r.cssText)}),transforms:transforms.map(v=>{const e=document.createElement('div');e.style.transform=v;return [v,e.style.transform,e.style.cssText]})};
return out;
})()
