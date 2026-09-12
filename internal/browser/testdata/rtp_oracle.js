(async()=>{const out={},run=async f=>{try{return {ok:true,value:await f()}}catch(e){return {name:e.name,message:e.message}}};
out.rtp={};for(const who of ['RTCRtpSender','RTCRtpReceiver']){out.rtp[who]={};for(const kind of ['audio','video','AUDIO','',null,undefined])out.rtp[who][String(kind)]=await run(()=>globalThis[who].getCapabilities(kind));out.rtp[who].missing=await run(()=>globalThis[who].getCapabilities());}
out.rtpMutation=await run(()=>{const a=RTCRtpSender.getCapabilities('audio');a.codecs[0].mimeType='author';a.headerExtensions.length=0;return RTCRtpSender.getCapabilities('audio')});
return out})()
