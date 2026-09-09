(async()=>{
 const out={lengths:[AudioBuffer.length,OfflineAudioContext.length,AudioBufferSourceNode.length,GainNode.length,OfflineAudioCompletionEvent.length]},c=new OfflineAudioContext(1,16,8000),g=c.createGain(),events=[];
 g.gain.setValueAtTime(.5,0).setValueAtTime(.25,8/8000);g.connect(c.destination);
 out.paramBefore=[g.gain.value,g.gain.defaultValue,g.gain.minValue,g.gain.maxValue,g.gain.automationRate];
 let completion;c.onstatechange=()=>events.push('state:'+c.state);c.oncomplete=e=>{completion=e;events.push('complete')};const p=c.startRendering();events.push(c.state);const buffer=await p;events.push('promise');await new Promise(r=>setTimeout(r,20));out.events=events;
 out.completion=[Object.prototype.toString.call(completion),completion.renderedBuffer===buffer,completion.isTrusted,Object.hasOwn(completion,'renderedBuffer')];out.paramAfter=g.gain.value;
 return out;
})()
