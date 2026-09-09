(async()=>{
 const out={},a=navigator.getBattery(),b=navigator.getBattery();
 out.promise={same:a===b,tag:Object.prototype.toString.call(a)};
 const battery=await a;out.battery={same:battery===await b,tag:Object.prototype.toString.call(battery),eventTarget:battery instanceof EventTarget,own:Object.keys(battery),charging:battery.charging,level:battery.level,chargingTime:String(battery.chargingTime),dischargingTime:String(battery.dischargingTime),handlers:['onchargingchange','onchargingtimechange','ondischargingtimechange','onlevelchange'].map(k=>battery[k])};
 try{const p=navigator.getBattery.call({});out.invalidBattery={then:typeof p.then};try{await p}catch(e){out.invalidBattery.error=e.name}}catch(e){out.invalidBattery={syncError:e.name}}
 const pads=navigator.getGamepads(),pads2=navigator.getGamepads();out.gamepads={array:Array.isArray(pads),same:pads===pads2,length:pads.length,empty:pads.every(v=>v===null)};
 try{navigator.getGamepads.call({})}catch(e){out.invalidGamepads=e.name}
 out.events={};for(const type of ['chargingchange','chargingtimechange','dischargingtimechange','levelchange']){const calls=[],first=()=>calls.push('first'),last=()=>calls.push('last'),name='on'+type;const handler=function(e){calls.push(this===battery&&e.target===battery?'handler':'bad receiver')};battery.addEventListener(type,first);battery[name]=handler;battery.addEventListener(type,last);battery.dispatchEvent(new Event(type));battery[name]=()=>calls.push('replacement');battery.dispatchEvent(new Event(type));battery[name]=null;battery[name]=handler;battery.dispatchEvent(new Event(type));battery[name]=null;battery.removeEventListener(type,first);battery.removeEventListener(type,last);out.events[type]=calls}
 return out;
})()
