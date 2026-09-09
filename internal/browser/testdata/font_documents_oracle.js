(async()=>{
 if(typeof document!=='object')return {available:false};
 const a=document.implementation.createHTMLDocument('a'),b=document.implementation.createHTMLDocument('b');
 const out={available:true,sets:[a.fonts===a.fonts,a.fonts===b.fonts,a.fonts===document.fonts,a.fonts.status,b.fonts.status]};
 let readyA=false,readyB=false;a.fonts.ready.then(()=>{readyA=true});b.fonts.ready.then(()=>{readyB=true});
 await new Promise(r=>setTimeout(r,20));out.ready=[readyA,readyB];
 const face=new FontFace('Example','local(Example)');a.fonts.add(face);out.sizes=[a.fonts.size,b.fonts.size,document.fonts.size];
 return out;
})()
