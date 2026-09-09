(async()=>{
 const frame=document.createElement('iframe');document.body.append(frame);const child=frame.contentWindow;
 const result={builtins:{}};
 for(const name of ['Object','Array','Error']){const C=child[name],value=new C();result.builtins[name]={instance:value instanceof C,prototype:Object.getPrototypeOf(value)===C.prototype,tag:Object.prototype.toString.call(value)}}
 const C=child.Function('a','this.a=a;this.target=new.target'),Other=child.Function('');
 const value=new C(17);result.user={value:value.a,target:value.target===C,instance:value instanceof C,prototype:Object.getPrototypeOf(value)===C.prototype};
 const changed=Reflect.construct(C,[23],Other);result.changed={value:changed.a,target:changed.target===Other,instance:changed instanceof Other,prototype:Object.getPrototypeOf(changed)===Other.prototype};
 const object=new child.Object();child.savedObject=object;const returns=child.Function('return savedObject');result.returnIdentity=new returns()===object;
 const error=new child.Error('sentinel');child.savedError=error;const fail=child.Function('throw savedError');try{new fail()}catch(e){result.error={identity:e===error,instance:e instanceof child.Error,message:e.message}}
 const primitive=child.Function('throw 19');try{new primitive()}catch(e){result.primitive=e}
 class Local extends C{};const local=new Local(31);result.localSubclass={value:local.a,target:local.target===Local,instance:local instanceof Local,prototype:Object.getPrototypeOf(local)===Local.prototype};
 const ordinary=child.Function('return ()=>1')();try{new ordinary()}catch(e){result.nonConstructor=e.name}
 child.counter=0;const hazardous=child.Function('return {toString(){counter++;throw 1},get dangerous(){counter++;throw 2}}')();result.sideEffects={keys:Reflect.ownKeys(hazardous),accessor:typeof Object.getOwnPropertyDescriptor(hazardous,'dangerous').get,counter:child.counter};
 result.environment={secureContextState:isSecureContext,isolationState:crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus(),viewport:{width:innerWidth,height:innerHeight,deviceScaleFactor:devicePixelRatio}};return result;
})()
