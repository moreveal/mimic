(async()=>{
 async function run(change){const c=new OfflineAudioContext(1,16,8000),b=c.createBuffer(1,4,8000),s=c.createBufferSource(),g=c.createGain();b.getChannelData(0).set([1,2,3,4]);if(change)b.getChannelData(0)[2]=9;s.buffer=b;s.loop=true;s.connect(g);g.connect(c.destination);g.gain.setValueAtTime(.5,0).setValueAtTime(.25,8/8000);s.start();s.stop(12/8000);return Array.from((await c.startRendering()).getChannelData(0))}
 const a=await run(false),b=await run(true),c=await run(false),out={samples:a,repeat:JSON.stringify(a)===JSON.stringify(c),changed:a.map((v,i)=>v===b[i]?null:i).filter(v=>v!==null)};
 const context=new OfflineAudioContext(1,16,8000),buffer=context.createBuffer(1,32,8000),data=buffer.getChannelData(0),source=context.createBufferSource();source.buffer=buffer;source.connect(context.destination);let ended=0;source.onended=()=>ended++;source.start();await context.startRendering();await new Promise(r=>setTimeout(r,10));out.ownership=[data.length,data===buffer.getChannelData(0),ended];
 const loopContext=new OfflineAudioContext(1,256,8000),loop=loopContext.createBufferSource();loop.buffer=loopContext.createBuffer(1,4,8000);loop.loop=true;loop.connect(loopContext.destination);let loopEnded=0;loop.onended=()=>loopEnded++;loop.start();await loopContext.startRendering();await new Promise(r=>setTimeout(r,10));out.loopEnded=loopEnded;
 return out;
})()
