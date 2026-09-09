(()=>{
 const stringify=Function.prototype.toString,source='function sample() {\n  return "[native code]  keep spacing";\n}',sample=eval('('+source+')');
 let reads=0;const bound=(function named(){}).bind(null);Object.defineProperty(bound,'name',{get(){reads++;throw Error('name getter')}});
 const proxy=new Proxy(function named(){},{get(){reads++;throw Error('proxy get')}}),revocable=Proxy.revocable(function(){},{});revocable.revoke();
 const result={source:stringify.call(sample),bound:stringify.call(bound),proxy:stringify.call(proxy),revoked:stringify.call(revocable.proxy),reads};
 try{stringify.call({})}catch(error){result.nonCallable=error.name}
 result.platform=stringify.call(document.createElement);return result;
})()
