// Shared lexical helpers for CSS declarations. Tokens keep balanced function
// arguments together; whitespace inside a function is not a shorthand boundary.
const cssValueTokens=input=>{
 const tokens=[];let value='',depth=0,quote='';
 for(let i=0;i<input.length;i++){
  const c=input[i];
  if(quote){value+=c;if(c==='\\'&&i+1<input.length)value+=input[++i];else if(c===quote)quote='';continue}
  if(c==='"'||c==="'"){quote=c;value+=c;continue}
  if(c==='/'&&input[i+1]==='*'){const end=input.indexOf('*/',i+2);if(end<0)return null;i=end+1;if(!depth&&value){tokens.push(value);value=''}else value+=' ';continue}
  if(c==='(')depth++;else if(c===')'&&--depth<0)return null;
  if(/\s/.test(c)&&depth===0){if(value){tokens.push(value);value=''}}else value+=c;
 }
 if(depth||quote)return null;if(value)tokens.push(value);return tokens;
};
const cssNumberPattern='[+-]?(?:\\d*\\.\\d+|\\d+)(?:e[+-]?\\d+)?';
const cssNumberRegex=new RegExp('^'+cssNumberPattern+'$','i');
const cssSerializeNumber=value=>{
 // Chrome's specified-value number serialization uses six significant digits
 // and saturates finite CSS numeric tokens at the float range (including 1e999).
 value=Number(Math.max(-3.4028234663852886e38,Math.min(3.4028234663852886e38,value)).toPrecision(6));
 if(value!==0&&(Math.abs(value)<0.0001||Math.abs(value)>=1000000))return value.toExponential().replace(/e([+-])(\d)$/, 'e$10$2');
 return String(value);
};
const cssNonnegativeNumber=value=>cssNumberRegex.test(value)&&Number(value)>=0?cssSerializeNumber(Number(value)):null;
const cssLengthUnits=new Set('px cm mm q in pt pc em ex ch rem lh rlh cap ic rcap ric rch rex vw vh vmin vmax vi vb svw svh svmin svmax svi svb lvw lvh lvmin lvmax lvi lvb dvw dvh dvmin dvmax dvi dvb cqw cqh cqi cqb cqmin cqmax'.split(' '));
const cssDimensionRegex=new RegExp('^('+cssNumberPattern+')([a-z]+|%)$','i');
const cssLengthTerm=value=>{const match=cssDimensionRegex.exec(value);if(!match||!Number.isFinite(Number(match[1])))return null;const unit=match[2].toLowerCase();return unit==='%'||cssLengthUnits.has(unit)?{number:Number(match[1]),unit}:null};
const cssLengthValue=value=>{
 if(cssNumberRegex.test(value)&&Number(value)===0)return '0px';
 const term=cssLengthTerm(value);if(term)return term.number>=0?cssSerializeNumber(term.number)+term.unit:null;
 const single=/^calc\(\s*([^\s()]+)\s*\)$/i.exec(value);if(single){const term=cssLengthTerm(single[1]);return term?'calc('+cssSerializeNumber(term.number)+term.unit+')':null}
 const match=/^calc\(\s*(\S+)\s+([+-])\s+(\S+)\s*\)$/i.exec(value);
 if(!match)return null;
 const a=cssLengthTerm(match[1]),b=cssLengthTerm(match[3]);if(!a||!b)return null;
 if(match[2]==='-')b.number=-b.number;
 if(a.unit===b.unit)return 'calc('+(a.number+b.number)+a.unit+')';
 // Keep the percentage term first in a mixed percentage/length calculation.
 if(a.unit!=='%'&&b.unit!=='%')return null;
 const first=a.unit==='%'?a:b,second=first===a?b:a;
 return 'calc('+first.number+first.unit+(second.number<0?' - ':' + ')+Math.abs(second.number)+second.unit+')';
};
const parseCSSFlex=value=>{
 const tokens=cssValueTokens(value.toLowerCase());if(!tokens||!tokens.length||tokens.length>3)return null;
 if(tokens.length===1&&tokens[0]==='none')return ['0','0','auto'];
 let grow=null,shrink=null,basis=null,numericEnded=false;
 for(const token of tokens){
  const number=cssNonnegativeNumber(token);
  if(number!==null&&grow===null){if(numericEnded)return null;grow=number;continue}
  if(number!==null&&shrink===null&&basis===null){shrink=number;continue}
  if(basis!==null)return null;
  basis=['auto','content','min-content','max-content','fit-content','stretch'].includes(token)?token:cssLengthValue(token);
  if(basis===null)return null;if(grow!==null)numericEnded=true;
 }
 return [grow??'1',shrink??'1',basis??'0%'];
};
const parseCSSFlexFlow=value=>{
 const tokens=cssValueTokens(value.toLowerCase());if(!tokens||!tokens.length||tokens.length>2)return null;
 let direction=null,wrap=null;
 for(const token of tokens){if(['row','row-reverse','column','column-reverse'].includes(token)){if(direction!==null)return null;direction=token}else if(['nowrap','wrap','wrap-reverse'].includes(token)){if(wrap!==null)return null;wrap=token}else return null}
 return [direction??'row',wrap??'nowrap'];
};
const cssNamedColors=new Set('aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow yellowgreen transparent currentcolor'.split(' '));
for(const name of 'activetext buttonborder buttonface buttontext canvas canvastext field fieldtext graytext highlight highlighttext linktext mark marktext selecteditem selecteditemtext visitedtext accentcolor accentcolortext activeborder activecaption appworkspace background buttonhighlight buttonshadow captiontext inactiveborder inactivecaption inactivecaptiontext infobackground infotext menu menutext scrollbar threeddarkshadow threedface threedhighlight threedlightshadow threedshadow window windowframe windowtext'.split(' '))cssNamedColors.add(name);
const cssColorValue=value=>{
 value=value.toLowerCase();if(cssNamedColors.has(value))return value;
 let match=/^#([\da-f]{3,4}|[\da-f]{6}|[\da-f]{8})$/.exec(value);
 if(match){let digits=match[1];if(digits.length<5)digits=Array.from(digits,c=>c+c).join('');const rgb=[0,2,4].map(i=>parseInt(digits.slice(i,i+2),16)),alpha=digits.length===8?parseInt(digits.slice(6),16):255;
  if(alpha===255)return 'rgb('+rgb.join(', ')+')';
  let fraction=alpha/255;for(let places=0;places<=3;places++){const rounded=Number(fraction.toFixed(places));if(Math.round(rounded*255)===alpha){fraction=rounded;break}}
  return 'rgba('+rgb.join(', ')+', '+fraction+')';
 }
 match=/^(rgb|rgba)\(\s*([^()]+)\s*\)$/.exec(value);if(!match)return null;
 const parts=match[2].split(',').map(v=>v.trim());if(parts.length!==3&&parts.length!==4)return null;
 const percentages=parts.slice(0,3).map(v=>v.endsWith('%'));if(percentages.some(Boolean)&&!percentages.every(Boolean))return null;
 const rgb=[];for(let i=0;i<3;i++){const token=percentages[i]?parts[i].slice(0,-1):parts[i];if(!cssNumberRegex.test(token))return null;rgb.push(Math.round(Math.max(0,Math.min(255,Number(token)*(percentages[i]?255:1)/(percentages[i]?100:1)))))}
 let alpha=1;if(parts.length===4){const percent=parts[3].endsWith('%'),token=percent?parts[3].slice(0,-1):parts[3];if(!cssNumberRegex.test(token))return null;alpha=Math.max(0,Math.min(1,Number(token)/(percent?100:1)))}
 return alpha===1?'rgb('+rgb.join(', ')+')':'rgba('+rgb.join(', ')+', '+cssSerializeNumber(alpha)+')';
};
const cssBorderWidth=value=>['thin','medium','thick'].includes(value.toLowerCase())?value.toLowerCase():value.includes('%')?null:cssLengthValue(value);
const cssBorderStyles=new Set('none hidden dotted dashed solid double groove ridge inset outset'.split(' '));
const parseCSSBorder=(value,stroke=false)=>{
 const tokens=cssValueTokens(value);if(!tokens||!tokens.length||tokens.length>(stroke?2:3))return null;
 let width=null,style=null,color=null;
 for(const token of tokens){const w=cssBorderWidth(token),c=cssColorValue(token);if(w!==null){if(width!==null)return null;width=w}else if(!stroke&&cssBorderStyles.has(token.toLowerCase())){if(style!==null)return null;style=token.toLowerCase()}else if(c!==null){if(color!==null)return null;color=c}else return null}
 return stroke?[width??'initial',color??'initial']:[width??'initial',style??'initial',color??'initial'];
};
const cssSplitTopLevel=(input,separator)=>{
 const parts=[];let start=0,depth=0,quote='';
 for(let i=0;i<input.length;i++){const c=input[i];if(quote){if(c==='\\')i++;else if(c===quote)quote='';continue}if(c==='"'||c==="'")quote=c;else if(c==='(')depth++;else if(c===')'){if(--depth<0)return null}else if(c===separator&&depth===0){parts.push(input.slice(start,i).trim());start=i+1}}
 if(depth||quote)return null;parts.push(input.slice(start).trim());return parts;
};
const cssStringValue=token=>{
 const quote=token[0];if(!['"',"'"].includes(quote)||token.at(-1)!==quote)return null;
 let text='';for(let i=1;i<token.length-1;i++){
  let c=token[i];if(c==='\n'||c==='\r'||c==='\f'||c===quote)return null;
  if(c==='\\'){
   c=token[++i];if(i>=token.length-1)return null;
   if(c==='\n'||c==='\f')continue;if(c==='\r'){if(token[i+1]==='\n')i++;continue}
   const hex=/^[\da-f]{1,6}/i.exec(token.slice(i));if(hex){const point=parseInt(hex[0],16);text+=String.fromCodePoint(!point||point>0x10ffff||point>=0xd800&&point<=0xdfff?0xfffd:point);i+=hex[0].length-1;if(/\s/.test(token[i+1])){i++;if(token[i]==='\r'&&token[i+1]==='\n')i++}continue}
  }
  text+=c==='\0'?'\ufffd':c;
 }
 return '"'+Array.from(text,c=>c==='"'||c==='\\'?'\\'+c:c.codePointAt(0)<32||c.codePointAt(0)===127?'\\'+c.codePointAt(0).toString(16)+' ':c).join('')+'"';
};
const parseCSSTextEmphasis=value=>{
 const tokens=cssValueTokens(value);if(!tokens||!tokens.length)return null;let style=null,color=null;
 for(let i=0;i<tokens.length;i++){
  const token=tokens[i],c=cssColorValue(token);if(c!==null){if(color!==null)return null;color=c;continue}
  if(style!==null)return null;
  const string=cssStringValue(token);if(string!==null){style=string;continue}
  if(token.toLowerCase()==='none'){style='none';continue}
  let fill=null,shape=null;for(;i<tokens.length;i++){const word=tokens[i].toLowerCase();if(['filled','open'].includes(word)){if(fill!==null)return null;fill=word}else if(['dot','circle','double-circle','triangle','sesame'].includes(word)){if(shape!==null)return null;shape=word}else break}
  if(fill===null&&shape===null)return null;i--;style=[fill,shape].filter(v=>v!==null).join(' ');
 }
 return [style??'initial',color??'initial'];
};
const parseCSSColumns=value=>{
 const tokens=cssValueTokens(value.toLowerCase());if(!tokens||!tokens.length||tokens.length>2)return null;
 let width=null,count=null,autos=0;
 for(const token of tokens){if(token==='auto'){autos++;continue}if(/^\+?\d+$/.test(token)&&Number(token)>0){if(count!==null)return null;count=String(Math.min(Number(token),2147483647));continue}const length=token.includes('%')?null:cssLengthValue(token);if(length===null||width!==null)return null;width=length}
 if(autos+(width!==null?1:0)+(count!==null?1:0)>2)return null;
 return [width??'auto',count??'auto','auto','auto'];
};
const cssExpandFour=values=>[values[0],values[1]??values[0],values[2]??values[0],values[3]??values[1]??values[0]];
const cssCompressFour=values=>{values=values.slice();if(values[3]===values[1]){values.pop();if(values[2]===values[0]){values.pop();if(values[1]===values[0])values.pop()}}return values.join(' ')};
const parseCSSRadius=value=>{
 const parts=cssSplitTopLevel(value,'/');if(!parts||parts.length>2)return null;
 const axes=[];for(const part of parts){const tokens=cssValueTokens(part);if(!tokens||!tokens.length||tokens.length>4)return null;const values=tokens.map(cssLengthValue);if(values.includes(null))return null;axes.push(cssExpandFour(values))}
 const [x,y=x]=axes;return x.map((v,i)=>v===y[i]?v:v+' '+y[i]);
};
const cssAliasInputValue=(name,value,inputName)=>{
 // The prefixed two-value form predates the standard corner shorthand: its
 // values describe horizontal and vertical radii, not alternating corners.
 if(name==='border-radius'&&String(inputName).toLowerCase()==='-webkit-border-radius'){
  const parts=cssSplitTopLevel(value,'/'),tokens=cssValueTokens(value);if(parts?.length===1&&tokens?.length===2)return tokens.join(' / ');
 }
 return value;
};
const cssShorthandParsers=new Map([['flex',parseCSSFlex],['flex-flow',parseCSSFlexFlow]]);
const cssLonghandParsers=new Map([
 ['flex-grow',value=>cssNonnegativeNumber(value)],['flex-shrink',value=>cssNonnegativeNumber(value)],
 ['flex-basis',value=>{value=value.toLowerCase();return ['auto','content','min-content','max-content','fit-content','stretch'].includes(value)?value:cssLengthValue(value)}],
 ['flex-direction',value=>{value=value.toLowerCase();return ['row','row-reverse','column','column-reverse'].includes(value)?value:null}],
 ['flex-wrap',value=>{value=value.toLowerCase();return ['nowrap','wrap','wrap-reverse'].includes(value)?value:null}]
]);
for(const name of ['border-block-start','border-block-end','border-inline-start','border-inline-end']){
 cssShorthandParsers.set(name,value=>parseCSSBorder(value));
 cssLonghandParsers.set(name+'-width',cssBorderWidth);cssLonghandParsers.set(name+'-style',value=>cssBorderStyles.has(value.toLowerCase())?value.toLowerCase():null);cssLonghandParsers.set(name+'-color',cssColorValue);
}
cssShorthandParsers.set('-webkit-text-stroke',value=>parseCSSBorder(value,true));
cssLonghandParsers.set('-webkit-text-stroke-width',cssBorderWidth);cssLonghandParsers.set('-webkit-text-stroke-color',cssColorValue);cssLonghandParsers.set('-webkit-text-fill-color',cssColorValue);
cssShorthandParsers.set('column-rule',value=>{const parsed=parseCSSBorder(value);return parsed?.map((v,i)=>v==='initial'?['medium','none','currentcolor'][i]:v)??null});
cssLonghandParsers.set('column-rule-width',cssBorderWidth);cssLonghandParsers.set('column-rule-style',value=>cssBorderStyles.has(value.toLowerCase())?value.toLowerCase():null);cssLonghandParsers.set('column-rule-color',cssColorValue);
cssShorthandParsers.set('columns',parseCSSColumns);
cssLonghandParsers.set('column-count',value=>value.toLowerCase()==='auto'?'auto':/^\+?\d+$/.test(value)&&Number(value)>0?String(Math.min(Number(value),2147483647)):null);
cssLonghandParsers.set('column-width',value=>value.toLowerCase()==='auto'?'auto':value.includes('%')?null:cssLengthValue(value));
cssShorthandParsers.set('text-emphasis',parseCSSTextEmphasis);
cssLonghandParsers.set('text-emphasis-style',value=>{const parsed=parseCSSTextEmphasis(value);return parsed&&parsed[1]==='initial'?parsed[0]:null});
cssShorthandParsers.set('border-radius',parseCSSRadius);
for(const name of cssShorthandComponents['border-radius'])cssLonghandParsers.set(name,value=>{
 const tokens=cssValueTokens(value);if(!tokens||tokens.length<1||tokens.length>2)return null;
 const values=tokens.map(cssLengthValue);if(values.includes(null))return null;return values.length===1||values[0]===values[1]?values[0]:values.join(' ');
});
cssLonghandParsers.set('text-emphasis-color',cssColorValue);
const serializeOrdinaryCSSShorthand=(name,values)=>{
 if(/^border-(block|inline)-(start|end)$/.test(name)||name==='-webkit-text-stroke'||name==='text-emphasis')return values.some(v=>!v||cssWideValue(v)&&v!=='initial')?'':values.filter(v=>v!=='initial').join(' ');
 if(values.some(v=>v===''||cssWideValue(v)))return '';
 if(name==='animation'||name==='transition')return serializeCSSTimelineShorthand(values,name==='animation');
 if(name==='flex')return values.join(' ');
 if(name==='flex-flow')return values[0]==='row'?(values[1]==='nowrap'?'row':values[1]):values[0]+(values[1]==='nowrap'?'':' '+values[1]);
 if(name==='column-rule')return values.filter((v,i)=>v!==['medium','none','currentcolor'][i]).join(' ')||'medium';
 if(name==='columns'){if(values[2]!=='auto'||values[3]!=='auto')return '';return values.slice(0,2).filter(v=>v!=='auto').join(' ')||'auto'}
 if(name==='border-radius'){const pairs=values.map(cssValueTokens);if(pairs.some(v=>!v||!v.length||v.length>2))return '';const x=pairs.map(v=>v[0]),y=pairs.map(v=>v[1]??v[0]),a=cssCompressFour(x),b=cssCompressFour(y);return a+(a===b?'':' / '+b)}
 return '';
};

// Priority is declaration metadata, never part of a CSSOM property value.
// Ignore bangs inside strings, comments and nested component values.
const cssTopLevelBang=value=>{
 let quote='',depth=0;
 for(let i=0;i<value.length;i++){const c=value[i];
  if(c==='\\'){i++;continue}
  if(quote){if(c===quote)quote='';continue}
  if(c==='"'||c==="'"){quote=c;continue}
  if(c==='/'&&value[i+1]==='*'){const end=value.indexOf('*/',i+2);if(end<0)return -1;i=end+1;continue}
  if('([{'.includes(c))depth++;else if(')]}'.includes(c))depth--;
  else if(c==='!'&&depth===0)return i;
 }
 return -1;
};
const cssExtractPriority=value=>{
 const index=cssTopLevelBang(value);if(index<0)return {value,priority:''};
 const suffix=value.slice(index+1).replace(/\/\*[\s\S]*?\*\//g,' ').trim();
 return /^important$/i.test(suffix)?{value:value.slice(0,index).trim(),priority:'important'}:null;
};
