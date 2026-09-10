(async()=>{
  const out={}, capture=fn=>{try{return {value:fn()}}catch(e){return {exception:{name:e.name,message:e.message}}}};
  const canvas=document.createElement('canvas');canvas.width=120;canvas.height=40;
  const c=canvas.getContext('2d',{willReadFrequently:true});c.font='16px Arial';
  out.fonts={check:document.fonts.check('16px Arial'),status:document.fonts.status,
    metrics: capture(()=>{const m=c.measureText('AAAA');return {width:m.width,left:m.actualBoundingBoxLeft,right:m.actualBoundingBoxRight,ascent:m.actualBoundingBoxAscent,descent:m.actualBoundingBoxDescent}})};
  const render=text=>{c.clearRect(0,0,120,40);c.fillText(text,4,22);return Array.from(c.getImageData(0,0,120,40).data)};
  const a=render('AAAA'),b=render('AAAB'),again=render('AAAA');
  out.canvas={repeat:a.every((x,i)=>x===again[i]),changed:a.some((x,i)=>x!==b[i]),
    invalid:capture(()=>c.getImageData(0,0,0,1))};
  const audio=document.createElement('audio'),video=document.createElement('video');
  out.media=['audio/ogg; codecs="vorbis"','audio/mpeg','video/webm; codecs="vp9"','video/mp4; codecs="avc1.42E01E"'].map(type=>[type,capture(()=>audio.canPlayType(type)),capture(()=>video.canPlayType(type))]);
  out.rtc=capture(()=>{const p=new RTCPeerConnection({iceServers:[]});try{return {configuration:p.getConfiguration(),signaling:p.signalingState,ice:p.iceConnectionState}}finally{p.close()}});
  return out;
})()
