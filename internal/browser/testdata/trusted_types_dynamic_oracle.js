(() => {
  const out={},calls=[];let rewrite=false;
  trustedTypes.createPolicy('default',{createScript:(...a)=>{calls.push(a);return rewrite?a[0].replace('42','43'):a[0]},createHTML:(...a)=>{calls.push(a);return a[0]}});
  const attempt=(name,fn)=>{calls.length=0;try{const v=fn();out[name]={value:v===undefined?'undefined':v,calls:calls.slice()}}catch(e){out[name]={error:e.name,calls:calls.slice()}}};
  attempt('functionSourceViaEval',()=>typeof eval('(function anonymous(\n) {\nreturn 42\n})'));
  attempt('nestedCoercion',()=>Function({toString(){eval('1+1');return 'return 42'}})());
  attempt('derivedConstructor',()=>new (class extends Function{})('return 42')());
  attempt('constructorParent',()=>Object.getPrototypeOf(Object.getPrototypeOf(async function(){}).constructor)('return 42')());
  rewrite=true;
  attempt('rewriteEval',()=>eval('42'));
  attempt('rewriteFunction',()=>Function('return 42')());
  attempt('rewriteGenerator',()=>Object.getPrototypeOf(function*(){}).constructor('yield 42')().next().value);
  rewrite=false;
  const sources=['(function x(){})','(function(){})',' (function anonymous(){})','\n(function anonymous(){})','(function anonymous(){})','(function* x(){})','(async function x(){})','(async function* x(){})','(function ','(async function','(functionx)','(()=>42)','function anonymous(){}','(async function anonymous(){})','(function* anonymous(){})','(async function* anonymous(){})','(function anonymousX(){})','(function anonymous','(function anonymous (){})','(function anonymous\t(){})'];
  for(let i=0;i<sources.length;i++)attempt('sourceLabel'+i,()=>{try{eval(sources[i])}catch{};return calls.map(x=>x[2])});
  attempt('timerConversion',()=>clearTimeout(setTimeout({toString(){calls.push(['handler']);return '42'}},{valueOf(){calls.push(['delay']);return 10000}})));
  attempt('timerBadDelay',()=>setTimeout('42',{valueOf(){calls.push(['delay']);throw new RangeError()}}));
  attempt('innerHTMLUndefined',()=>{document.querySelector('#box').innerHTML=undefined;return document.querySelector('#box').innerHTML});
  attempt('outerHTMLUndefined',()=>{document.createElement('div').outerHTML=undefined});
  attempt('scriptInnerTextNull',()=>{document.createElement('script').innerText=null});
  attempt('scriptTextNull',()=>{document.createElement('script').text=null});
  attempt('scriptTextContentNull',()=>{document.createElement('script').textContent=null});
  const p=trustedTypes.createPolicy('allowed',{createHTML:s=>s,createScript:s=>s});
  const name=Object.getOwnPropertyDescriptor(TrustedHTML,'name');
  attempt('constructorNameCannotConferBrand',()=>{Object.defineProperty(TrustedHTML,'name',{value:'TrustedScript'});const v=p.createHTML('42');return [trustedTypes.isHTML(v),trustedTypes.isScript(v),eval(v)===v]});
  Object.defineProperty(TrustedHTML,'name',name);
  return out;
})()
