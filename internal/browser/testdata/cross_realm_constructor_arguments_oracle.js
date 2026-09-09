(async()=>{
 const frame=document.createElement('iframe');document.body.append(frame);const child=frame.contentWindow;
 const result={},parentObject={value:17};let reads=0;Object.defineProperty(parentObject,'read',{get(){reads++;return this.value}});parentObject.self=parentObject;
 const set=new child.Set([parentObject,parentObject,23]);result.set={size:set.size,has:set.has(parentObject),first:set.values().next().value===parentObject};
 const C=child.Function('arg','this.arg=arg;arg.changed=1;this.read=arg.read;this.cycle=arg.self===arg');const instance=new C(parentObject);
 result.argument={identity:instance.arg===parentObject,changed:parentObject.changed,read:instance.read,reads,cycle:instance.cycle};
 const Returns=child.Function('arg','return arg');result.returnParent=new Returns(parentObject)===parentObject;
 let receiver;function callback(arg){receiver=this;return arg}
 const Calls=child.Function('fn','arg','this.returned=fn.call(this,arg)');const called=new Calls(callback,parentObject);result.callback={receiver:receiver===called,returned:called.returned===parentObject};
 const oldC=C,oldInstance=instance;await new Promise(resolve=>{frame.onload=resolve;frame.srcdoc='<p>new document</p>'});
 const after=new oldC(parentObject);result.afterNavigation={oldReference:oldInstance.arg===parentObject,newInstance:after.arg===parentObject,prototype:Object.getPrototypeOf(after)===oldC.prototype,oldRead:oldInstance.read};
 result.environment={secureContextState:isSecureContext,isolationState:crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus(),viewport:{width:innerWidth,height:innerHeight,deviceScaleFactor:devicePixelRatio}};return result;
})()
