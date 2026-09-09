(async()=>{
const out={},c=new OfflineAudioContext(1,1024,48000),o=c.createOscillator();
const error=f=>{try{f();return null}catch(e){return e.name}};
out.defaults=[o.type,o.numberOfInputs,o.numberOfOutputs,o.channelCount,o.channelCountMode,o.channelInterpretation,...['frequency','detune'].map(k=>{const p=o[k];return [p.value,p.defaultValue,p.minValue,p.maxValue,p.automationRate]})];
out.types=['sine','square','sawtooth','triangle','custom','bad'].map(t=>[t,error(()=>o.type=t),o.type]);
out.bounds=[error(()=>o.frequency.value=1e8),o.frequency.value,error(()=>o.detune.value=1e8),o.detune.value];
out.scheduling=[error(()=>o.stop()),error(()=>o.start(-1)),error(()=>o.start()),error(()=>o.start()),error(()=>o.stop(-1)),error(()=>o.stop())];
out.wave=[error(()=>c.createPeriodicWave(new Float32Array(1),new Float32Array(1))),error(()=>c.createPeriodicWave(new Float32Array(2),new Float32Array(3)))];
for(const type of ['sine','square','sawtooth','triangle'])for(const frequency of [0,1000,-1000,24000,30000]){const c=new OfflineAudioContext(1,32,48000),o=new OscillatorNode(c,{type,frequency});o.connect(c.destination);o.start();out[type+frequency]=Array.from((await c.startRendering()).getChannelData(0));}
return out;
})()
