// CSS transform matrices act on a child's contribution to its parent box.
// The child's own getBBox remains in its local coordinate system.
const cssTransform=(n,box,raw)=>{
 const entries=computedCSSDeclarations(n),get=key=>entries.find(e=>e.name===key)?.value;
 const fail=reason=>{host.semanticMissingAt('svg_css_transform.js:5','SVG.cssTransformResolution',JSON.stringify({reason,value:String(raw).slice(0,256),origin:get('transform-origin'),referenceBox:get('transform-box'),nodeId:elementSlot(n).nodeId}));return unsupported('cssTransform')};
 const mode=get('transform-box')||'view-box';let reference;
 if(mode==='fill-box'||mode==='content-box')reference=box||[0,0,0,0];
 else if(mode==='view-box'){const [w,h]=viewport(n);reference=[0,0,w,h]}else return fail('unsupported-reference-box');
 const size=cssComputedFontSize(n);if(size===null)return fail('unresolved-font-size');
 let root=n;for(let p=parent(root);elementSlot(p)?.type==='element';p=parent(root))root=p;
 const rootSize=cssComputedFontSize(root),width=reference[2]-reference[0],height=reference[3]-reference[1];
 const len=(value,axis)=>{const v=cssResolveLength(value,{em:size,rem:rootSize,percent:axis?height:width});if(v===null||!Number.isFinite(v))return fail('unsupported-length');return Math.fround(v)};
 const angle=value=>{const m=new RegExp('^('+numberPattern+')(deg|rad|grad|turn)?$','i').exec(value);if(!m||(!m[2]&&Number(m[1])!==0))return fail('unsupported-angle');return Number(m[1])*({deg:Math.PI/180,grad:Math.PI/200,turn:2*Math.PI,rad:1}[m[2]?.toLowerCase()]||1)};
 // Range reduction and degree conversion precede sin/cos in gfx::Transform.
 const rotation=value=>{
  const m=new RegExp('^('+numberPattern+')(deg|rad|grad|turn)?$','i').exec(value);if(!m)return fail('unsupported-angle');
  let degrees=Number(m[1])*({deg:1,grad:.9,turn:360,rad:180/Math.PI}[m[2]?.toLowerCase()]||1),s,c;
  if(degrees>-90000000&&degrees<90000000){
   let octant=Math.trunc(degrees/45);
   if(octant===degrees/45){const q=Math.SQRT2/2;return [[1,0],[q,q],[0,1],[-q,q],[-1,0],[-q,-q],[0,-1],[q,-q]][octant&7]}
   if(degrees<0)--octant;degrees-=octant*45;if(octant&1)degrees=45-degrees;
   const radians=degrees*Math.PI/180;s=Math.sin(radians);c=Math.cos(radians);
   if((octant+1)&2)[s,c]=[c,s];if(octant&4)s=-s;if((octant+2)&4)c=-c;
  }else{const radians=(degrees%360)*Math.PI/180;s=Math.sin(radians);c=Math.cos(radians)}
  return [c,s];
 };
 let matrix=Array.isArray(raw)?raw:ident,rest=Array.isArray(raw)?'':String(raw).trim(),count=0;
 if(rest.toLowerCase()==='none')rest='';
 while(rest){if(++count>256)return fail('transform-list-limit');const match=/^([a-zA-Z0-9]+)\(\s*([^()]*)\)\s*/.exec(rest);if(!match)return fail('unsupported-transform-syntax');rest=rest.slice(match[0].length);const name=match[1].toLowerCase(),args=match[2].split(/\s*,\s*|\s+/).filter(Boolean);let m;
  const arity=(min,max=min)=>{if(args.length<min||args.length>max)fail('invalid-arity')};
  const numeric=()=>{const a=args.map(Number);if(!a.every(Number.isFinite))fail('invalid-number');return a};
  if(name==='matrix'){arity(6);m=numeric()}
  else if(name==='matrix3d'){arity(16);const a=numeric();if([2,3,6,7,8,9,11,14].some(i=>a[i]!==0)||a[10]!==1||a[15]!==1)return fail('nonplanar-matrix');m=[a[0],a[1],a[4],a[5],a[12],a[13]]}
  else if(name==='translatez'){arity(1);if(len(args[0],0)!==0)return fail('nonzero-translation-z');m=ident}
  else if(name==='translate3d'){arity(3);if(len(args[2],0)!==0)return fail('nonzero-translation-z');m=[1,0,0,1,len(args[0],0),len(args[1],1)]}
  else if(name==='scale3d'){arity(3);const a=numeric().map(Math.fround);if(a[2]!==1)return fail('nonunit-scale-z');m=[a[0],0,0,a[1],0,0]}
  else if(['translate','translatex','translatey'].includes(name)){arity(1,name==='translate'?2:1);m=[1,0,0,1,name==='translatey'?0:len(args[0],0),name==='translatey'?len(args[0],1):args[1]?len(args[1],1):0]}
  else if(['scale','scalex','scaley'].includes(name)){arity(1,name==='scale'?2:1);const a=numeric().map(Math.fround);m=[name==='scaley'?1:a[0],0,0,name==='scalex'?1:a[1]??a[0],0,0]}
  else if((name==='rotate'||name==='rotatez')){arity(1);const [c,s]=rotation(args[0]);m=[c,s,-s,c,0,0]}
  else if(['skew','skewx','skewy'].includes(name)){arity(1,name==='skew'?2:1);m=[1,name==='skewy'?Math.tan(angle(args[0])):args[1]?Math.tan(angle(args[1])):0,name==='skewy'?0:Math.tan(angle(args[0])),1,0,0]}
  else return fail('unsupported-transform-function:'+name);
  matrix=multiply(matrix,m);
 }
 let origin=String(get('transform-origin')??attr(n,'transform-origin')??'0 0').trim().split(/\s+/);
 if(origin.length===3){if(len(origin.pop(),0)!==0)return fail('nonzero-origin-z')}
 if(origin.length<1||origin.length>2)return fail('unsupported-origin');
 if(origin.length===1)origin=['top','bottom'].includes(origin[0])?['50%',origin[0]]:[origin[0],'50%'];
 if(['top','bottom'].includes(origin[0])||['left','right'].includes(origin[1]))origin.reverse();
 const keywords={left:'0%',top:'0%',center:'50%',right:'100%',bottom:'100%'};
 const x=reference[0]+len(keywords[origin[0]]||origin[0],0),y=reference[1]+len(keywords[origin[1]]||origin[1],1);
 return multiply(multiply([1,0,0,1,x,y],matrix),[1,0,0,1,-x,-y]);
};
