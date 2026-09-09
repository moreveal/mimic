(()=>{
 const fail=fn=>{try{fn();return null}catch(e){return e.name}},out={};
 out.rates=[2999,3000,768000,768001,1e40].map(sampleRate=>[sampleRate,fail(()=>new AudioBuffer({length:1,sampleRate}))]);
 const c=new OfflineAudioContext(1,8,8000),g=new GainNode(c,{channelCount:1,channelCountMode:'explicit',channelInterpretation:'discrete',gain:.25});
 out.options=[g.channelCount,g.channelCountMode,g.channelInterpretation,g.gain.value];
 out.paramErrors=[NaN,Infinity,1e40].map(value=>fail(()=>{g.gain.value=value}));
 class DerivedGain extends GainNode{}class DerivedSource extends AudioBufferSourceNode{}
 out.subclass=[new DerivedGain(c) instanceof DerivedGain,new DerivedSource(c) instanceof DerivedSource];
 const b=c.createBuffer(1,2,8000),s=c.createBufferSource();s.buffer=b;s.buffer=null;out.reassign=fail(()=>{s.buffer=b});
 out.destination=[fail(()=>{c.destination.channelCount=2}),fail(()=>{c.destination.channelCountMode='max'}),c.destination.channelCountMode,c.destination.channelCount,c.destination.maxChannelCount];
 out.sourceRate=[fail(()=>{s.playbackRate.automationRate='a-rate'}),s.playbackRate.automationRate];
 return out;
})()
