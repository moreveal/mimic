// A generated IDL shape must not silently pretend to implement an operation.
// Keep unsupported capabilities observable and attach the declaring interface.
svgBindings.add('SVGGraphicsElement.getBBox');
svgBindings.add('SVGTextContentElement.getComputedTextLength');
for(const type of ['SVGFEGaussianBlurElement','SVGFEDropShadowElement'])svgMethod(type,'setStdDeviation',function(x,y){const n=svgElementCheck(this);if(!globalThis[type].prototype.isPrototypeOf(n))throw new TypeError('Illegal invocation');x=svgFloat(x);y=svgFloat(y);svgAttr(n,'stdDeviation',x===y?String(x):x+' '+y)});
svgMethod('SVGMarkerElement','setOrientToAuto',function(){const n=svgElementCheck(this);if(tag(n)!=='marker')throw new TypeError('Illegal invocation');svgAttr(n,'orient','auto')});
svgMethod('SVGMarkerElement','setOrientToAngle',function(angle){const n=svgElementCheck(this);if(tag(n)!=='marker')throw new TypeError('Illegal invocation');svgSlot(angle,'SVGAngle');svgAttr(n,'orient',angle.valueAsString)});
for(const type of Object.getOwnPropertyNames(globalThis).filter(name=>/^SVG/.test(name)&&name!=='SVGElement')){
 const prototype=globalThis[type]?.prototype;if(!prototype)continue;
 for(const key of Object.getOwnPropertyNames(prototype)){
  if(key==='constructor'||svgBindings.has(type+'.'+key))continue;
  const d=Object.getOwnPropertyDescriptor(prototype,key);if(!d.get&&typeof d.value!=='function')continue;
  const fail=function(){if(!prototype.isPrototypeOf(this)||(!elementSlot(this)&&!svgSlots.has(this)&&!svgRectSlots.has(this)))throw new TypeError('Illegal invocation');svgFail(type+'.'+key,'SVG capability has an interface but no supported semantic implementation')};
  if(typeof d.value==='function')svgMethod(type,key,fail);else svgProp(type,key,fail,d.set?fail:undefined);
 }
}
