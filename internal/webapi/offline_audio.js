// CPU-side PCM observations. No audio device, native renderer or global graph.
(()=>{
 if(typeof globalThis.OfflineAudioContext!=='function'||typeof globalThis.AudioBuffer!=='function')return;
 const buffers=new WeakMap(),contexts=new WeakMap(),nodes=new WeakMap(),params=new WeakMap(),completions=new WeakMap();
 const exception=(name,message)=>new DOMException(message||name,name);
 const unsupported=name=>{host.semanticMissing('WebAudio.'+name);throw exception('NotSupportedError','Unsupported audio operation: '+name)};
 const requireSlot=(map,value)=>{const s=map.get(value);if(!s)throw new TypeError('Illegal invocation');return s};
 const finite=value=>{value=Number(value);if(!Number.isFinite(value))throw new TypeError('Expected finite number');return value};
 const float=value=>{value=Math.fround(finite(value));if(!Number.isFinite(value))throw new TypeError('Expected finite float');return value};
 const time=value=>{value=finite(value);if(value<0)throw new RangeError('Negative time');return value};
 const method=(type,name,value)=>{if(!globalThis[type])return;const prior=Object.getOwnPropertyDescriptor(globalThis[type].prototype,name);Object.defineProperty(value,'name',{value:name,configurable:true});if(prior?.value)Object.defineProperty(value,'length',{value:prior.value.length,configurable:true});markNative(value,name);Object.defineProperty(globalThis[type].prototype,name,{value,writable:true,configurable:true,enumerable:true})};
 const getter=(type,name,map,key=name,set)=>{if(!globalThis[type])return;const get=function(){const s=requireSlot(map,this);return typeof key==='function'?key(s):s[key]};markNative(get,name,'get ');if(set)markNative(set,name,'set ');Object.defineProperty(globalThis[type].prototype,name,{get,set,configurable:true,enumerable:true})};
 const install=(name,ctor)=>{ctor.prototype=globalThis[name].prototype;Object.defineProperty(ctor.prototype,'constructor',{value:ctor,writable:true,configurable:true});markNative(ctor,name);Object.defineProperty(globalThis,name,{value:ctor,writable:true,configurable:true})};
 const dimensions=options=>{if(!options||options.length===undefined||options.sampleRate===undefined)throw new TypeError('Missing audio dimensions');const length=Number(options.length)>>>0,numberOfChannels=options.numberOfChannels===undefined?1:Number(options.numberOfChannels)>>>0,sampleRate=float(options.sampleRate);if(!length||!numberOfChannels||numberOfChannels>32||sampleRate<3000||sampleRate>768000)throw exception('NotSupportedError','Invalid audio dimensions');if(length*numberOfChannels>16*1024*1024)unsupported('PCM storage limit');return {length,numberOfChannels,sampleRate}};
 function AudioBuffer(options){if(!new.target)throw new TypeError('Expected new');const s=dimensions(options),object=Object.create(new.target.prototype);s.channels=Array.from({length:s.numberOfChannels},()=>new Float32Array(s.length));buffers.set(object,s);return object}
 install('AudioBuffer',AudioBuffer);
 for(const key of ['length','sampleRate','numberOfChannels'])getter('AudioBuffer',key,buffers);
 getter('AudioBuffer','duration',buffers,s=>s.length/s.sampleRate);
 const channel=(s,index)=>{index=Number(index)>>>0;if(index>=s.numberOfChannels)throw exception('IndexSizeError');return s.channels[index]};
 method('AudioBuffer','getChannelData',function getChannelData(index){const s=requireSlot(buffers,this);if(!arguments.length)throw new TypeError('Expected channel');return channel(s,index)});
 for(const name of ['copyFromChannel','copyToChannel'])method('AudioBuffer',name,function(data,index,offset=0){const s=requireSlot(buffers,this);if(arguments.length<2||!(data instanceof Float32Array))throw new TypeError('Expected Float32Array and channel');if(!data.length)return;const target=channel(s,index);offset=Number(offset)>>>0;if(offset>=s.length)return;const count=Math.min(data.length,s.length-offset);if(name==='copyFromChannel')data.set(target.subarray(offset,offset+count));else target.set(data.subarray(0,count),offset)});
 function OfflineAudioCompletionEvent(type,init){if(!new.target||arguments.length<2||!init||init.renderedBuffer===undefined)throw new TypeError('Expected completion buffer');requireSlot(buffers,init.renderedBuffer);const event=new Event(type,init);Object.setPrototypeOf(event,new.target.prototype);completions.set(event,{renderedBuffer:init.renderedBuffer});return event}
 install('OfflineAudioCompletionEvent',OfflineAudioCompletionEvent);getter('OfflineAudioCompletionEvent','renderedBuffer',completions);
 const emit=(object,type,buffer)=>{const event=buffer?new OfflineAudioCompletionEvent(type,{renderedBuffer:buffer}):new Event(type);dispatchEventCore(object,event,true)};
 const makeNode=(context,type,inputs,outputs,extra={})=>{requireSlot(contexts,context);const object=new EventTarget();Object.setPrototypeOf(object,globalThis[type].prototype);nodes.set(object,{context,type,numberOfInputs:inputs,numberOfOutputs:outputs,channelCount:2,channelCountMode:'max',channelInterpretation:'speakers',incoming:[],...extra});requireSlot(contexts,context).nodes.push(object);return object};
 const makeParam=(context,value,rate='a-rate')=>{const object=Object.create(AudioParam.prototype);params.set(object,{context,value,defaultValue:value,minValue:-3.4028234663852886e38,maxValue:3.4028234663852886e38,automationRate:rate,fixedRate:rate==='k-rate',timeline:[]});return object};
 for(const key of ['defaultValue','minValue','maxValue'])getter('AudioParam',key,params);
 getter('AudioParam','value',params,'value',function(value){requireSlot(params,this).value=float(value)});
 getter('AudioParam','automationRate',params,'automationRate',function(value){const s=requireSlot(params,this);value=String(value);if(!['a-rate','k-rate'].includes(value))throw new TypeError('Invalid automation rate');if(s.fixedRate&&value!==s.automationRate)throw exception('InvalidStateError');s.automationRate=value});
 method('AudioParam','setValueAtTime',function setValueAtTime(value,when){const s=requireSlot(params,this);if(arguments.length<2)throw new TypeError('Expected value and time');value=float(value);when=time(when);s.timeline=s.timeline.filter(e=>e.time!==when);s.timeline.push({time:when,value});s.timeline.sort((a,b)=>a.time-b.time);return this});
 method('AudioParam','cancelScheduledValues',function cancelScheduledValues(when){const s=requireSlot(params,this);when=time(when);s.timeline=s.timeline.filter(e=>e.time<when);return this});
 for(const name of ['linearRampToValueAtTime','exponentialRampToValueAtTime','setTargetAtTime','setValueCurveAtTime','cancelAndHoldAtTime'])method('AudioParam',name,function(){requireSlot(params,this);unsupported(name)});
 const parameterAt=(object,frame,rate)=>{const s=requireSlot(params,object);if(s.automationRate==='k-rate')frame=Math.floor(frame/128)*128;let value=s.value;for(const e of s.timeline){if(e.time>frame/rate)break;value=e.value}return value};
 for(const key of ['context','numberOfInputs','numberOfOutputs'])getter('AudioNode',key,nodes);
 getter('AudioNode','channelCount',nodes,'channelCount',function(value){const s=requireSlot(nodes,this);value=Number(value)>>>0;if(value<1||value>32)throw exception('NotSupportedError');s.channelCount=value});
 for(const [key,values]of [['channelCountMode',['max','clamped-max','explicit']],['channelInterpretation',['speakers','discrete']]])getter('AudioNode',key,nodes,key,function(value){const s=requireSlot(nodes,this);value=String(value);if(!values.includes(value))throw new TypeError('Invalid channel mode');s[key]=value});
 getter('AudioDestinationNode','maxChannelCount',nodes,'channelCount');
 method('AudioNode','connect',function connect(destination,output=0,input=0){const a=requireSlot(nodes,this),b=nodes.get(destination);if(!b){if(params.has(destination))unsupported('AudioParam connections');throw new TypeError('Expected AudioNode')}output=Number(output)>>>0;input=Number(input)>>>0;if(a.context!==b.context)throw exception('InvalidAccessError');if(output>=a.numberOfOutputs||input>=b.numberOfInputs)throw exception('IndexSizeError');if(!b.incoming.includes(this))b.incoming.push(this);return destination});
 method('AudioNode','disconnect',function disconnect(destination){const a=requireSlot(nodes,this),all=requireSlot(contexts,a.context).nodes;if(arguments.length===0){for(const node of all){const s=nodes.get(node);s.incoming=s.incoming.filter(n=>n!==this)}return}if(typeof destination==='number'){if((destination>>>0)>=a.numberOfOutputs)throw exception('IndexSizeError');for(const node of all){const s=nodes.get(node);s.incoming=s.incoming.filter(n=>n!==this)}return}const b=requireSlot(nodes,destination);if(!b.incoming.includes(this))throw exception('InvalidAccessError');b.incoming=b.incoming.filter(n=>n!==this)});
 const makeSource=context=>makeNode(context,'AudioBufferSourceNode',0,1,{buffer:null,playbackRate:makeParam(context,1,'k-rate'),detune:makeParam(context,0,'k-rate'),loop:false,loopStart:0,loopEnd:0,started:false,startTime:0,offset:0,duration:Infinity,stopTime:Infinity,onended:null});
 function AudioBufferSourceNode(context,options={}){if(!new.target)throw new TypeError('Expected new');const object=makeSource(context);Object.setPrototypeOf(object,new.target.prototype);options=options??{};for(const key of ['buffer','loop','loopStart','loopEnd','channelCount','channelCountMode','channelInterpretation'])if(options[key]!==undefined)object[key]=options[key];for(const key of ['playbackRate','detune'])if(options[key]!==undefined)object[key].value=options[key];return object}
 install('AudioBufferSourceNode',AudioBufferSourceNode);
 getter('AudioBufferSourceNode','buffer',nodes,'buffer',function(value){const s=requireSlot(nodes,this);if(value!==null)requireSlot(buffers,value);if(s.bufferAssigned&&value!==null)throw exception('InvalidStateError');if(value!==null)s.bufferAssigned=true;s.buffer=value});
 for(const key of ['playbackRate','detune'])getter('AudioBufferSourceNode',key,nodes);
 for(const key of ['loop','loopStart','loopEnd'])getter('AudioBufferSourceNode',key,nodes,key,function(value){requireSlot(nodes,this)[key]=key==='loop'?!!value:finite(value)});
 getter('AudioScheduledSourceNode','onended',nodes,'onended',function(value){requireSlot(nodes,this).onended=typeof value==='function'?value:null});
 method('AudioBufferSourceNode','start',function start(when=0,offset=0,duration){const s=requireSlot(nodes,this);when=time(when);offset=time(offset);duration=duration===undefined?Infinity:time(duration);if(s.started)throw exception('InvalidStateError');s.started=true;s.startTime=when;s.offset=offset;s.duration=duration});
 method('AudioScheduledSourceNode','stop',function stop(when=0){const s=requireSlot(nodes,this);when=time(when);if(!s.started)throw exception('InvalidStateError');s.stopTime=when});
 const makeGain=context=>makeNode(context,'GainNode',1,1,{gain:makeParam(context,1)});
 function GainNode(context,options={}){if(!new.target)throw new TypeError('Expected new');const object=makeGain(context);Object.setPrototypeOf(object,new.target.prototype);options=options??{};if(options.gain!==undefined)object.gain.value=options.gain;for(const key of ['channelCount','channelCountMode','channelInterpretation'])if(options[key]!==undefined)object[key]=options[key];return object}
 install('GainNode',GainNode);getter('GainNode','gain',nodes);
 function OfflineAudioContext(channels,length,sampleRate){if(!new.target||arguments.length===0)throw new TypeError('Expected offline dimensions');if(typeof channels!=='object'&&arguments.length<3)throw new TypeError('Expected three arguments');const s={...dimensions(typeof channels==='object'?channels:{numberOfChannels:channels,length,sampleRate}),state:'suspended',currentTime:0,nodes:[],started:false,onstatechange:null,oncomplete:null};const object=new EventTarget();Object.setPrototypeOf(object,new.target.prototype);contexts.set(object,s);s.destination=makeNode(object,'AudioDestinationNode',1,0,{channelCount:s.numberOfChannels,channelCountMode:'explicit'});return object}
 install('OfflineAudioContext',OfflineAudioContext);
 Object.defineProperty(OfflineAudioContext,'length',{value:1,configurable:true});
 for(const key of ['sampleRate','currentTime','state','destination'])getter('BaseAudioContext',key,contexts);
 getter('OfflineAudioContext','length',contexts);
 for(const [type,key]of [['BaseAudioContext','onstatechange'],['OfflineAudioContext','oncomplete']])getter(type,key,contexts,key,function(value){requireSlot(contexts,this)[key]=typeof value==='function'?value:null});
 for(const name of Object.getOwnPropertyNames(BaseAudioContext.prototype)){const d=Object.getOwnPropertyDescriptor(BaseAudioContext.prototype,name);if(name!=='constructor'&&typeof d.value==='function')method('BaseAudioContext',name,function(){requireSlot(contexts,this);unsupported(name)})}
 method('BaseAudioContext','createBuffer',function createBuffer(channels,length,sampleRate){requireSlot(contexts,this);if(arguments.length<3)throw new TypeError('Expected three arguments');return new AudioBuffer({numberOfChannels:channels,length,sampleRate})});
 method('BaseAudioContext','createBufferSource',function createBufferSource(){return makeSource(this)});
 method('BaseAudioContext','createGain',function createGain(){return makeGain(this)});
 method('BaseAudioContext','decodeAudioData',function decodeAudioData(){try{requireSlot(contexts,this);unsupported('decodeAudioData')}catch(e){return Promise.reject(e)}});
 for(const name of ['suspend','resume'])method('OfflineAudioContext',name,function(){try{requireSlot(contexts,this);unsupported(name)}catch(e){return Promise.reject(e)}});
 function render(context){
  const c=requireSlot(contexts,context),cache=new Map(),visiting=new Set();if(nodes.get(c.destination).channelCount!==c.numberOfChannels)unsupported('destination channel reconfiguration');let storage=0,work=0;
  const allocate=count=>{storage+=count*c.length;if(storage>16*1024*1024)unsupported('graph storage limit');return Array.from({length:count},()=>new Float32Array(c.length))};
  const processedFrames=Math.ceil(c.length/128)*128;
  function sourceOutput(object,output){
   const n=nodes.get(object);n.ended=false;if(!n.started)return;
   const b=n.buffer&&buffers.get(n.buffer),start=n.startTime*c.sampleRate,first=Math.ceil(start),stop=Math.ceil(n.stopTime*c.sampleRate);
   if(!b){n.ended=first<processedFrames;return}
   if(params.get(n.playbackRate).timeline.length||params.get(n.detune).timeline.length)unsupported('scheduled source resampling');
   // The rate ratio is formed before scaling: reassociation changes rounding
   // observed by the mixed-rate phase oracle near Float32 midpoints.
   const speed=n.playbackRate.value*Math.pow(2,n.detune.value/1200),step=(b.sampleRate/c.sampleRate)*speed;
   if(!Number.isFinite(step)||Math.abs(step)>1024)unsupported('resampling ratio limit');
   const offset=Math.min(b.length,Math.round(n.offset*b.sampleRate)),duration=n.duration*b.sampleRate;
   const loopStart=Math.max(0,n.loopStart*b.sampleRate),loopEnd=n.loopEnd>0?Math.min(b.length,n.loopEnd*b.sampleRate):b.length;
   if(n.loop&&(step<0||!Number.isInteger(loopStart)||!Number.isInteger(loopEnd)||loopStart>=loopEnd||loopStart>=b.length||offset>=b.length))unsupported('loop interval');
   // Starts occur at ceil(frame), while their fractional remainder advances the
   // source phase. Offset itself is rounded to the nearest source sample.
   let position=offset+(first-start)*step;
   for(let frame=first;frame<=processedFrames;frame++){
    if(++work>16*1024*1024)unsupported('source evaluation limit');
    const elapsed=frame-start;
    if(frame>=stop||elapsed*Math.abs(step)>=duration){n.ended=true;break}
    if(n.loop&&position>=loopEnd)position=loopStart+(position-loopStart)%(loopEnd-loopStart);
    if(position<0||position>=b.length){n.ended=true;break}
    if(frame===processedFrames)break;
    if(output&&frame<c.length){
    let lower=Math.floor(position),upper=lower+1;
    if(n.loop&&upper>=loopEnd)upper=loopStart;
    // Native linear interpolation extrapolates the last pair at the non-loop
    // edge; a loop instead interpolates toward its first sample.
    else if(upper>=b.length){lower=Math.max(0,b.length-2);upper=b.length-1}
    const fraction=position-lower;
    for(let ch=0;ch<output.length;ch++){const values=b.channels[ch],a=values[lower];output[ch][frame]=fraction===0?a:a+(values[upper]-a)*fraction}
    }
    // Carry phase across frames and render quanta; recomputing from elapsed
    // time loses observable rounding, even when the difference is tiny.
    position+=step;
   }
  }
  function nodeOutput(object){
   if(cache.has(object))return cache.get(object);if(visiting.has(object))unsupported('feedback graphs');visiting.add(object);const n=nodes.get(object);let result;
   if(n.type==='AudioBufferSourceNode'){
    const b=n.buffer&&buffers.get(n.buffer);result=allocate(b?b.numberOfChannels:1);
    sourceOutput(object,result);
   }else{
    const inputs=n.incoming.map(nodeOutput),maximum=Math.max(1,...inputs.map(v=>v.length));let count=n.channelCountMode==='explicit'?n.channelCount:n.channelCountMode==='clamped-max'?Math.min(n.channelCount,maximum):maximum;
    if(n.channelInterpretation==='speakers'&&inputs.some(v=>v.length!==count&&(count>2||v.length>2)))unsupported('multichannel speaker conversion');result=allocate(count);
    work+=inputs.length*count*c.length;if(work>16*1024*1024)unsupported('graph mixing limit');
    for(const input of inputs)for(let ch=0;ch<count;ch++)for(let frame=0;frame<c.length;frame++){
     const value=n.channelInterpretation==='discrete'?(input[ch]?.[frame]??0):count===1&&input.length===2?Math.fround(Math.fround(input[0][frame]*.5)+Math.fround(input[1][frame]*.5)):input[Math.min(ch,input.length-1)][frame];result[ch][frame]=Math.fround(result[ch][frame]+value);
    }
    if(n.type==='GainNode')for(let frame=0;frame<c.length;frame++){const gain=parameterAt(n.gain,frame,c.sampleRate);for(const ch of result)ch[frame]=Math.fround(ch[frame]*gain)}
   }
   visiting.delete(object);cache.set(object,result);return result;
  }
  const samples=nodeOutput(c.destination),buffer=new AudioBuffer({numberOfChannels:c.numberOfChannels,length:c.length,sampleRate:c.sampleRate});for(let ch=0;ch<samples.length;ch++)buffer.copyToChannel(samples[ch],ch);return buffer;
 }
 method('OfflineAudioContext','startRendering',function startRendering(){try{const c=requireSlot(contexts,this);if(c.started)throw exception('InvalidStateError');c.started=true;c.state='running';return new Promise((resolve,reject)=>{setTimeout(()=>{
  emit(this,'statechange');try{const buffer=render(this);c.currentTime=Math.ceil(c.length/128)*128/c.sampleRate;c.state='closed';for(const object of c.nodes){const n=nodes.get(object);if(n.type==='AudioBufferSourceNode'&&n.ended)emit(object,'ended')}emit(this,'complete',buffer);resolve(buffer)}catch(e){c.state='closed';reject(e)}setTimeout(()=>emit(this,'statechange'),0);
 },0)})}catch(e){return Promise.reject(e)}});
})();
