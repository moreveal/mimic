  // Bind semantic domains to the already-selected Blink exposure. Constructor
  // objects and descriptors come from generated data; wrappers carry only slots.
  const installNavigatorCapabilities=()=>{
    globalThis.__mimicDispatchInput=(nodeID,type)=>dispatchTrusted(wrap(nodeID)||document.body||globalThis,new Event(type,{bubbles:true,cancelable:true}));
    const members=globalThis.__mimicNavigatorMembers||[];
    delete globalThis.__mimicNavigatorMembers;
    const names=new Set('vendorSub productSub appCodeName doNotTrack userActivation scheduling geolocation bluetooth clipboard credentials ink devicePosture hid keyboard managed mediaCapabilities mediaSession mediaDevices permissions presentation serial serviceWorker virtualKeyboard wakeLock usb windowControlsOverlay xr storageBuckets locks connection storage webkitTemporaryStorage webkitPersistentStorage login protectedAudience deprecatedRunAdAuctionEnforcesKAnonymity'.split(' '));
    const slots=new WeakMap(),singletons=new Map();
    const native=(fn,name,prefix='')=>{Object.defineProperty(fn,'name',{value:prefix+name,configurable:true});markNative(fn,name,prefix);return fn};
    const state=(domain,...args)=>host.capabilityState(domain,...args);
    const secureContext=host.documentSecurity().secureContext;
    const conditions=globalThis.__mimicCapabilityConditions||{};delete globalThis.__mimicCapabilityConditions;
    const disabled=condition=>[condition?.RuntimeEnabled,condition?.ExposureFeatures?.Window].some(flag=>typeof flag==='string'&&!state('feature',flag));
    for(const [type,condition] of Object.entries(conditions))if(disabled(condition))delete globalThis[type];
    const check=(object,type)=>{const slot=slots.get(object);if(!slot||slot.type!==type)throw new TypeError('Illegal invocation');return slot};
    const create=(type,data={})=>{const ctor=globalThis[type];if(!ctor||!ctor.prototype)throw new TypeError('Unavailable interface '+type);if(!Object.hasOwn(ctor.prototype,Symbol.toStringTag))Object.defineProperty(ctor.prototype,Symbol.toStringTag,{value:type,configurable:true});const object=Object.create(ctor.prototype);slots.set(object,{type,...data});return object};
    const error=(name,message)=>new DOMException(message,name);
    const deny=(name,message)=>Promise.reject(error(name,message));
    const method=(type,name,implementation,async=false)=>{
      const proto=globalThis[type]?.prototype,d=proto&&Object.getOwnPropertyDescriptor(proto,name);
      if(!d||typeof d.value!=='function')return;
      const fn={[name](...args){const run=()=>{const slot=check(this,type);if(args.length<d.value.length)throw new TypeError(`Failed to execute '${name}' on '${type}': ${d.value.length} argument${d.value.length===1?'':'s'} required, but only ${args.length} present.`);return implementation.call(this,slot,...args)};return async?Promise.resolve().then(run):run()}}[name];
      Object.defineProperty(fn,'length',{value:d.value.length,configurable:true});native(fn,name);
      Object.defineProperty(proto,name,{...d,value:fn});
    };
    const attribute=(type,name,get,set)=>{
      const proto=globalThis[type]?.prototype,d=proto&&Object.getOwnPropertyDescriptor(proto,name);if(!d||!d.get)return;
      const getter={[name](){return get(check(this,type),this)}}[name];native(getter,name,'get ');
      const descriptor={...d,get:getter};
      if(d.set&&set){descriptor.set={[name](value){set(check(this,type),value,this)}}[name];native(descriptor.set,name,'set ')}
      Object.defineProperty(proto,name,descriptor);
    };
    const scalar=()=>state('identity');
    slots.set(nav,{type:'Navigator'});
    // A machine without battery/gamepad backends has observable, empty device
    // state. Battery promises and the manager are stable within this Navigator.
    let batteryPromise;
    if(typeof BatteryManager==='function'){
      const readBattery=()=>state('battery');
      for(const key of ['charging','level'])attribute('BatteryManager',key,()=>readBattery()[key]);
      for(const key of ['chargingTime','dischargingTime'])attribute('BatteryManager',key,()=>readBattery()[key]??Infinity);
      for(const key of ['onchargingchange','onchargingtimechange','ondischargingtimechange','onlevelchange'])attribute('BatteryManager',key,(_s,target)=>eventHandlerRecord(target,key.slice(2)).value,(_s,value,target)=>setEventHandlerValue(target,key.slice(2),value));
      const getBattery={getBattery(){try{check(this,'Navigator');if(!batteryPromise)batteryPromise=Promise.resolve(create('BatteryManager'));return batteryPromise}catch(e){return Promise.reject(e)}}}.getBattery;
      native(getBattery,'getBattery');
      const descriptor=Object.getOwnPropertyDescriptor(Navigator.prototype,'getBattery');
      if(descriptor)Object.defineProperty(Navigator.prototype,'getBattery',{...descriptor,value:getBattery});
    }
    method('Navigator','getGamepads',()=>{
      if(!documentPolicy.allowsFeature('gamepad'))throw error('SecurityError','Access to gamepads is disallowed by permissions policy.');
      return [null,null,null,null];
    });
    const domainSources=new Set(members.filter(m=>names.has(m.name)).map(m=>m.origin?.source));
    // Close the declaring Navigator domain too (not only its object-valued
    // entry point). In particular auction operations must not inherit a
    // successful null from the generic surface when there is no auction backend.
    for(const member of members)if(member.kind==='operation'&&domainSources.has(member.origin?.source)){
      method('Navigator',member.name,()=>{throw error('NotSupportedError',`Navigator.${member.name} backend is unavailable.`)},String(member.returnType).startsWith('Promise'));
    }
    method('Navigator','javaEnabled',()=>false);
    const capabilityInterfaces=globalThis.__mimicCapabilityInterfaces||{};delete globalThis.__mimicCapabilityInterfaces;
    const boundTypes=new Set(members.filter(m=>names.has(m.name)).map(m=>m.type));
    for(const type of ['PermissionStatus','StorageBucket','Lock','MediaDeviceInfo','DelegatedInkTrailPresenter','KeyboardLayoutMap'])boundTypes.add(type);
    // A deliberately absent subsystem fails explicitly, with the WebIDL return
    // convention, rather than inheriting semanticMissing's successful null.
    for(const type of boundTypes)for(const member of capabilityInterfaces[type]||[]){
      const unsupported=()=>{throw error('NotSupportedError',`${type}.${member.name} backend is unavailable.`)};
      if(member.kind==='operation')method(type,member.name,unsupported,String(member.returnType).startsWith('Promise'));
      if(member.kind==='attribute')attribute(type,member.name,unsupported,unsupported);
    }
    const quotaPrototype={};
    Object.defineProperty(quotaPrototype,Symbol.toStringTag,{value:'DeprecatedStorageQuota',configurable:true});
    for(const [name,length,fn] of [
      ['queryUsageAndQuota',1,function(success,failure){check(this,'DeprecatedStorageQuota');if(typeof success!=='function')throw new TypeError('The callback provided as parameter 1 is not a function.');setTimeout(()=>{const q=state('quota');if(q===null){if(typeof failure==='function')failure(error('NotSupportedError','The implementation did not support the requested type of object or operation.'));return}success(q.usage,q.quota)},0)}],
      ['requestQuota',1,function(bytes,success,failure){check(this,'DeprecatedStorageQuota');if(success!==undefined&&typeof success!=='function')throw new TypeError('The callback provided as parameter 2 is not a function.');setTimeout(()=>{const q=state('quota');if(q===null){if(typeof failure==='function')failure(error('NotSupportedError','The implementation did not support the requested type of object or operation.'));return}if(success)success(Math.min(Math.max(0,Number(bytes)||0),q.quota))},0)}]
    ]){const wrapper={[name](...args){if(!args.length)throw new TypeError(`Failed to execute '${name}' on 'DeprecatedStorageQuota': 1 argument required, but only 0 present.`);return fn.apply(this,args)}}[name];Object.defineProperty(wrapper,'length',{value:length,configurable:true});native(wrapper,name);Object.defineProperty(quotaPrototype,name,{value:wrapper,writable:true,enumerable:true,configurable:true})}
    for(const member of members){
      if(!names.has(member.name))continue;
      const d=Object.getOwnPropertyDescriptor(Navigator.prototype,member.name);if(!d)continue;
      const flags=[member.origin?.extended?.RuntimeEnabled,member.extended?.RuntimeEnabled].filter(f=>typeof f==='string');
      if(flags.some(flag=>!state('feature',flag))||disabled(conditions[member.type])){delete Navigator.prototype[member.name];continue}
      const getter={[member.name](){
        if(this!==nav)throw new TypeError('Illegal invocation');
        if(['vendorSub','productSub','appCodeName','doNotTrack'].includes(member.name))return scalar()[member.name];
        if(member.name==='deprecatedRunAdAuctionEnforcesKAnonymity')return false;
        const key=member.type==='DeprecatedStorageQuota'?member.type:member.name;
        if(!singletons.has(key)){
          let value;
          if(member.type==='DeprecatedStorageQuota'){value=Object.create(quotaPrototype);slots.set(value,{type:member.type})}
          else value=create(member.type);
          singletons.set(key,value);
        }
        return singletons.get(key);
      }}[member.name];
      native(getter,member.name,'get ');Object.defineProperty(Navigator.prototype,member.name,{...d,get:getter});
    }
    // Event-handler attributes use private slots and retain EventTarget's shared
    // listener machinery. No own fields leak through singleton objects.
    for(const type of new Set(members.filter(m=>names.has(m.name)).map(m=>m.type))){
      const proto=globalThis[type]?.prototype;if(!proto)continue;
      for(const name of Object.getOwnPropertyNames(proto))if(name.startsWith('on'))attribute(type,name,s=>s[name]||null,(s,v)=>s[name]=typeof v==='function'?v:null);
    }
    for(const name of ['isActive','hasBeenActive'])attribute('UserActivation',name,()=>state('activation')[name]);
    method('Scheduling','isInputPending',()=>state('activation').pending);
    for(const name of ['effectiveType','downlink','rtt','saveData'])attribute('NetworkInformation',name,()=>state('network')[name]);
    attribute('DevicePosture','type',()=>state('devices').posture||'continuous');

    // One permission source feeds query, location, clipboard and media.
    const permissionNames=new Set(globalThis.__mimicPermissionNames||[]);delete globalThis.__mimicPermissionNames;
    const permission=name=>documentPolicy.allowsFeature(name)?state('permission',name):'denied';
    const permissionStatuses=new Set();
    globalThis.__mimicPermissionChanged=name=>{for(const status of permissionStatuses){const s=slots.get(status);if(s.name===name&&s.lastState!==permission(name)){s.lastState=permission(name);dispatchTrusted(status,new Event('change'))}}};
    method('Permissions','query',(_s,descriptor)=>{
      if(!descriptor||!('name' in Object(descriptor)))throw new TypeError("Failed to execute 'query' on 'Permissions': Failed to read the 'name' property from 'PermissionDescriptor': Required member is undefined.");
      const name=String(descriptor.name);
      if(!permissionNames.has(name))throw new TypeError("Failed to execute 'query' on 'Permissions': Failed to read the 'name' property from 'PermissionDescriptor': The provided value '"+name+"' is not a valid enum value of type PermissionName.");
      const disabled={
        'ambient-light-sensor':['GenericSensorExtraClasses','GenericSensorExtraClasses flag is not enabled.'],
        'nfc':['WebNFC','Web NFC is not enabled.'],
        'system-wake-lock':['SystemWakeLock','System Wake Lock is not enabled.'],
        'speaker-selection':['SpeakerSelection','The Speaker Selection API is not enabled.'],
        'web-app-installation':['WebAppInstallation','The Web App Install API is not enabled.'],
        'geolocation-approximate':['ApproximateGeolocationPermission','Permission API support for approximate geolocation is not enabled.']
      }[name];
      if(disabled&&!state('feature',disabled[0]))throw new TypeError("Failed to execute 'query' on 'Permissions': "+disabled[1]);
      if(name==='push'&&!descriptor.userVisibleOnly)throw error('NotSupportedError',"Failed to execute 'query' on 'Permissions': Push Permission without userVisibleOnly:true isn't supported yet.");
      if(name==='fullscreen'&&!descriptor.allowWithoutGesture)throw new TypeError("Failed to execute 'query' on 'Permissions': Fullscreen Permission only supports allowWithoutGesture:true.");
      if(name==='top-level-storage-access'&&!descriptor.requestedOrigin)throw new TypeError("Failed to execute 'query' on 'Permissions': The requested origin is invalid.");
      const status=create('PermissionStatus',{name,lastState:permission(name)});permissionStatuses.add(status);return status;
    },true);
    attribute('PermissionStatus','state',s=>permission(s.name));attribute('PermissionStatus','name',s=>s.name);
    attribute('PermissionStatus','onchange',s=>s.onchange||null,(s,v)=>s.onchange=typeof v==='function'?v:null);
    const permissionPrompt=()=>new Promise(()=>{});
    for(const [name,message] of [['get','No credential type was specified in the request.'],['create',"Only exactly one of 'password', 'federated', and 'publicKey' credential types are currently supported."]])method('CredentialsContainer',name,(_s,options={})=>{
      if(options.signal?.aborted)throw options.signal.reason;
      if(name==='get'&&options.password&&options.mediation==='silent')return null;
      throw error('NotSupportedError',message);
    },true);
    method('CredentialsContainer','preventSilentAccess',()=>{state('preventSilent')},true);
    method('CredentialsContainer','store',()=>{throw error('NotSupportedError','Credential storage is unavailable.')},true);
    for(const name of ['read','readText'])method('Clipboard',name,()=>{
      if(permission('clipboard-read')==='prompt'&&state('presentation').mode==='headful')return permissionPrompt();
      if(permission('clipboard-read')!=='granted'){throw error('NotAllowedError',`Failed to execute '${name}' on 'Clipboard': Read permission denied.`)}
      if(name==='readText')return state('clipboard');
      throw error('NotSupportedError','Clipboard item transport is unavailable.');
    },true);
    method('Clipboard','writeText',(_s,value)=>{
      if(permission('clipboard-write')==='denied'||(permission('clipboard-write')!=='granted'&&!state('activation').isActive))throw error('NotAllowedError',"Failed to execute 'writeText' on 'Clipboard': Write permission denied.");
      state('clipboardWrite',String(value));
    },true);
    method('Clipboard','write',()=>{throw error('NotSupportedError','Clipboard item transport is unavailable.')},true);
    let watchID=0;const watches=new Set();
    const locate=(success,failure,options,watch)=>{
      if(typeof success!=='function')throw new TypeError('The callback provided as parameter 1 is not a function.');
      const id=++watchID;if(watch)watches.add(id);
      if(secureContext&&permission('geolocation')==='prompt'&&state('presentation').mode==='headful')return id;
      setTimeout(()=>{if(watch&&!watches.has(id))return;if(typeof failure==='function'){
        const denied=!secureContext||permission('geolocation')!=='granted';
        failure(create('GeolocationPositionError',{code:denied?1:2,message:!secureContext?'Only secure origins are allowed (see: https://goo.gl/Y0ZkNV).':denied?'User denied Geolocation':'Position unavailable'}));
      }},0);return id;
    };
    method('Geolocation','getCurrentPosition',(_s,success,failure,options)=>{locate(success,failure,options,false)});
    method('Geolocation','watchPosition',(_s,success,failure,options)=>locate(success,failure,options,true));
    method('Geolocation','clearWatch',(_s,id)=>{watches.delete(Number(id))});
    attribute('GeolocationPositionError','code',s=>s.code);attribute('GeolocationPositionError','message',s=>s.message);

    // Hardware enumeration reflects the absence of granted transports. Chooser
    // requests enforce activation and never manufacture successful devices.
    method('Bluetooth','getAvailability',()=>state('devices').bluetooth,true);
    for(const [type,name] of [['HID','getDevices'],['USB','getDevices'],['Serial','getPorts']])method(type,name,()=>state('deviceList',type),true);
    for(const [type,name] of [['Bluetooth','requestDevice'],['HID','requestDevice'],['USB','requestDevice'],['Serial','requestPort']])method(type,name,()=>{
      if(!state('activation').isActive)throw error('SecurityError',`Failed to execute '${name}' on '${type}': Must be handling a user gesture to show a permission request.`);
      if(type==='HID')return [];
      throw error('NotFoundError','No device selected.');
    },true);

    method('StorageManager','estimate',()=>state('quota'),true);
    method('StorageManager','persisted',()=>permission('persistent-storage')==='granted',true);
    method('StorageManager','persist',()=>permission('persistent-storage')==='granted',true);
    const bucketName=value=>{const name=String(value);if(!/^[a-z0-9][a-z0-9_-]{0,63}$/.test(name))throw new TypeError('The bucket name is not valid.');return name};
    const bucket=(s,operation='get',value)=>{const b=state('bucket',operation,s.name,value);if(b===null)throw error('InvalidStateError','The storage bucket has been deleted.');return b};
    method('StorageBucketManager','keys',()=>state('bucket','keys'),true);
    method('StorageBucketManager','open',(_s,name,options={})=>{
      name=bucketName(name);
      if(options.durability!==undefined&&!['strict','relaxed'].includes(options.durability))throw new TypeError('Invalid StorageBucketDurability');
      if(options.quota!==undefined&&!(Number(options.quota)>0))throw new TypeError('The quota must be greater than zero.');
      if(options.expires!==undefined&&!(Number(options.expires)>Date.now()))throw new TypeError('The expiration must be in the future.');
      state('bucket','open',name,JSON.stringify(options));return create('StorageBucket',{name});
    },true);
    method('StorageBucketManager','delete',(_s,name)=>{state('bucket','delete',bucketName(name))},true);
    attribute('StorageBucket','name',s=>s.name);
    for(const name of ['persisted','durability','expires'])method('StorageBucket',name,s=>bucket(s)[name],true);
    method('StorageBucket','persist',s=>bucket(s,'persist').persisted,true);
    method('StorageBucket','estimate',s=>{const b=bucket(s);return {usage:b.usage,quota:b.quota}},true);
    method('StorageBucket','setExpires',(s,value)=>{if(!(Number(value)>Date.now()))throw new TypeError('The expiration must be in the future.');bucket(s,'setExpires',Number(value))},true);
    method('LockManager','query',()=>state('lock','query'),true);
    method('LockManager','request',(_s,name,options,callback)=>{
      name=String(name);if(typeof options==='function'){callback=options;options={}}options=options||{};
      if(typeof callback!=='function')throw new TypeError('The callback is not a function.');
      if(name.startsWith('-'))throw error('NotSupportedError',"Names starting with '-' are reserved");
      const mode=options.mode===undefined?'exclusive':String(options.mode);
      if(!['exclusive','shared'].includes(mode))throw new TypeError('Invalid LockMode');
      if(options.steal)throw error('NotSupportedError','Stealing an active lock is not supported.');
      if(options.signal&&options.ifAvailable)throw error('NotSupportedError','The signal option cannot be used with ifAvailable.');
      if(options.signal?.aborted)throw options.signal.reason;
      const ticket=state('lock','enqueue',name,mode);
      return new Promise((resolve,reject)=>{
        let finished=false,timer;
        const release=()=>{if(finished)return;finished=true;if(timer!==undefined)clearTimeout(timer);state('lock','release',ticket);options.signal?.removeEventListener('abort',abort)};
        const abort=()=>{release();reject(options.signal.reason)};
        options.signal?.addEventListener('abort',abort,{once:true});
        const attempt=()=>{
          if(finished)return;
          if(!state('lock','acquire',ticket)){
            if(!options.ifAvailable){timer=setTimeout(attempt,1);return}
            release();Promise.resolve().then(()=>callback(null)).then(resolve,reject);return;
          }
          options.signal?.removeEventListener('abort',abort);
          Promise.resolve().then(()=>callback(create('Lock',{name,mode}))).then(v=>{release();resolve(v)},e=>{release();reject(e)});
        };attempt();
      });
    },true);
    attribute('Lock','name',s=>s.name);attribute('Lock','mode',s=>s.mode);

    method('MediaDevices','enumerateDevices',()=>state('media').kinds.map(kind=>create('MediaDeviceInfo',{deviceId:'',kind,label:'',groupId:''})),true);
    method('MediaDevices','getSupportedConstraints',()=>Object.fromEntries((state('media').constraints||[]).map(name=>[name,true])));
    for(const name of ['deviceId','kind','label','groupId'])attribute('MediaDeviceInfo',name,s=>s[name]);
    method('MediaDeviceInfo','toJSON',s=>({deviceId:s.deviceId,kind:s.kind,label:s.label,groupId:s.groupId}));
    method('MediaDevices','getUserMedia',(_s,constraints={})=>{
      if(!constraints.audio&&!constraints.video)throw new TypeError("Failed to execute 'getUserMedia' on 'MediaDevices': At least one of audio and video must be requested");
      if(state('presentation').mode==='headful'&&((constraints.video&&permission('camera')==='prompt')||(constraints.audio&&permission('microphone')==='prompt')))return permissionPrompt();
      if((constraints.video&&permission('camera')!=='granted')||(constraints.audio&&permission('microphone')!=='granted')){
        throw error('NotAllowedError','Permission denied');
      }
      throw error('NotFoundError','Requested device not found');
    },true);
    method('MediaDevices','getDisplayMedia',()=>{throw error('NotAllowedError','Permission denied')},true);
    for(const name of ['decodingInfo','encodingInfo'])method('MediaCapabilities',name,(_s,configuration)=>{
      if(!configuration||(!configuration.audio&&!configuration.video))throw new TypeError('The provided configuration is not valid.');
      const supported=name==='decodingInfo'&&[configuration.video,configuration.audio].filter(Boolean).every(track=>(state('media').decoding||[]).includes(track.contentType));
      const result={supported,smooth:supported,powerEfficient:supported};if(name==='decodingInfo')result.keySystemAccess=null;return result;
    },true);
    attribute('MediaSession','metadata',s=>s.metadata||null,(s,v)=>{if(v!==null&&!(v instanceof MediaMetadata))throw new TypeError('Invalid MediaMetadata');s.metadata=v});
    attribute('MediaSession','playbackState',s=>s.playbackState||'none',(s,v)=>{if(!['none','paused','playing'].includes(String(v)))throw new TypeError('Invalid MediaSessionPlaybackState');s.playbackState=String(v)});
    method('MediaSession','setActionHandler',(s,name,handler)=>{(s.actions||(s.actions=new Map())).set(String(name),handler)});
    method('MediaSession','setPositionState',(s,value)=>{if(value&&(!(value.duration>=0)||(value.position||0)>value.duration||(value.playbackRate??1)===0))throw new TypeError('Invalid position state');s.position=value});
    attribute('Presentation','defaultRequest',s=>s.defaultRequest||null,(s,v)=>{if(v!==null&&!(v instanceof PresentationRequest))throw new TypeError('Invalid PresentationRequest');s.defaultRequest=v});
    attribute('Presentation','receiver',()=>null);

    attribute('ServiceWorkerContainer','controller',()=>null);
    attribute('ServiceWorkerContainer','ready',s=>s.ready||(s.ready=new Promise(()=>{})));
    method('ServiceWorkerContainer','getRegistrations',()=>[],true);
    method('ServiceWorkerContainer','getRegistration',()=>undefined,true);
    method('ServiceWorkerContainer','register',()=>{throw error('NotSupportedError','Service Worker execution backend is unavailable.')},true);
    method('ServiceWorkerContainer','startMessages',()=>{});
    method('XRSystem','isSessionSupported',(_s,mode)=>{if(!['inline','immersive-vr','immersive-ar'].includes(mode))throw new TypeError('Invalid XRSessionMode');return mode==='inline'},true);
    method('XRSystem','requestSession',(_s,mode)=>{
      if(mode!=='inline'&&!state('activation').isActive)throw error('SecurityError',"Failed to execute 'requestSession' on 'XRSystem': The requested session requires user activation.");
      throw error('NotSupportedError','The specified session configuration is not supported.');
    },true);
    method('NavigatorManagedData','getManagedConfiguration',()=>{throw error('NotAllowedError','Managed configuration is empty. This API is available only for managed apps.')},true);
    method('NavigatorLogin','setStatus',(_s,status)=>{if(!['logged-in','logged-out'].includes(status))throw new TypeError('Invalid login status');state('login',status)},true);
    method('WakeLock','request',()=>{throw error('NotAllowedError','Wake Lock permission request denied')},true);
    method('ProtectedAudience','queryFeatureSupport',()=>undefined);
    const rect=()=>new DOMRect(0,0,0,0);
    attribute('WindowControlsOverlay','visible',()=>false);method('WindowControlsOverlay','getTitlebarAreaRect',rect);
    attribute('VirtualKeyboard','boundingRect',s=>s.rect||(s.rect=rect()));
    attribute('VirtualKeyboard','overlaysContent',s=>!!s.overlaysContent,(s,v)=>s.overlaysContent=!!v);
    method('VirtualKeyboard','show',()=>{});method('VirtualKeyboard','hide',()=>{});
    method('Keyboard','unlock',()=>{});
    method('Keyboard','lock',()=>{
      if(permission('keyboard-lock')==='prompt'&&state('presentation').mode==='headful')return permissionPrompt();
      throw error('InvalidStateError','lock() request could not be registered.');
    },true);
    method('Keyboard','getLayoutMap',()=>create('KeyboardLayoutMap',{map:new Map(Object.entries(state('keyboard')||{}))}),true);
    attribute('KeyboardLayoutMap','size',s=>s.map.size);
    for(const name of ['get','has','keys','values','entries'])method('KeyboardLayoutMap',name,(s,...args)=>s.map[name](...args));
    method('KeyboardLayoutMap','forEach',function(s,callback,thisArg){if(typeof callback!=='function')throw new TypeError('The callback is not a function.');s.map.forEach((v,k)=>callback.call(thisArg,v,k,this))});
    if(globalThis.KeyboardLayoutMap){const iterator=function(){return check(this,'KeyboardLayoutMap').map.entries()};markNative(iterator,'entries');Object.defineProperty(KeyboardLayoutMap.prototype,Symbol.iterator,{value:iterator,writable:true,configurable:true})}
    method('Ink','requestPresenter',(_s,options={})=>create('DelegatedInkTrailPresenter',{presentationArea:options.presentationArea||null}),true);
    attribute('DelegatedInkTrailPresenter','presentationArea',s=>s.presentationArea);
    attribute('DelegatedInkTrailPresenter','expectedImprovement',()=>0);
    method('DelegatedInkTrailPresenter','updateInkTrailStartPoint',()=>{throw error('NotSupportedError','Ink rendering backend is unavailable.')});
  };
