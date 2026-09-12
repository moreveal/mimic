(async () => {
  const results = {};
  const audio = {contentType:'audio/mp4; codecs="mp4a.40.2"', channels:'2', bitrate:128000, samplerate:48000};
  const video = {contentType:'video/mp4; codecs="avc1.42E01E"', width:1920, height:1080, bitrate:2646242, framerate:'25'};
  const configurations = [];
  for (const type of ['file','media-source','webrtc']) {
    configurations.push(['audio-'+type,{type,audio}]);
    configurations.push(['video-'+type,{type,video}]);
  }
  configurations.push(['unsupported-video',{type:'file',video:{...video,contentType:'video/mp4; codecs="unrecognized"'}}]);
  for(const [key,value] of [['width',0],['height',0],['width',-1],['bitrate',-1],['framerate',0],['framerate',-1]])configurations.push([key+'-'+value,{type:'file',video:{...video,[key]:value}}]);
  configurations.push(['cached-video',{type:'file',video}]);
  for (const [name, configuration] of configurations) {
    let settled = false, value = null, error = null;
    const promise = navigator.mediaCapabilities.decodingInfo(configuration).then(
      result => { settled = true; value = result; },
      failure => { settled = true; error = failure.name; });
    await Promise.resolve(); await Promise.resolve();
    const microtask = settled;
    await promise;
    results[name] = {microtask,value,error};
  }
  const originalTimeout = globalThis.setTimeout;
  try {
    globalThis.setTimeout = () => {throw new Error('author timer must not implement platform service');};
    results.authorTimer = (await navigator.mediaCapabilities.decodingInfo({type:'file',video})).supported;
  } finally {globalThis.setTimeout = originalTimeout;}
  results.concurrent = (await Promise.all(Array.from({length:4},()=>navigator.mediaCapabilities.decodingInfo({type:'file',video})))).map(x=>x.supported);
  return results;
})()
