// SVG inline text positions and shaping are observations, not a paint surface.
// Horizontal ink edges use font outlines; Windows raster hinting remains an
// explicit approximation. Advances, baseline metrics and DOM positioning do
// not depend on that approximation.
const textLayout=target=>{
 let root=target;while(tag(root)!=='text'){root=parent(root);if(!root||!['text','tspan'].includes(tag(root)))return {box:null,length:0,characters:[]}}
 const declarations=new Map(),styles=new Map(),records=[];
 const own=(n,key)=>{let d=declarations.get(n);if(!d){d=computedCSSDeclarations(n);declarations.set(n,d)}const entry=d.find(v=>v.name===key);return entry?entry.value:attr(n,key)};
 const inherited=(n,key,fallback)=>{for(let p=n;isDOMNode(p);p=parent(p)){if(elementSlot(p)?.type!=='element')continue;const v=own(p,key);if(v!=null&&v!==''&&v!=='inherit'&&v!=='unset'){return v==='initial'?fallback:v}}return fallback};
 const fontSize=n=>{const size=cssComputedFontSize(n);if(size===null)unsupported('textFontSize');return size};
 const spacing=(raw,size)=>{if(raw==='normal')return 0;const m=new RegExp('^('+numberPattern+')(px|em)?$').exec(String(raw));if(!m)unsupported('textSpacing');return Number(m[1])*(m[2]==='em'?size:1)};
 const style=n=>{let value=styles.get(n);if(value)return value;const size=fontSize(n);const weight=inherited(n,'font-weight','normal'),fontStyle=inherited(n,'font-style','normal');if(!['normal','italic','oblique'].includes(fontStyle))unsupported('textFontStyle');if(inherited(n,'writing-mode','horizontal-tb')!=='horizontal-tb'||inherited(n,'direction','ltr')!=='ltr')unsupported('textDirection');value={size,family:inherited(n,'font-family','serif'),weight:weight==='bold'?700:weight==='normal'?400:Number(weight),italic:fontStyle!=='normal',anchor:inherited(n,'text-anchor','start'),baseline:inherited(n,'dominant-baseline','auto'),letter:spacing(inherited(n,'letter-spacing','normal'),size),word:spacing(inherited(n,'word-spacing','normal'),size),noKern:inherited(n,'font-kerning','auto')==='none',white:inherited(n,'white-space','normal')};if(own(n,'textLength')!=null||own(n,'lengthAdjust')!=null||inherited(n,'baseline-shift','baseline')!=='baseline')unsupported('textLengthOrBaselineShift');styles.set(n,value);return value};
 const collect=(n,ancestors,depth)=>{if(depth>256)unsupported('textTreeDepth');const chain=ancestors.concat(n),s=style(n);for(const row of host.nodeChildren(elementSlot(n).nodeId)){if(row.type==='text'){for(const ch of Array.from(host.textContent(row.nodeId))){if(records.length>=16384)unsupported('textComplexity');records.push({ch,node:n,chain,style:s})}}else if(row.type==='element'){const child=wrap(row);if(displayNone(child))continue;if(tag(child)==='tspan')collect(child,chain,depth+1);else if(tag(child)==='textPath')unsupported('textPath')}}};
 collect(root,[],0);
 // Collapse across text-node/tspan boundaries before consuming position lists.
 const normalized=[];for(const record of records){if(!['pre','pre-wrap','break-spaces'].includes(record.style.white)&&/[\t\n\r\f ]/.test(record.ch)){if(!normalized.length||normalized[normalized.length-1].ch===' ')continue;record.ch=' '}normalized.push(record)}while(normalized.length&&normalized[normalized.length-1].ch===' '&&!['pre','pre-wrap','break-spaces'].includes(normalized[normalized.length-1].style.white))normalized.pop();
 if(!normalized.length)return {box:null,length:0,characters:[]};
 const counts=new Map(),lists=new Map();
 const list=(n,key)=>{let values=lists.get(n);if(!values){values={};for(const k of ['x','y','dx','dy','rotate']){const raw=attr(n,k);values[k]=raw==null?[]:numbers(raw);if(raw&&values[k].length===0)unsupported('textPositionUnits')}lists.set(n,values)}return values[key]};
 for(const record of normalized){for(const n of record.chain){const index=counts.get(n)||0;for(const key of ['x','y','dx','dy','rotate']){const a=list(n,key),v=key==='rotate'&&a.length?a[Math.min(index,a.length-1)]:a[index];if(v!==undefined)record[key]=v}counts.set(n,index+1)}}
 let x=0,y=0,chunk=null,total=null,computedLength=0;const characters=[];
 const flush=()=>{if(!chunk)return;const shift=chunk.anchor==='middle'?(x-chunk.start)/2:chunk.anchor==='end'?x-chunk.start:0;for(const item of chunk.items)if(item.selected){const b=item.box.slice();b[0]-=shift;b[2]-=shift;total=union(total,b);for(const c of item.characters||[]){c.start[0]-=shift;c.end[0]-=shift;c.box[0]-=shift;c.box[2]-=shift;characters.push(c)}}chunk=null};
 for(let i=0;i<normalized.length;){
  const first=normalized[i],s=first.style;let end=i+1;
  while(end<normalized.length&&normalized[end].node===first.node&&['x','y','dx','dy','rotate'].every(k=>normalized[end][k]===undefined))end++;
  if(first.x!==undefined||first.y!==undefined)flush();if(first.x!==undefined)x=first.x;if(first.y!==undefined)y=first.y;x+=first.dx||0;y+=first.dy||0;
  if(!chunk)chunk={start:x,anchor:s.anchor,items:[]};
  const selected=first.chain.includes(target),text=normalized.slice(i,end).map(r=>r.ch).join('');
  if(s.size===0){i=end;continue}
  const shaped=JSON.parse(host.shapeText(text,s.family,s.size,s.weight,Number(s.italic),Number(s.noKern),Number(s.letter!==0)));
  if(shaped.error)unsupported('textFontResource');
  let baseline=0;switch(s.baseline){case 'auto':case 'alphabetic':break;case 'middle':baseline=shaped.xHeight/2;break;case 'hanging':baseline=Math.round(shaped.ascent*.8*64)/64;break;default:unsupported('textBaseline')}
  let pen=0,ink=null;const clusters=new Map();
  for(const glyph of shaped.glyphs){let group=clusters.get(glyph.cluster);if(!group){group={start:pen,end:pen};clusters.set(glyph.cluster,group)}const offset=pen+glyph.xOffset;if(glyph.ink)ink=union(ink,[offset+glyph.left,-shaped.ascent+baseline+glyph.yOffset,offset+glyph.right,shaped.descent+baseline+glyph.yOffset]);pen+=glyph.advance+s.letter+(normalized[i+glyph.cluster]?.ch===' '?s.word:0);group.end=pen}
  const advance=Math.ceil(pen*64)/64;let b=union([0,-shaped.ascent+baseline,advance,shaped.descent+baseline],ink);
  if(first.rotate){const a=first.rotate*Math.PI/180;b=transformBox(b,[Math.cos(a),Math.sin(a),-Math.sin(a),Math.cos(a),0,0])}
  b=[b[0]+x,b[1]+y,b[2]+x,b[3]+y];const rows=[],keys=Array.from(clusters.keys()).sort((a,b)=>a-b),rotation=first.rotate||0,angle=rotation*Math.PI/180,rotationMatrix=[Math.cos(angle),Math.sin(angle),-Math.sin(angle),Math.cos(angle),x,y];
  for(let k=0;k<keys.length;k++){const from=keys[k],to=keys[k+1]??(end-i),group=clusters.get(from),left=Math.floor(group.start*64)/64,width=Math.ceil((group.end-group.start)*64)/64,id={};for(let j=from;j<to;j++){for(let unit=0;unit<normalized[i+j].ch.length;unit++)rows.push({id,start:point(rotationMatrix,left,baseline),end:point(rotationMatrix,left+width,baseline),box:transformBox([left,-shaped.ascent+baseline,left+width,shaped.descent+baseline],rotationMatrix),rotation,length:width})}}
  chunk.items.push({selected,box:b,characters:rows});if(selected)computedLength+=advance;x+=advance;i=end;
 }
 flush();host.semanticMissingAt('svg_text.js:40','SVG.approximateTextInkBounds');return {box:total,length:computedLength,characters};
};

if(globalThis.SVGTextContentElement){
 const getComputedTextLength={getComputedTextLength(){const n=check(this);if(!['text','tspan'].includes(tag(n)))unsupported('textContentGeometry');return measurable(n)?textLayout(n).length:0}}.getComputedTextLength;
 markNative(getComputedTextLength,'getComputedTextLength');Object.defineProperty(SVGTextContentElement.prototype,'getComputedTextLength',{value:getComputedTextLength,writable:true,enumerable:true,configurable:true});
}

const svgTextRows=n=>{if(!['text','tspan'].includes(tag(n)))svgFail('textContentGeometry','textPath character positioning is unsupported');return measurable(n)?textLayout(n).characters:[]};
const svgCharIndex=(rows,index)=>{index=(+index)>>>0;if(index>=rows.length)throw new DOMException('Character index out of range','IndexSizeError');return index};
svgMethod('SVGTextContentElement','getNumberOfChars',function(){return svgTextRows(check(this)).length});
for(const name of ['getStartPositionOfChar','getEndPositionOfChar','getExtentOfChar','getRotationOfChar'])svgMethod('SVGTextContentElement',name,function(index){const rows=svgTextRows(check(this)),r=rows[svgCharIndex(rows,index)];return name==='getExtentOfChar'?svgRect(r.box):name==='getRotationOfChar'?r.rotation:svgPoint(...(name==='getStartPositionOfChar'?r.start:r.end))});
svgMethod('SVGTextContentElement','getSubStringLength',function(index,count){const rows=svgTextRows(check(this));index=svgCharIndex(rows,index);count=(+count)>>>0;let sum=0;const seen=new Set();for(const row of rows.slice(index,index+count))if(!seen.has(row.id)){seen.add(row.id);sum+=row.length}return Math.fround(sum)});
svgMethod('SVGTextContentElement','getCharNumAtPosition',function(value={}){const rows=svgTextRows(check(this));if(value===null)value={};const x=Number(value.x??0),y=Number(value.y??0);return rows.findIndex(r=>x>=r.box[0]&&x<=r.box[2]&&y>=r.box[1]&&y<=r.box[3])});
svgMethod('SVGTextContentElement','selectSubString',function(index,count){const rows=svgTextRows(check(this));svgCharIndex(rows,index);Number(count);svgFail('textSelection','SVG text selection is unsupported')});
