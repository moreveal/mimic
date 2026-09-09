(()=>{
 const ns='http://www.w3.org/2000/svg',out={},make=(tag,attrs={},children=[])=>{const n=document.createElementNS(ns,tag);for(const [k,v] of Object.entries(attrs))n.setAttribute(k,String(v));for(const c of children)n.appendChild(c);return n},box=n=>{const b=n.getBBox();return [b.x,b.y,b.width,b.height].map(x=>Math.round(x*1e5)/1e5)},run=(name,fn)=>{try{out[name]=fn()}catch(e){out[name]={error:e.name}}};
 const svg=make('svg',{width:200,height:100});document.body.appendChild(svg);
 for(const [name,tag,attrs] of [['rect','rect',{x:10,y:20,width:30,height:40}],['negative','rect',{x:-5,y:-7,width:3,height:2}],['zero','rect',{x:10,y:20,width:0,height:30}],['invalid','rect',{width:-10,height:20}],['circle','circle',{cx:20,cy:30,r:10}],['ellipse','ellipse',{cx:15,cy:25,rx:5,ry:10}],['line','line',{x1:20,y1:10,x2:-10,y2:30}],['polyline','polyline',{points:'10,20 30,5 -10,50'}],['polygon','polygon',{points:'10,20 30,5 -10,50'}],['path','path',{d:'M10 20h30v40h-30z'}],['quadratic','path',{d:'M0 0 Q50 100 100 0'}],['cubic','path',{d:'M0 0 C0 100 100 100 100 0'}],['arc','path',{d:'M0 0 A50 25 0 0 1 100 0'}]]){
  run(name,()=>{const n=make(tag,attrs);svg.appendChild(n);const result={tag:Object.prototype.toString.call(n),graphics:n instanceof SVGGraphicsElement,geometry:n instanceof SVGGeometryElement,box:box(n)};n.remove();return result});
 }
 const r=()=>make('rect',{x:10,y:20,width:30,height:40});
 run('empty',()=>{const g=make('g');svg.appendChild(g);return box(g)});
 run('union',()=>{const g=make('g',{},[r(),make('circle',{cx:0,cy:0,r:5})]);svg.appendChild(g);return box(g)});
 run('transforms',()=>{const a=r();a.setAttribute('transform','translate(5 10) scale(2 3)');const g=make('g',{transform:'translate(100 200)'},[a]);svg.appendChild(g);return {self:box(a),group:box(g)}});
 run('nested',()=>{const g=make('g',{},[make('g',{transform:'translate(5 10)'},[make('g',{transform:'scale(2)'},[r()])])]);svg.appendChild(g);return box(g)});
 run('rotate',()=>{const n=r();n.setAttribute('transform','rotate(90)');const g=make('g',{},[n]);svg.appendChild(g);return box(g)});
 run('detached',()=>box(make('g',{},[r()])));
 run('detachedRoot',()=>box(make('svg',{},[r()])));
 run('hidden',()=>{const g=make('g',{},[r(),make('rect',{x:100,y:100,width:10,height:10,display:'none'}),make('rect',{x:60,y:70,width:10,height:10,visibility:'hidden'})]);svg.appendChild(g);const before=box(g);g.style.display='none';return {before,after:box(g)}});
 run('noneSelf',()=>{const n=r();n.style.display='none';svg.appendChild(n);return box(n)});
 run('defs',()=>{const g=make('g',{},[make('defs',{},[make('rect',{width:1000,height:1000})]),r()]);svg.appendChild(g);return box(g)});
 run('viewBox',()=>{const s=make('svg',{width:200,height:100,viewBox:'0 0 400 200'},[make('rect',{x:'10%',y:'20%',width:'50%',height:'25%'})]);document.body.appendChild(s);const b=box(s);s.remove();return b});
 run('units',()=>{const n=make('rect',{x:'1in',y:'1cm',width:'2pt',height:'3mm'});svg.appendChild(n);return box(n)});
 run('mutation',()=>{const n=r();svg.appendChild(n);const a=n.getBBox();n.setAttribute('width','50');const b=n.getBBox();a.width=999;return {distinct:a!==b,old:[a.x,a.y,a.width,a.height],next:box(n),tag:Object.prototype.toString.call(b)}});
 run('options',()=>{const n=r();svg.appendChild(n);let reads=0;const b=n.getBBox({get fill(){reads++;return false},get stroke(){reads++;return true}});return {reads,box:[b.x,b.y,b.width,b.height]}});
 run('receiver',()=>{const method=SVGGraphicsElement.prototype.getBBox,errors=[];for(const v of [{},document.body,Object.create(SVGGraphicsElement.prototype)]){try{method.call(v);errors.push('accepted')}catch(e){errors.push(e.name)}}return {errors,length:method.length,name:method.name,own:Object.hasOwn(SVGGElement.prototype,'getBBox')}});
 run('foreign',()=>{const f=document.createElement('iframe');document.body.appendChild(f);const d=f.contentDocument,s=d.createElementNS(ns,'svg'),n=d.createElementNS(ns,'rect');for(const [k,v] of Object.entries({x:10,y:20,width:30,height:40}))n.setAttribute(k,v);s.appendChild(n);d.body.appendChild(s);const b=SVGGraphicsElement.prototype.getBBox.call(n);const value=[b.x,b.y,b.width,b.height];f.remove();return value});
 run('zeroFamily',()=>['rect','circle','ellipse','line','polyline','polygon','path'].map(t=>{const n=make(t,{x:10,y:20,cx:10,cy:20,rx:0,ry:10,r:-10,d:'M10 20',points:'10,20'});svg.appendChild(n);return [t,box(n)]}));
 run('zeroChildren',()=>{const g=make('g',{},[make('rect',{x:100,y:200,width:0,height:10}),r()]);svg.appendChild(g);return box(g)});
 run('hiddenAncestors',()=>{const n=r(),g=make('g',{display:'none'},[n]),s=make('svg',{display:'none'},[g]);document.body.appendChild(s);const result=[box(n),box(g),box(s)];s.remove();return result});
 run('defsSelf',()=>{const d=make('defs',{},[r()]);svg.appendChild(d);return box(d)});
 run('nestedViewport',()=>{const s=make('svg',{x:5,y:10,width:200,height:100,viewBox:'0 0 100 100'},[r()]),g=make('g',{},[s]);svg.appendChild(g);return {self:box(s),parent:box(g)}});
 run('cssGeometry',()=>{const n=make('rect',{x:1,y:2,width:3,height:4,style:'x: 10px; y: 20px; width: 30px; height: 40px'});svg.appendChild(n);return box(n)});
 run('skew',()=>{const n=r();n.setAttribute('transform','skewX(45)');const g=make('g',{},[n]);svg.appendChild(g);return box(g)});
 for(const [name,d] of Object.entries({smooth:'M0 0 Q50 100 100 0 T200 0',smoothCubic:'M0 0 C0 100 100 100 100 0 S200 -100 200 0',relative:'m10 20c0 40 40 40 40 0s40 -40 40 0',arcLarge:'M0 0 A50 25 0 1 0 100 0',arcRotated:'M0 0 A50 25 90 0 1 0 100',arcCompact:'M0 0A50 25 0 01100 0',partial:'M10 20L30 40L50',moveOnly:'M10 20 M100 200',closedMove:'M10 20z'}))run(name,()=>{const n=make('path',{d});svg.appendChild(n);return box(n)});
 run('hiddenMutation',()=>{const n=r(),g=make('g',{},[n]);svg.appendChild(g);const a=box(g);g.style.display='none';n.setAttribute('width','80');const b=box(g),c=box(n);g.style.display='';return [a,b,c,box(g)]});
 run('hiddenInitially',()=>{const g=make('g',{display:'none'},[r()]);svg.appendChild(g);return box(g)});
 run('hiddenAfterSiblingQuery',()=>{const n=r(),g=make('g',{},[n]),s=r();svg.appendChild(g);svg.appendChild(s);box(s);g.style.display='none';return box(g)});
 run('moveTail',()=>{const n=make('path',{d:'M10 20L30 40M100 200'});svg.appendChild(n);return box(n)});
 run('pointGroup',()=>{const g=make('g',{},[make('path',{d:'M100 200'}),make('line',{x1:100,y1:200,x2:100,y2:200}),r()]);svg.appendChild(g);return box(g)});
 run('caseSensitive',()=>['g','G','rect','RECT','foreignObject','foreignobject','x:rect'].map(t=>{const n=make(t);return [t,Object.prototype.toString.call(n),typeof n.getBBox]}));
 run('overrides',()=>{const n=r(),g=make('g',{},[n]);svg.appendChild(g);Object.defineProperties(n,{getAttribute:{value:()=>{throw Error('author getAttribute')}},localName:{value:'fake'},isConnected:{value:false}});Object.defineProperty(g,'children',{get(){throw Error('author children')}});return {rect:box(n),group:box(g)}});
 run('malformed',()=>{const a=make('polyline',{points:'10,20 30,40 bad 100,200'}),b=r();b.setAttribute('transform','translate(5 junk)');const g=make('g',{},[b]);svg.appendChild(a);svg.appendChild(g);return [box(a),box(g)]});
 run('shadow',()=>{const host=document.createElement('div');document.body.appendChild(host);const root=host.attachShadow({mode:'open'}),s=make('svg',{},[r()]);root.appendChild(s);const b=box(s);host.remove();return b});
 run('constructorOverride',()=>{const C=globalThis.SVGGElement;try{globalThis.SVGGElement=function Fake(){};const g=make('g',{},[r()]);svg.appendChild(g);return {canonical:g instanceof C,tag:Object.prototype.toString.call(g),box:box(g)}}finally{globalThis.SVGGElement=C}});
 svg.remove();return out;
})()
