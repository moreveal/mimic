(async()=>{
 const out={};
 const describe=sdp=>({lines:sdp.split('\r\n').filter(l=>l&&!/^o=|^a=ice-ufrag:|^a=ice-pwd:|^a=fingerprint:/.test(l)),credentials:sdp.split('\r\n').filter(l=>/^a=ice-ufrag:/.test(l)).map((l,i,a)=>a.indexOf(l))});
 for(const [name,options,data] of [['empty',{},false],['data',{},true],['audio',{offerToReceiveAudio:true},false],['video',{offerToReceiveVideo:true},false],['both',{offerToReceiveAudio:true,offerToReceiveVideo:true},true]]){
  const p=new RTCPeerConnection();if(data)p.createDataChannel('local');const a=await p.createOffer(options),b=await p.createOffer(options);
  out[name]={first:describe(a.sdp),repeat:describe(b.sdp),plain:Object.getPrototypeOf(a)===Object.prototype,keys:Object.keys(a),transceivers:p.getTransceivers().map(t=>({direction:t.direction,mid:t.mid,currentDirection:t.currentDirection,kind:t.receiver.track.kind}))};p.close();
 }
 out.lifecycle={};for(const data of [false,true,'both']){
  const p=new RTCPeerConnection();if(data)p.createDataChannel('local');const log=[],candidates=[];p.onicecandidate=e=>{if(e.candidate)candidates.push({mid:e.candidate.sdpMid,index:e.candidate.sdpMLineIndex,type:e.candidate.type})};
  p.onsignalingstatechange=()=>log.push('signaling:'+p.signalingState);p.onicegatheringstatechange=()=>log.push('ice:'+p.iceGatheringState);
  const a=await p.createOffer(data==='both'?{offerToReceiveAudio:true,offerToReceiveVideo:true}:{});const pending=p.setLocalDescription(a);log.push('returned:'+p.signalingState);await pending;log.push('resolved:'+p.signalingState);
  await new Promise(r=>setTimeout(r,250));out.lifecycle[data]={log,candidates,local:p.localDescription?.type,current:p.currentLocalDescription?.type??null,pending:p.pendingLocalDescription?.type??null,mid:p.getTransceivers().map(t=>t.mid)};p.close();
 }
 return out;
})()
