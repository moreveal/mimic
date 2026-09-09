(()=>{
 const out={},error=fn=>{try{fn();return null}catch(e){return e.name}};
 out.invalid=[()=>new AudioBuffer(),()=>new AudioBuffer({length:0,sampleRate:44100}),()=>new AudioBuffer({length:4,sampleRate:0}),()=>new AudioBuffer({length:4,sampleRate:44100,numberOfChannels:0}),()=>new AudioBuffer({length:4,sampleRate:44100,numberOfChannels:33})].map(error);
 const b=new AudioBuffer({length:5,sampleRate:44100,numberOfChannels:2}),a=b.getChannelData(0),c=b.getChannelData(1);
 out.buffer=[Object.prototype.toString.call(b),b.length,b.sampleRate,b.numberOfChannels,b.duration,a===b.getChannelData(0),a===c,Array.from(a)];
 a.set([1,.5,-.5,2,-2]);out.shared=Array.from(b.getChannelData(0));
 b.copyToChannel(new Float32Array([3,4,5]),1,3);out.copyTo=Array.from(c);
 const dst=new Float32Array([9,9,9,9]);b.copyFromChannel(dst,0,3);out.copyFrom=Array.from(dst);
 out.errors=[()=>b.getChannelData(2),()=>b.getChannelData(-1),()=>b.copyToChannel(new Float32Array(0),2),()=>b.copyFromChannel([],0),()=>b.copyFromChannel(new Float32Array(2),0,9)].map(error);
 return out;
})()
