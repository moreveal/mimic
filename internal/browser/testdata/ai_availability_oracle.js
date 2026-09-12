(async()=>{
 const out={},run=async(k,f)=>{let v;try{v=f();out[k]={promise:v instanceof Promise};out[k].value=await Promise.race([v,new Promise((_,reject)=>setTimeout(()=>reject(new Error('oracle timeout')),1500))])}catch(e){out[k]={...out[k],error:e.name,message:e.message}}};
 for(const name of ['Summarizer','LanguageModel','Writer','Rewriter','Translator','LanguageDetector']){
  const C=globalThis[name];out[name]={type:typeof C};if(typeof C!=='function')continue;
  out[name].own=Object.getOwnPropertyNames(C);out[name].prototypeHasAvailability=Object.hasOwn(C.prototype,'availability');
  for(const m of ['availability','create']){const d=Object.getOwnPropertyDescriptor(C,m);out[name][m]=d?{name:d.value.name,length:d.value.length,writable:d.writable,enumerable:d.enumerable,configurable:d.configurable,constructable:!!d.value.prototype}:null}
 }
 for(const name of ['Summarizer','LanguageModel','Translator','LanguageDetector']){
  const C=globalThis[name];if(!C)continue;
  for(const m of ['availability','create']){
   for(const [key,value] of [['missing',undefined],['null',null],['number',1],['valid',name==='Translator'?{sourceLanguage:'en',targetLanguage:'fr'}:{}]])await run(name+'-'+m+'-'+key,()=>C[m](value));
   const seen=[];await run(name+'-'+m+'-reads',()=>C[m](new Proxy(name==='Translator'?{sourceLanguage:'en',targetLanguage:'fr'}:{},{get(t,k){seen.push(String(k));return t[k]}})));out[name+'-'+m+'-optionReads']=seen;
  }
 }
 if(typeof Summarizer!=='function')return out;
 for(const [k,v] of [['missing',undefined],['null',null],['number',1],['typeBad',{type:'bad'}],['formatBad',{format:'bad'}],['lengthBad',{length:'bad'}],['languageBad',{outputLanguage:'xx-invalid'}],['valid',{type:'key-points',format:'markdown',length:'short'}]])await run('availability-'+k,()=>Summarizer.availability(v));
 for(const receiver of [null,{},globalThis])await run('receiver-'+(receiver===null?'null':receiver===globalThis?'window':'object'),()=>Reflect.apply(Summarizer.availability,receiver,[]));
 for(const [key,value]of [['expectedInputLanguages',null],['expectedContextLanguages',1],['outputLanguage',Symbol('x')]])await run('invalid-'+key,()=>Summarizer.availability({[key]:value}));
 for(const [key,value]of [['monitor',1],['signal',{}]])await run('invalid-create-'+key,()=>Summarizer.create({[key]:value}));
 await run('create-aborted',()=>{const c=new AbortController();c.abort('sentinel');return Summarizer.create({signal:c.signal})});
 let modelCase=0;for(const [key,value] of [['temperature',1n],['topK',1n],['temperature',Infinity],['expectedInputs',null],['expectedOutputs',1],['expectedInputs',[{type:'bad'}]],['expectedInputs',[{type:'text',languages:['en']}]],['expectedInputs',[{}]],['expectedOutputs',[{type:'image'}]]])await run('model-'+(modelCase++)+'-'+key,()=>LanguageModel.availability({[key]:value}));
 await run('model-prompts',()=>LanguageModel.create({initialPrompts:[{role:'bad',content:'x'}]}));
 const iterReads=[];await run('sequence-custom',()=>Summarizer.availability({expectedInputLanguages:{get [Symbol.iterator](){iterReads.push('iterator');return function*(){yield 'en'}}}}));out.iteratorReads=iterReads;
 const nestedReads=[];await run('model-nested',()=>LanguageModel.availability({expectedInputs:[new Proxy({type:'text'},{get(t,k){nestedReads.push(String(k));return t[k]}})]}));out.nestedReads=nestedReads;
 const promptReads=[];await run('model-prompt-reads',()=>LanguageModel.create({initialPrompts:[new Proxy({role:'user',content:'x'},{get(t,k){promptReads.push(String(k));return t[k]}})]}));out.promptReads=promptReads;
 const seen=[];await run('options',()=>Summarizer.availability(new Proxy({},{get(t,k){seen.push(String(k));return undefined}})));out.optionReads=seen;
 await run('getterThrow',()=>Summarizer.availability({get type(){throw new Error('sentinel')}}));
 return out;
})()
