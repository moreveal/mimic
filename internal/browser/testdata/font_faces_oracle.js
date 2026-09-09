(async()=>{
 const result={}, fail=fn=>{try{fn();return null}catch(e){return e.name}};
 const fonts=typeof document==='object'?document.fonts:self.fonts;
 result.set=[Object.prototype.toString.call(fonts),fonts=== (typeof document==='object'?document.fonts:self.fonts),fonts.size,fonts.status,fonts.ready===fonts.ready,(await fonts.ready)===fonts,fonts instanceof EventTarget];
 result.constructors=[fail(()=>new FontFace()),fail(()=>new FontFaceSet()),fail(()=>fonts.add({})),fail(()=>FontFace.prototype.load.call({}))];
 const a=new FontFace('Test Family','local("missing-test-font-4927")'), b=new FontFace('Other','url("data:font/woff;base64,AA==")');
 const props=['family','style','weight','stretch','unicodeRange','variant','featureSettings','variationSettings','display','ascentOverride','descentOverride','lineGapOverride','sizeAdjust','status'];
 result.defaults=props.map(k=>[k,a[k]===undefined?null:a[k]]);
 result.face=[Object.prototype.toString.call(a),a.loaded===a.loaded,a instanceof FontFace];
 result.add=[fonts.add(a)===fonts,fonts.add(a)===fonts,fonts.add(b)===fonts,fonts.size,fonts.has(a),Array.from(fonts).map(f=>f.family),Array.from(fonts.entries()).map(([k,v])=>k===v)];
 const calls=[];fonts.forEach(function(v,k,s){calls.push([v.family,k===v,s===fonts,this===a])},a);result.forEach=calls;
 result.delete=[fonts.delete(a),fonts.delete(a),fonts.size];fonts.clear();result.clear=[fonts.size,fonts.has(b)];
 result.emptyCheck=['12px serif','12px "unknown-font-4927"','bold 16px sans-serif','invalid',''].map(v=>{try{return fonts.check(v)}catch(e){return e.name}});
 result.emptyLoad=await fonts.load('12px serif').then(x=>x.length,e=>e.name);
 result.badLoad=await fonts.load('invalid').then(x=>x.length,e=>e.name);
 result.loadBefore=[a.status,a.load()===a.loaded,a.status];
 result.loadError=await a.loaded.then(()=>null,e=>e.name);result.loadAfter=[a.status,a.load()===a.loaded];
 const bad=new FontFace('Bad',new Uint8Array([0,1,2,3]));result.binaryStatus=bad.status;result.binaryError=await bad.loaded.then(()=>null,e=>e.name);
 return result;
})()
