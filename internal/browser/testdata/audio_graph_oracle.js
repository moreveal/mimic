(async()=>{
 const out={},c=new OfflineAudioContext(1,256,8000),b=c.createBuffer(1,8,8000);
 const data=b.getChannelData(0);data.set([1,.5,-.5,-1,.25,0,2,-2]);
 const s=c.createBufferSource(),g=c.createGain();s.buffer=b;g.gain.value=.5;
 out.nodes=[s.context===c,g.context===c,s.numberOfInputs,s.numberOfOutputs,g.numberOfInputs,g.numberOfOutputs,s.channelCount,g.channelCount,s.buffer===b,s.playbackRate.value,s.detune.value,s.loop,g.gain===g.gain];
 out.connections=[s.connect(g)===g,g.connect(c.destination)===c.destination];
 let ended=0;s.onended=()=>ended++;
 s.start(2/8000);out.afterStart=[data.length,b.getChannelData(0)===data];
 const rendered=await c.startRendering();out.samples=Array.from(rendered.getChannelData(0).slice(0,14));
 await new Promise(r=>setTimeout(r,20));out.ended=ended;
 const fail=fn=>{try{fn();return null}catch(e){return e.name}};
 out.errors=[fail(()=>s.start()),fail(()=>c.createBufferSource().start(-1)),fail(()=>g.connect(new OfflineAudioContext(1,8,8000).destination)),fail(()=>g.gain.setValueAtTime(1,-1))];
 return out;
})()
