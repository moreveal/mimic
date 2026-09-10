// The configured environment currently owns one screen at the origin.
(()=>{
 if(typeof ScreenOrientation!=="function")return;
 const orientation=new EventTarget();Object.setPrototypeOf(orientation,ScreenOrientation.prototype);
 const getter=(prototype,key,read)=>{markNative(read,key,'get ');Object.defineProperty(prototype,key,{get:read,configurable:true,enumerable:true})};
 for(const [key,value] of [['availLeft',0],['availTop',0],['isExtended',false]])getter(Screen.prototype,key,function(){if(this!==screen)throw new TypeError('Illegal invocation');return value});
 getter(Screen.prototype,'orientation',function(){if(this!==screen)throw new TypeError('Illegal invocation');return orientation});
 getter(ScreenOrientation.prototype,'type',function(){if(this!==orientation)throw new TypeError('Illegal invocation');const s=host.screen();return s.width>=s.height?'landscape-primary':'portrait-primary'});
 getter(ScreenOrientation.prototype,'angle',function(){if(this!==orientation)throw new TypeError('Illegal invocation');return 0});
 for(const [prototype,target] of [[Screen.prototype,screen],[ScreenOrientation.prototype,orientation]]){let handler=null;Object.defineProperty(prototype,'onchange',{get(){if(this!==target)throw new TypeError('Illegal invocation');return handler},set(value){if(this!==target)throw new TypeError('Illegal invocation');handler=typeof value==='function'?value:null},enumerable:true,configurable:true})}
 if(typeof Notification==='function')getter(Notification,'permission',function(){return host.notificationPermission()});
 Object.defineProperty(globalThis,'clientInformation',{get:()=>navigator,enumerable:true,configurable:true});
 getter(HTMLElement.prototype,'tabIndex',function(){const slot=elementSlot(this);if(!slot||slot.namespaceURI!=='http://www.w3.org/1999/xhtml')throw new TypeError('Illegal invocation');const match=/^[\t\n\f\r ]*([+-]?\d+)/.exec(this.getAttribute('tabindex')||'');if(match){const value=Number(match[1]);if(value>=-2147483648&&value<=2147483647)return value}if(['A','AREA','INPUT','BUTTON','SELECT','TEXTAREA','IFRAME','AUDIO','VIDEO','OBJECT'].includes(this.tagName))return 0;return -1});
 const descriptor=Object.getOwnPropertyDescriptor(HTMLElement.prototype,'tabIndex');descriptor.set=function(value){if(!elementSlot(this))throw new TypeError('Illegal invocation');this.setAttribute('tabindex',String(Number(value)>>0))};markNative(descriptor.set,'tabIndex','set ');Object.defineProperty(HTMLElement.prototype,'tabIndex',descriptor);
})();
