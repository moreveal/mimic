// Resolve local use references directly from canonical DOM state. There is no
// mutable clone tree: edits to the referenced geometry are immediately visible.
const svgUseStack=new Set();
const svgUseBox=(n,depth)=>{
 const href=attr(n,'href')??attr(n,'xlink:href')??'';if(!href)return null;if(!href.startsWith('#'))svgFail('use','external referenced documents are unsupported');
 const id=href.slice(1);let root=n;while(parent(root)&&isDOMNode(parent(root)))root=parent(root);
 const find=node=>{for(const child of childElements(node)){if(attr(child,'id')===id)return child;const found=find(child);if(found)return found}return null};
 // Document has no Element slot in the surface; enter through its root element.
 const reference=root===document?document.getElementById(id):find(root);if(!reference||!elementSlot(reference)||elementSlot(reference).namespaceURI!==ns||displayNone(reference)||svgUseStack.has(reference)||reference===n)return null;
 svgUseStack.add(reference);try{
  const inspect=node=>{if(['text','tspan','textPath','foreignObject','image','svg'].includes(tag(node)))svgFail('use','referenced text, nested viewport or external-content context is unsupported');for(const entry of computedCSSDeclarations(node))if(['x','y','width','height','rx','ry','r','cx','cy'].includes(entry.name)&&/%|em|ex/.test(entry.value))svgFail('use','instance-relative CSS geometry is unsupported');for(const k of ['x','y','width','height','rx','ry','r','cx','cy'])if(/%|em|ex/.test(attr(node,k)||''))svgFail('use','instance-relative attribute geometry is unsupported');for(const child of childElements(node))inspect(child)};inspect(reference);
  let b;if(tag(reference)==='symbol'){b=null;for(const child of childElements(reference)){if(displayNone(child))continue;b=union(b,transformBox(bounds(child,depth+1),transform(child,bounds(child,depth+1))))}
   const v=numbers(attr(reference,'viewBox'));if(v.length===4&&v[2]>0&&v[3]>0){const w=length(n,'width','x'),h=length(n,'height','y');let sx=w/v[2],sy=h/v[3],ox=0,oy=0;const par=attr(reference,'preserveAspectRatio')||'xMidYMid meet';if(par!=='none'){sx=sy=par.includes('slice')?Math.max(sx,sy):Math.min(sx,sy);ox=(w-v[2]*sx)*(par.includes('xMin')?0:par.includes('xMax')?1:.5);oy=(h-v[3]*sy)*(par.includes('YMin')?0:par.includes('YMax')?1:.5)}b=transformBox(b,[sx,0,0,sy,ox-v[0]*sx,oy-v[1]*sy])}
  }else{b=bounds(reference,depth+1);b=transformBox(b,transform(reference,b))}
  return transformBox(b,[1,0,0,1,length(n,'x'),length(n,'y','y')]);
 }finally{svgUseStack.delete(reference)}
};
