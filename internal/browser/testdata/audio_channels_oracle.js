(async()=>{
 async function render(channels,input,discrete=false,kRate=false){const c=new OfflineAudioContext(channels,8,8000),s=c.createBufferSource(),g=c.createGain(),b=c.createBuffer(input.length,8,8000);input.forEach((v,i)=>b.copyToChannel(new Float32Array(v),i));s.buffer=b;s.connect(g);g.connect(c.destination);if(discrete)c.destination.channelInterpretation='discrete';if(kRate){g.gain.automationRate='k-rate';g.gain.setValueAtTime(.5,0).setValueAtTime(.25,4/8000)}s.start();const r=await c.startRendering();return Array.from({length:channels},(_,i)=>Array.from(r.getChannelData(i)))}
 return {up:await render(2,[[1,2,3]]),discrete:await render(2,[[1,2,3]],true),down:await render(1,[[1,2,3,3e38],[3,4,5,3e38]]),kRate:await render(1,[[1,1,1,1,1,1,1,1]],false,true)};
})()
