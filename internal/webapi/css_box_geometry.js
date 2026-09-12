// A transient box graph projects the canonical CSS/DOM state. No state survives
// an independent style read; geometry never reparses an inline-only DOM copy.
// CSS lengths leave room for float conversion within signed 26.6 LayoutUnit.
const cssGeometryLength=value=>Math.fround(Math.max(-(2**31)/64+2,Math.min(Math.trunc((2**31-1)/64)-2,value)));
const cssBoxModel=(()=>{
 const tag=element=>elementSlot(element)?.tagName||'',textContent=element=>host.textContent(elementSlot(element).nodeId);
 const unit=value=>Math.trunc(cssGeometryLength(value)*64)/64,collapse=(a,b)=>Math.max(a,b,0)+Math.min(a,b,0);
 const invisible=new Set(['STYLE','SCRIPT','HEAD','TITLE','META','LINK','TEMPLATE','OPTION','NOSCRIPT']);
 const tableDisplays={TABLE:'table',CAPTION:'table-caption',TBODY:'table-row-group',THEAD:'table-header-group',TFOOT:'table-footer-group',TR:'table-row',TD:'table-cell',TH:'table-cell'};
 const blocks=new Set(['HTML','BODY','DIV','P','SECTION','MAIN','ARTICLE','ASIDE','HEADER','FOOTER','NAV','FORM','FIELDSET','DETAILS','SUMMARY','H1','H2','H3','H4','H5','H6','UL','OL','LI','TABLE','CAPTION','TBODY','THEAD','TFOOT','TR','TD','TH']);
 const state=element=>{
  const cache=styleReadCache.boxStyles||(styleReadCache.boxStyles=new WeakMap());if(cache.has(element))return cache.get(element);
  const entries=computedCSSDeclarations(element),get=name=>geometryValue(element,entries.find(e=>e.name===name)?.value??(tag(element)==='BODY'&&/^margin-(top|right|bottom|left)$/.test(name)?'8px':undefined));
  const hiddenInput=tag(element)==='INPUT'&&String(host.getAttribute(elementSlot(element).nodeId,'type')||'').toLowerCase()==='hidden';
  const display=hiddenInput?'none':get('display')||(invisible.has(tag(element))||host.getAttribute(elementSlot(element).nodeId,'hidden')!==null?'none':tableDisplays[tag(element)]|| (tag(element)==='SUMMARY'?'list-item':blocks.has(tag(element))?'block':'inline')),position=get('position')||'static';
  const result={element,entries,get,display,position};cache.set(element,result);
  result.inherited=name=>{for(let p=element;p;p=geometryParent(p)){const v=computedCSSDeclarations(p).find(e=>e.name===name)?.value;if(v&&!['inherit','unset'].includes(v))return v}return null};
  result.length=(text,basis=0)=>{
   if(text==null||['auto','none','normal','initial','unset'].includes(text))return null;
   const value=cssResolveLength(text,{em:result.fontSize??16,rem:(cssComputedFontSize(document.documentElement)??16),percent:basis});
   if(value!==null)return unit(value);
   const viewport=/^([+-]?[\d.]+)(vw|vh|vmin|vmax)$/.exec(text);if(viewport){const size=host.viewport();return unit(Number(viewport[1])*({vw:size.width,vh:size.height,vmin:Math.min(size.width,size.height),vmax:Math.max(size.width,size.height)}[viewport[2]])/100)}
   return null;
  };
  result.edges=basis=>{
   const out={};for(const side of ['top','right','bottom','left']){
    const borderStyle=get('border-'+side+'-style'),borderWidth=get('border-'+side+'-width');
    out['b'+side]=borderStyle&&['none','hidden'].includes(borderStyle)?0:borderWidth?Math.max(0,Math.floor(result.length(borderWidth)??({thin:1,medium:3,thick:5}[borderWidth]||0))):0;
    out['p'+side]=Math.max(0,result.length(get('padding-'+side),basis)??(['TD','TH'].includes(tag(element))?1:0));
    out['m'+side]=result.length(get('margin-'+side),basis)||0;
   }return out;
  };
  // Publish a complete recursive state before resolving font inheritance.
  // Complex author selectors can re-enter geometry while font size walks the
  // ancestor cascade; callers must never observe a half-built state object.
  result.fontSize=(cssComputedFontSize(element)??16);
  return result;
 };
 const children=element=>{const root=elementShadows.get(element)||element,data=elementSlot(root);return data?host.nodeChildren(data.nodeId).map(wrap):Array.from(fragmentState(root).children)};
 const rendered=element=>{
  for(let parent=element;parent;parent=geometryParent(parent))if(state(parent).display==='none')return false;
  return true;
 };
 const textInfo=(element,text)=>{
  const s=state(element),family=s.inherited('font-family')||'"Times New Roman"',weight=Number(s.inherited('font-weight'))|| (tag(element)==='TH'?700:400),italic=s.inherited('font-style')==='italic';
  if(s.fontSize===0){const raw=s.inherited('line-height'),height=raw&&raw!=='normal'?(cssNumberRegex.test(raw)?0:s.length(raw,0)):0;return {width:0,height:height??0,ascent:0,descent:0}}
  const metrics=styleReadCache.textMetrics||(styleReadCache.textMetrics=new Map()),key=JSON.stringify([text,family,s.fontSize,weight,italic]);
  let shaped=metrics.get(key);if(!shaped){shaped=JSON.parse(host.shapeText(text,family,s.fontSize,weight,Number(italic),0,0));metrics.set(key,shaped)}
  if(shaped.error){host.semanticMissingAt('css_box_geometry.js/textInfo','CSS.textBoxMetrics');return {width:0,height:0,ascent:0,descent:0}}
  const raw=s.inherited('line-height'),height=raw&&raw!=='normal'?(cssNumberRegex.test(raw)?unit(Number(raw)*s.fontSize):s.length(raw,s.fontSize)):shaped.ascent+shaped.descent+(shaped.lineGap||0);
  return {width:Math.ceil(shaped.glyphs.reduce((a,g)=>a+g.advance,0)*64)/64,height:height??shaped.ascent+shaped.descent+(shaped.lineGap||0),ascent:shaped.ascent,descent:shaped.descent};
 };
 const textOf=element=>children(element).filter(n=>elementSlot(n)?.type==='text').map(n=>textContent(n)).join('').replace(/[\t\n\r\f ]+/g,' ').trim();
 const controlSize=element=>{
  const cache=styleReadCache.controlSizes||(styleReadCache.controlSizes=new WeakMap());
  if(cache.has(element))return cache.get(element);
  const result=uncachedControlSize(element);cache.set(element,result);return result;
 };
 const uncachedControlSize=element=>{
  const s=state(element);if(tag(element)==='BUTTON'&&!textContent(element))return {width:16,height:6};
  if(tag(element)==='PROGRESS')return {width:s.fontSize*10,height:s.fontSize};
  return compatibilityElementState.controlGeometry?.(element,s.entries)||null;
 };
 const intrinsic=element=>{
  const cache=styleReadCache.intrinsicWidths||(styleReadCache.intrinsicWidths=new WeakMap());
  if(cache.has(element))return cache.get(element);
  const value=uncachedIntrinsic(element);cache.set(element,value);return value;
 };
 const uncachedIntrinsic=element=>{
  const s=state(element);if(s.display==='none')return 0;const own=controlSize(element);if(own)return own.width;
  if(s.display==='table')return tableColumns(element).width;
  const listType=s.get('list-style-type')||(tag(element)==='SUMMARY'?'disclosure-closed':''),inside=s.get('list-style-position')==='inside'||tag(element)==='SUMMARY';
  // Disclosure markers are geometric symbols: Blink uses .66em plus .4em
  // inline margin, each quantized independently to a layout unit.
  const marker=s.display==='list-item'&&inside&&['disclosure-open','disclosure-closed'].includes(listType)?unit(s.fontSize*.66)+unit(s.fontSize*.4):0;
  let width=textInfo(element,textOf(element)).width+marker,line=width;
  for(const child of children(element)){if(elementSlot(child)?.type!=='element')continue;const c=state(child);if(c.display==='none'||['absolute','fixed'].includes(c.position))continue;const edges=c.edges(0),value=(c.length(c.get('width'))??intrinsic(child))+edges.pleft+edges.pright+edges.bleft+edges.bright+edges.mleft+edges.mright;if(c.display==='inline'||replacedGeometryTags.has(tag(child))){line+=value;width=Math.max(width,line)}else {width=Math.max(width,line,value);line=0}}
  return Math.max(width,line);
 };
 const tableColumns=table=>{
  const rows=compatibilitySelectors.query(table,'tr').filter(row=>{for(let p=geometryParent(row);p;p=geometryParent(p)){if(tag(p)==='TABLE')return p===table}return false}),columns=[],cells=[];
  const s=state(table),spacing=s.length(s.get('border-spacing')?.split(/\s+/)[0])??2;
  for(const row of rows){const list=children(row).filter(cell=>['TD','TH'].includes(tag(cell)));cells.push(list);for(let i=0;i<list.length;i++){const cell=list[i],c=state(cell),e=c.edges(0),w=(c.length(c.get('width'))??intrinsic(cell))+e.pleft+e.pright+e.bleft+e.bright;columns[i]=Math.max(columns[i]||0,w)}}
  const e=s.edges(0),width=columns.reduce((a,b)=>a+b,0)+spacing*(columns.length+1)+e.bleft+e.bright;
  return {rows,cells,columns,spacing,width};
 };
 const tableSize=(element,value)=>{
  const info=tableColumns(element),s=state(element),e=value.edges,cache=styleReadCache.boxSizes;
  const captions=children(element).filter(c=>c.tagName==='CAPTION'),contentWidth=value.width-e.bleft-e.bright;
  let captionHeight=0;
  for(const caption of captions){const c=state(caption),ce=c.edges(value.width),font=textInfo(caption,textOf(caption)),available=Math.max(0,value.width-ce.mleft-ce.mright-ce.pleft-ce.pright-ce.bleft-ce.bright);let lines=1,line=0;
   for(const word of textOf(caption).split(' ')){const wordWidth=textInfo(caption,word).width,space=line?textInfo(caption,' ').width:0;if(line&&line+space+wordWidth>available){lines++;line=wordWidth}else line+=space+wordWidth}
   const h=lines*font.height+ce.ptop+ce.pbottom+ce.btop+ce.bbottom;
   cache.set(caption,{width:value.width-ce.mleft-ce.mright,height:h,edges:ce,positions:new Map(),contentHeight:lines*font.height});value.positions.set(caption,{x:ce.mleft-e.bleft,y:captionHeight-e.btop});captionHeight+=h+ce.mtop+ce.mbottom;
  }
  let cursor=captionHeight+info.spacing;const groupStarts=new Map();
  for(let r=0;r<info.rows.length;r++){
   const row=info.rows[r],group=geometryParent(row),rowCells=info.cells[r],height=Math.max(0,...rowCells.map(cell=>{const c=state(cell),ce=c.edges(contentWidth);return (c.length(c.get('height'))??textInfo(cell,textOf(cell)).height)+ce.ptop+ce.pbottom+ce.btop+ce.bbottom}));
   let x=0;const rowBox={width:info.columns.reduce((a,b)=>a+b,0)+Math.max(0,info.columns.length-1)*info.spacing,height,edges:state(row).edges(contentWidth),positions:new Map(),contentHeight:height};cache.set(row,rowBox);
   for(let i=0;i<rowCells.length;i++){const cell=rowCells[i],ce=state(cell).edges(contentWidth);cache.set(cell,{width:info.columns[i],height,edges:ce,positions:new Map(),contentHeight:height-ce.ptop-ce.pbottom-ce.btop-ce.bbottom});rowBox.positions.set(cell,{x,y:0});x+=info.columns[i]+info.spacing}
   if(group===element)value.positions.set(row,{x:info.spacing,y:cursor});else{
    if(!groupStarts.has(group)){groupStarts.set(group,cursor);cache.set(group,{width:rowBox.width,height:0,edges:state(group).edges(contentWidth),positions:new Map(),contentHeight:0});value.positions.set(group,{x:info.spacing,y:cursor})}
    const groupBox=cache.get(group);groupBox.positions.set(row,{x:0,y:cursor-groupStarts.get(group)});groupBox.height=cursor-groupStarts.get(group)+height;groupBox.contentHeight=groupBox.height;
   }
   cursor+=height+info.spacing;
  }
  value.height=cursor+e.btop+e.bbottom;value.contentHeight=cursor;return value;
 };
 const containingWidth=element=>{const parent=geometryParent(element);if(!parent)return host.viewport().width;const width=layoutWidthFor(parent),edges=state(parent).edges(width);return Math.max(0,width-edges.pleft-edges.pright-edges.bleft-edges.bright)};
 const width=element=>{
  const known=styleReadCache.boxSizes?.get(element);if(known)return known.width;
  const cache=styleReadCache.widths;if(cache.has(element))return cache.get(element);cache.set(element,0);
  const s=state(element);if(s.display==='none')return 0;
  const basis=containingWidth(element),e=s.edges(basis),extra=e.pleft+e.pright+e.bleft+e.bright;
  let content=s.length(s.get('width'),basis),borderBox=s.get('box-sizing')==='border-box';
  if(content===null){
   const attribute=host.getAttribute(elementSlot(element).nodeId,'width');if(attribute&&/^\d+(?:\.\d+)?$/.test(attribute))content=Number(attribute);
   else if(controlSize(element))return cache.set(element,controlSize(element).width).get(element);
   else if(s.display==='table'){content=tableColumns(element).width-extra}
   else if(['absolute','fixed'].includes(s.position)||s.display==='inline-block'||s.display==='inline')content=Math.min(Math.max(0,basis-e.mleft-e.mright-extra),intrinsic(element));
   else content=Math.max(0,basis-e.mleft-e.mright-extra);
  }
  let value=Math.max(0,content+(borderBox?0:extra));
  const minimum=s.length(s.get('min-width'),basis),maximum=s.length(s.get('max-width'),basis);
  if(minimum!==null)value=Math.max(value,minimum+(borderBox?0:extra));if(maximum!==null)value=Math.min(value,maximum+(borderBox?0:extra));cache.set(element,value);return value;
 };
 const positionedContainer=element=>{if(state(element).position==='fixed')return null;for(let p=geometryParent(element);p;p=geometryParent(p))if(state(p).position!=='static')return p;return null};
 const size=element=>{
  const cache=styleReadCache.boxSizes||(styleReadCache.boxSizes=new WeakMap());
  for(let parent=geometryParent(element);parent;parent=geometryParent(parent))if(state(parent).display==='table'){if(!cache.has(parent))size(parent);break}
  if(cache.has(element))return cache.get(element);
  const s=state(element),basis=containingWidth(element),e=s.edges(basis),value={width:layoutWidthFor(element),height:0,edges:e,positions:new Map(),contentHeight:0};cache.set(element,value);
  if(!['absolute','fixed'].includes(s.position)&&!['inline','inline-block'].includes(s.display)){const left=s.get('margin-left')==='auto',right=s.get('margin-right')==='auto',free=Math.max(0,basis-value.width-e.mleft-e.mright);if(left)e.mleft=free/(right?2:1);if(right)e.mright=free/(left?2:1)}
  if(!rendered(element)||!computedStyleDocumentAvailable(element)||!computedStyleAvailable(element)){value.width=0;return value;}
  if(s.display==='table')return tableSize(element,value);
  const parent=geometryParent(element),rawHeight=s.get('height');let height=rawHeight?.endsWith('%')&&!definiteGeometryHeight(parent)?null:s.length(rawHeight,rawHeight?.endsWith('%')&&parent?size(parent).height:0);
  // Opposing insets stretch an auto-sized non-replaced absolute box in its
  // containing padding box. Querying it first must also complete parent flow.
  if(height===null&&['absolute','fixed'].includes(s.position)&&!replacedGeometryTags.has(tag(element))){const container=positionedContainer(element),cb=container?size(container):null,basis=cb?cb.height-cb.edges.btop-cb.edges.bbottom:host.viewport().height,top=s.length(s.get('top'),basis),bottom=s.length(s.get('bottom'),basis);if(top!==null&&bottom!==null)height=Math.max(0,basis-top-bottom-e.mtop-e.mbottom-(s.get('box-sizing')==='border-box'?0:e.ptop+e.pbottom+e.btop+e.bbottom))}
  value.height=height===null?0:height+(s.get('box-sizing')==='border-box'?0:e.ptop+e.pbottom+e.btop+e.bbottom);
  const own=controlSize(element);if(own){value.height=height===null?own.height:value.height;return value}
  const text=textOf(element),font=textInfo(element,text),contentWidth=Math.max(0,value.width-e.pleft-e.pright-e.bleft-e.bright);
  const generated=pseudo=>{const entries=uncachedCSSDeclarations(element,pseudo),get=name=>entries.find(e=>e.name===name)?.value;if(get('display')==='none'||!['\"\"',"''"].includes(get('content'))||['absolute','fixed'].includes(get('position')))return 0;return (s.length(get('height'))||0)+(s.length(get('padding-top'),value.width)||0)+(s.length(get('padding-bottom'),value.width)||0)};
  let textLines=text?1:0,lastTextWidth=0,lineText='';const wrapping=!['nowrap','pre'].includes(s.inherited('white-space'));if(text)for(const word of text.split(' ')){const candidate=lineText?lineText+' '+word:word,advance=textInfo(element,candidate).width;if(wrapping&&lineText&&advance>contentWidth){textLines++;lineText=word;lastTextWidth=textInfo(element,word).width}else {lineText=candidate;lastTextWidth=advance}}
  let cursor=generated('before'),margin=0,lineWidth=lastTextWidth,lineHeight=textLines*font.height,line=[],adjoiningMargins=[];
  const collapsedMargin=extra=>{const values=[margin,...adjoiningMargins,...extra];return Math.max(0,...values)+Math.min(0,...values)};
  const flush=()=>{if(!lineHeight)return;for(const child of line){const box=size(child),c=state(child);value.positions.set(child,{x:lineWidth===0?0:value.positions.get(child)?.x||0,y:cursor+(c.display==='inline'||replacedGeometryTags.has(tag(child))?(tag(child)==='BUTTON'&&!textContent(child)?font.ascent-box.height/2:tag(child)==='PROGRESS'?font.ascent+unit(state(child).fontSize*.2)-box.height:Math.max(0,lineHeight-box.height)):0)})}cursor+=lineHeight;line=[];lineWidth=0;lineHeight=0};
  for(const child of children(element)){
   if(elementSlot(child)?.type!=='element')continue;const c=state(child);if(!rendered(child))continue;
   if(['absolute','fixed'].includes(c.position)){value.positions.set(child,{x:0,y:cursor+lineHeight});continue}
   const box=size(child),ce=box.edges,isInline=c.display==='inline'||c.display==='inline-block'||replacedGeometryTags.has(tag(child))||tag(child)==='PROGRESS';
   if(isInline){const w=box.width+ce.mleft+ce.mright;if(lineWidth&&lineWidth+w>contentWidth)flush();lineHeight=Math.max(lineHeight,font.height,box.height+ce.mtop+ce.mbottom);value.positions.set(child,{x:lineWidth+ce.mleft,y:cursor+ce.mtop});line.push(child);lineWidth+=w;continue}
   flush();if(box.height===0&&ce.btop===0&&ce.bbottom===0&&ce.ptop===0&&ce.pbottom===0){adjoiningMargins.push(ce.mtop,ce.mbottom);value.positions.set(child,{x:ce.mleft,y:cursor+collapsedMargin([])});continue}cursor+=collapsedMargin([ce.mtop]);adjoiningMargins=[];value.positions.set(child,{x:ce.mleft,y:cursor});cursor+=box.height;margin=ce.mbottom;
  }
  flush();cursor+=collapsedMargin([])+generated('after');
  // Closed disclosure content still has queryable boxes. Only the summary
  // contributes to the disclosure's visible normal-flow height.
  if(tag(element)==='DETAILS'&&host.getAttribute(elementSlot(element).nodeId,'open')===null){const summary=children(element).find(node=>tag(node)==='SUMMARY');if(summary){const box=size(summary);cursor=(value.positions.get(summary)?.y||0)+box.height+box.edges.mbottom}}
  value.contentHeight=cursor;
  value.height=(height===null?cursor:height)+(s.get('box-sizing')==='border-box'&&height!==null?0:e.ptop+e.pbottom+e.btop+e.bbottom);
  const min=s.length(s.get('min-height')),max=s.length(s.get('max-height'));if(min!==null)value.height=Math.max(value.height,min);if(max!==null)value.height=Math.min(value.height,max);return value;
 };
 const rect=element=>{
  const cache=styleReadCache.rects;if(cache.has(element))return cache.get(element);
  const value={x:0,y:0,left:0,top:0,width:0,height:0};cache.set(element,value);
  if(!rendered(element)||!computedStyleDocumentAvailable(element)||!computedStyleAvailable(element)){value.width=value.height=0;value.right=value.bottom=0;return value}
  const s=state(element),box=size(element);value.width=box.width;value.height=box.height;
  const parent=geometryParent(element),parentRect=parent?rect(parent):{x:0,y:0,width:host.viewport().width,height:host.viewport().height},parentBox=parent?size(parent):null;
  const local=parentBox?.positions.get(element)||{x:0,y:0};
  value.x=parentRect.x+(parentBox?parentBox.edges.bleft+parentBox.edges.pleft:0)+local.x;value.y=parentRect.y+(parentBox?parentBox.edges.btop+parentBox.edges.ptop:0)+local.y;
  if(['absolute','fixed'].includes(s.position)){
   const ancestor=positionedContainer(element);
   const r=ancestor?rect(ancestor):{x:0,y:0,...host.viewport()},a=ancestor?size(ancestor).edges:{bleft:0,btop:0};
   const left=s.length(s.get('left'),r.width),right=s.length(s.get('right'),r.width),top=s.length(s.get('top'),r.height),bottom=s.length(s.get('bottom'),r.height);
   if(left!==null)value.x=r.x+a.bleft+left;else if(right!==null)value.x=r.x+r.width-right-value.width;
   if(top!==null)value.y=r.y+a.btop+top;else if(bottom!==null)value.y=r.y+r.height-bottom-value.height;
   value.x+=box.edges.mleft;value.y+=box.edges.mtop;
  }else if(s.position==='relative'){
   const left=s.length(s.get('left'),parentRect.width),right=s.length(s.get('right'),parentRect.width),top=s.length(s.get('top'),parentRect.height),bottom=s.length(s.get('bottom'),parentRect.height);value.x+=left??-(right||0);value.y+=top??-(bottom||0);
  }
  value.left=value.x;value.top=value.y;value.right=value.x+value.width;value.bottom=value.y+value.height;
  // Offset coordinates use the offset parent's padding edge, not the DOM
  // parent's box. A static BODY denotes the initial containing block.
  let offsetParent=null;if(s.position!=='fixed')for(let p=parent;p;p=geometryParent(p))if(state(p).position!=='static'||['BODY','TD','TH','TABLE'].includes(tag(p))){offsetParent=p;break}
  const origin=offsetParent&&!(tag(offsetParent)==='BODY'&&state(offsetParent).position==='static')?rect(offsetParent):null,oe=origin?size(offsetParent).edges:null;
  value.clientWidth=Math.max(0,value.width-box.edges.bleft-box.edges.bright);value.clientHeight=Math.max(0,value.height-box.edges.btop-box.edges.bbottom);
  value.offsetLeft=value.x-(origin?origin.x+oe.bleft:0);value.offsetTop=value.y-(origin?origin.y+oe.btop:0);return value;
 };
 const hasBox=element=>withStyleReadCache(()=>foreignCSSObservation(element,'box')??(computedStyleDocumentAvailable(element)&&computedStyleAvailable(element)&&rendered(element)));
 return {width,rect,state,size,hasBox};
})();
