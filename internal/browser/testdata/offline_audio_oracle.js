(async()=>{
 const out={},error=fn=>{try{fn();return null}catch(e){return e.name}};
 out.invalid=[()=>new OfflineAudioContext(),()=>new OfflineAudioContext(0,10,44100),()=>new OfflineAudioContext(1,0,44100),()=>new OfflineAudioContext({length:8,sampleRate:44100})].map(error);
 const c=new OfflineAudioContext(1,1000,44100),d=c.destination;
 out.initial=[Object.prototype.toString.call(c),c.length,c.sampleRate,c.currentTime,c.state,c.destination===d,Object.prototype.toString.call(d),d.context===c,d.numberOfInputs,d.numberOfOutputs,d.channelCount,d.channelCountMode,d.channelInterpretation,d.maxChannelCount];
 const events=[];c.onstatechange=()=>events.push('state:'+c.state);c.oncomplete=e=>events.push('complete:'+e.renderedBuffer.length);
 const result=await c.startRendering().then(b=>{events.push('promise');return b});
 out.render=[c.state,c.currentTime,result.length,result.numberOfChannels,result.sampleRate,Array.from(result.getChannelData(0)).every(v=>v===0),events];
 out.second=await c.startRendering().then(()=>null,e=>e.name);
 return out;
})()
