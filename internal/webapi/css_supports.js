const compatibilityCSSSupports={};
(()=>{
 if(!globalThis.CSS)return;
 const prior=CSS.supports;
 const declaration=(property,value)=>{
  const name=cssName(property);
  if(cssShorthandParsers.has(name)||cssLonghandParsers.has(name)){const normalized=normalizeCSSValue(name,value,property);return normalized!==null&&normalized!==''}
  if(webkitCSSKeywords.has(name)){const normalized=normalizeCSSValue(name,value,property);return normalized!==null&&normalized!==''}
  if(webkitCSSAliases.has(String(property).toLowerCase())&&['initial','inherit','unset','revert','revert-layer'].includes(String(value).trim().toLowerCase()))return true;
  return typeof prior==='function'?!!prior.call(CSS,property,value):false;
 };
 const condition=text=>{
  text=text.trim();
  const negate=/^not\s+/i.exec(text);
  if(negate){const tail=text.slice(negate[0].length).trim();if(!/^\(/.test(tail))return null;const result=condition(tail);return result===null?null:!result}
  let depth=0,quote='',parts=[],start=0,operator='';
  for(let i=0;i<text.length;i++){
   const c=text[i];if(quote){if(c==='\\')i++;else if(c===quote)quote='';continue}
   if(c==='"'||c==="'"){quote=c;continue}
   if(c==='(')depth++;else if(c===')'){if(--depth<0)return null}
   if(depth===0){const match=/^\s+(and|or)\s+/i.exec(text.slice(i));if(match){const op=match[1].toLowerCase();if(operator&&operator!==op)return null;operator=op;parts.push(text.slice(start,i));i+=match[0].length-1;start=i+1}}
  }
  if(depth||quote)return null;
  if(operator){parts.push(text.slice(start));const values=parts.map(condition);if(values.includes(null))return null;return operator==='and'?values.every(Boolean):values.some(Boolean)}
  if(text[0]!=='('||text.at(-1)!==')')return null;
  const inner=text.slice(1,-1).trim(),match=/^([-\w]+)\s*:\s*([\s\S]*)$/.exec(inner);
  return match?declaration(match[1],match[2].replace(/\s*!important\s*$/i,'')):(condition(inner)??false);
 };
 const supports={supports(property,value){if(arguments.length===0)throw new TypeError('Not enough arguments');if(arguments.length>1)return declaration(String(property),String(value));const text=String(property).trim();return condition(/^[-\w]+\s*:/.test(text)?'('+text+')':text)===true}}.supports;
 compatibilityCSSSupports.matches=text=>supports(text);
 Object.defineProperty(supports,'length',{value:1,configurable:true});markNative(supports,'supports');Object.defineProperty(CSS,'supports',{value:supports,writable:true,enumerable:true,configurable:true});
})();
