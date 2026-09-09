(async()=>{
 const out={},err=f=>{try{f();return null}catch(e){return e.name}};
 async function render({channels=1,length=384,frequency=1000,gain=1,disconnect=false,stop=Infinity,start=0,compress=false,customRate=0}={}){
  const c=new OfflineAudioContext(channels,length,48000),o=new OscillatorNode(c,{frequency}),g=c.createGain();g.gain.value=gain;
  if(customRate)o.setPeriodicWave(new PeriodicWave(new OfflineAudioContext(1,8,customRate)));
  o.connect(g);if(disconnect)o.disconnect(g);let n=g;if(compress){n=c.createDynamicsCompressor();g.connect(n)}n.connect(c.destination);
  let ended=0;o.onended=()=>ended++;o.start(start);if(Number.isFinite(stop))o.stop(stop);const b=await c.startRendering();return {data:Array.from(b.getChannelData(0)),second:channels===2?Array.from(b.getChannelData(1)):null,ended,reduction:compress?n.reduction:null};
 }
 const a=await render(),b=await render(),half=await render({gain:.5}),stereo=await render({channels:2}),disconnected=await render({disconnect:true});
 out.relations=[a.data.every((v,i)=>v===b.data[i]),a.data.every((v,i)=>Math.fround(v*.5)===half.data[i]),a.data.every((v,i)=>v===stereo.second[i]),disconnected.data.every(v=>v===0)];
 out.lifecycle=[];for(const [start,stop]of [[0,0],[1,0],[1,.002],[0,.008],[0,.00801]]){const r=await render({start,stop});out.lifecycle.push([start,stop,r.ended,r.data.some(v=>v!==0)])}
 out.pcmPartial=await render({compress:true,length:333});out.pcmForeign=await render({customRate:8000});
 return out;
})()
