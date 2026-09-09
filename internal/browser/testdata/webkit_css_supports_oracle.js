(()=>{
 const conditions=['(-webkit-appearance: none)','(-webkit-appearance: bad)','not (-webkit-appearance: bad)','(-webkit-appearance: none) and (-webkit-user-select: text)','(-webkit-appearance: bad) or (-webkit-user-select: text)','((-webkit-appearance: none))','not bad','not','not ()','()','(foo)','not (foo)','(-webkit-appearance: none) and','(-webkit-appearance: none) or','(-webkit-appearance: none) and (-webkit-user-select: text) or (-webkit-user-drag: none)','(-webkit-appearance: var(--x, auto))','(-webkit-appearance: AUTO)','(-webkit-appearance: auto !important)','(-webkit-appearance: )','(-webkit-appearance: none) AND (-webkit-user-select: text)','NOT (-webkit-appearance: bad)'];
 return Object.fromEntries(conditions.map(c=>[c,CSS.supports(c)]));
})()
