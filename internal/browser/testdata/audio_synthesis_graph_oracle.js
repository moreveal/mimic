(async()=>{
 const out={},error=f=>{try{f();return null}catch(e){return e.name}};
 const shape=(rate,length)=>new OfflineAudioContext(1,length,rate);
 for(const config of [
 {rate:44100,length:5000,type:'triangle',frequency:10000,compress:true},
 {rate:8000,length:300,type:'square',frequency:73,start:.25/8000,stop:280.1/8000},
 {rate:48000,length:256,type:'sine',frequency:.5},
 {rate:48000,length:512,type:'sawtooth',frequency:200,automate:true},
 {rate:96000,length:128,type:'triangle',frequency:10000},
 {rate:48000,length:256,frequency:1000,custom:true,normalize:true},
 {rate:48000,length:256,frequency:1000,custom:true,normalize:false},
 ]) {
  const c=shape(config.rate,config.length),o=c.createOscillator();if(config.type)o.type=config.type;o.frequency.value=config.frequency;
  if(config.custom){const real=new Float32Array([.8,.25,.125,0]),imag=new Float32Array([.1,.75,0,.25]);const w=c.createPeriodicWave(real,imag,{disableNormalization:!config.normalize});o.setPeriodicWave(w);real.fill(100);imag.fill(100);}
  if(config.automate){o.frequency.setValueAtTime(200,0);o.frequency.linearRampToValueAtTime(400,128/config.rate);o.detune.setValueAtTime(1200,256/config.rate)}
  let n=o;if(config.compress){n=c.createDynamicsCompressor();n.threshold.value=-50;n.knee.value=40;n.ratio.value=12;n.attack.value=0;n.release.value=.25;o.connect(n)}
  n.connect(c.destination);let ended=0;o.onended=()=>ended++;o.start(config.start||0);if(config.stop)o.stop(config.stop);const b=await c.startRendering();out['pcm'+JSON.stringify(config)]={data:Array.from(b.getChannelData(0)),ended,type:o.type};
 }
 const c=shape(48000,128),w=new PeriodicWave(c);out.constructors=[error(()=>new OscillatorNode(c,{type:'custom',periodicWave:w})),error(()=>new PeriodicWave(c,{real:[0,1]})),error(()=>c.createPeriodicWave([0,1],[0,1])),error(()=>new PeriodicWave(c,{real:[0,Infinity]}))];
 return out;
})()
