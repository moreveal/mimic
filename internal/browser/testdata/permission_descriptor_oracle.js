(async()=>{
  const out={names:{},conversion:{}};
  for(const name of ['geolocation','notifications','camera','microphone','midi','persistent-storage','push','clipboard-read','clipboard-write','accelerometer','gyroscope','magnetometer','background-sync','display-capture','storage-access','local-network-access','window-management','fullscreen']){
    try{const p=await navigator.permissions.query({name,userVisibleOnly:true,allowWithoutGesture:true});out.names[name]={name:p.name,state:p.state,tag:Object.prototype.toString.call(p),own:Object.keys(p)};}
    catch(e){out.names[name]={error:e.name,message:e.message};}
  }
  const cases={missing:()=>({}),undefined:()=>({name:undefined}),null:()=>null,number:()=>3,symbol:()=>({name:Symbol('x')}),getter:log=>({get name(){log.push('get');return 'camera'}}),proxy:log=>new Proxy({name:'camera'},{has(t,k){log.push('has:'+String(k));return Reflect.has(t,k)},get(t,k){log.push('get:'+String(k));return Reflect.get(t,k)}}),throwing:log=>({get name(){log.push('get');throw new Error('author')}})};
  for(const [key,create] of Object.entries(cases)){
    const log=[];let p;try{p=navigator.permissions.query(create(log));log.push('returned');}catch(e){log.push('threw:'+e.name+':'+e.message)}
    const immediate=log.slice();if(p)try{const status=await p;log.push('resolved:'+status.name)}catch(e){log.push('rejected:'+e.name+':'+e.message)}out.conversion[key]={immediate,log};
  }
  out.descriptors={};
  for(const name of ['geolocation','notifications','microphone','camera','midi','push','clipboard-read','clipboard-write','fullscreen','top-level-storage-access','accelerometer','background-fetch','periodic-background-sync','screen-wake-lock','payment-handler','pointer-lock','window-management','local-network','loopback-network']){
    for(const enabled of [false,true]){
      const log=[];const d=new Proxy({name,sysex:enabled,panTiltZoom:enabled,userVisibleOnly:enabled,allowWithoutGesture:enabled,allowWithoutSanitization:enabled,requestedOrigin:location.origin},{get(t,k){log.push(String(k));return Reflect.get(t,k)}});
      try{const s=await navigator.permissions.query(d);out.descriptors[name+':'+enabled]={log,name:s.name,state:s.state};}catch(e){out.descriptors[name+':'+enabled]={log,error:e.name,message:e.message}}
    }
  }
  out.clipboard={};for(const gesture of [false,true])for(const sanitation of [false,true]){const s=await navigator.permissions.query({name:'clipboard-write',allowWithoutGesture:gesture,allowWithoutSanitization:sanitation});out.clipboard[gesture+':'+sanitation]={name:s.name,state:s.state}}
  return out;
})()
