const cssTimingKeywords=new Set('ease linear ease-in ease-out ease-in-out step-start step-end'.split(' '));
const cssTimeRegex=new RegExp('^('+cssNumberPattern+')(ms|s)$','i');
const cssTimeValue=value=>{const match=cssTimeRegex.exec(value);return match?{value:cssSerializeNumber(Number(match[1]))+match[2].toLowerCase(),number:Number(match[1])}:null};
const cssTimingValue=value=>{
 value=value.toLowerCase();if(cssTimingKeywords.has(value))return value;
 let match=/^cubic-bezier\(\s*([^()]*)\s*\)$/.exec(value);
 if(match){const parts=match[1].split(',').map(v=>v.trim());if(parts.length!==4||parts.some(v=>!cssNumberRegex.test(v)))return null;const values=parts.map(Number);if(values[0]<0||values[0]>1||values[2]<0||values[2]>1)return null;return 'cubic-bezier('+values.map(cssSerializeNumber).join(', ')+')'}
 match=/^steps\(\s*(\+?\d+)\s*(?:,\s*(jump-start|jump-end|jump-none|jump-both|start|end)\s*)?\)$/.exec(value);
 if(match){const count=Number(match[1]),position=match[2]||'end';if(count<1||position==='jump-none'&&count<2)return null;return 'steps('+Math.min(count,2147483647)+(['end','jump-end'].includes(position)?'':', '+position)+')'}
 return null;
};
const cssAnimationIdentifier=value=>/^(?:--[-_a-zA-Z0-9]*|-?[_a-zA-Z][-_a-zA-Z0-9]*)$/.test(value)&&!cssWideValue(value.toLowerCase())&&value.toLowerCase()!=='default'?value:null;
const cssAnimationName=value=>{
 if(value.toLowerCase()==='none')return 'none';
 const string=cssStringValue(value);if(string!==null){const inner=string.slice(1,-1);return cssAnimationIdentifier(inner)&&inner.toLowerCase()!=='none'?inner:string}
 return cssAnimationIdentifier(value);
};
const parseCSSTimelineShorthand=(value,animation)=>{
 const layers=cssSplitTopLevel(value,',');if(!layers||layers.some(v=>!v))return null;
 const output=[];
 for(const layer of layers){
  const tokens=cssValueTokens(layer);if(!tokens?.length)return null;
  let duration=null,timing=null,delay=null,name=null,iteration=null,direction=null,fill=null,play=null,behavior=null;
  for(const token of tokens){
   const lower=token.toLowerCase(),time=cssTimeValue(token),easing=cssTimingValue(token);
   if(time){if(duration===null&&time.number>=0)duration=time.value;else if(delay===null)delay=time.value;else return null;continue}
   if(animation&&lower==='auto'&&duration===null){duration='auto';continue}
   if(easing!==null&&timing===null){timing=easing;continue}
   if(animation){
    const count=cssNonnegativeNumber(token);
    if(iteration===null&&(count!==null||lower==='infinite')){iteration=count??'infinite';continue}
    if(direction===null&&['normal','reverse','alternate','alternate-reverse'].includes(lower)){direction=lower;continue}
    if(fill===null&&['none','forwards','backwards','both'].includes(lower)){fill=lower;continue}
    if(play===null&&['running','paused'].includes(lower)){play=lower;continue}
   }else if(behavior===null&&['normal','allow-discrete'].includes(lower)){behavior=lower;continue}
   const identifier=animation?cssAnimationName(token):cssAnimationIdentifier(token);
   if(identifier===null||name!==null)return null;name=identifier;
  }
  if(animation)output.push([duration??'auto',timing??'ease',delay??'0s',iteration??'1',direction??'normal',fill??'none',play??'running',name??'none']);
  else{if(name==='none'&&layers.length!==1)return null;output.push([name??'all',duration??'0s',timing??'ease',delay??'0s',behavior??'normal'])}
 }
 const columns=output[0].map((_,i)=>output.map(row=>row[i]).join(', '));
 return animation?columns.concat(['auto','normal','normal']):columns;
};
const serializeCSSTimelineShorthand=(values,animation)=>{
 if(animation&&(values[8]!=='auto'||values[9]!=='normal'||values[10]!=='normal'))return '';
 const lists=values.slice(0,animation?8:5).map(v=>cssSplitTopLevel(v,','));
 if(lists.some(v=>!v||v.length!==lists[0].length))return '';
 return lists[0].map((_,i)=>{
  const row=lists.map(values=>values[i]);if(animation)return row.join(' ');
  const [name,duration,timing,delay,behavior]=row,parts=[];
  if(name!=='all')parts.push(name);
  const durationTime=cssTimeValue(duration),delayTime=cssTimeValue(delay);
  if(!durationTime||!delayTime)return '';
  if(durationTime.number!==0||delayTime.number>0)parts.push(duration);
  if(timing!=='ease'||cssTimingKeywords.has(name.toLowerCase()))parts.push(timing);
  if(delayTime.number!==0)parts.push(delay);
  if(behavior!=='normal'||['normal','allow-discrete'].includes(name.toLowerCase()))parts.push(behavior);
  return parts.join(' ')||'all';
 }).join(', ');
};
cssShorthandParsers.set('animation',value=>parseCSSTimelineShorthand(value,true));
cssShorthandParsers.set('transition',value=>parseCSSTimelineShorthand(value,false));
// Ordinary transition parsing has a different insertion order from a CSS-wide
// reset. Both orders are visible through style.item()/indexed properties.
const cssOrdinaryShorthandOrder={transition:['transition-behavior','transition-duration','transition-timing-function','transition-delay','transition-property']};
const cssListValue=(value,parse)=>{const items=cssSplitTopLevel(value,',');if(!items?.length||items.some(v=>!v))return null;const values=items.map(parse);return values.includes(null)?null:values.join(', ')};
const cssKeywordValue=words=>value=>words.includes(value.toLowerCase())?value.toLowerCase():null;
for(const prefix of ['animation','transition']){
 cssLonghandParsers.set(prefix+'-duration',value=>cssListValue(value,item=>{if(prefix==='animation'&&item.toLowerCase()==='auto')return 'auto';const time=cssTimeValue(item);return time&&time.number>=0?time.value:null}));
 cssLonghandParsers.set(prefix+'-delay',value=>cssListValue(value,item=>cssTimeValue(item)?.value??null));
 cssLonghandParsers.set(prefix+'-timing-function',value=>cssListValue(value,cssTimingValue));
}
cssLonghandParsers.set('animation-name',value=>cssListValue(value,cssAnimationName));
cssLonghandParsers.set('animation-iteration-count',value=>cssListValue(value,item=>item.toLowerCase()==='infinite'?'infinite':cssNonnegativeNumber(item)));
for(const [property,words] of Object.entries({'animation-direction':['normal','reverse','alternate','alternate-reverse'],'animation-fill-mode':['none','forwards','backwards','both'],'animation-play-state':['running','paused'],'transition-behavior':['normal','allow-discrete']}))cssLonghandParsers.set(property,value=>cssListValue(value,cssKeywordValue(words)));
cssLonghandParsers.set('transition-property',value=>{
 const result=cssListValue(value,item=>['all','none'].includes(item.toLowerCase())?item.toLowerCase():cssAnimationIdentifier(item));
 return result&&result!=='none'&&cssSplitTopLevel(result,',').includes('none')?null:result;
});
