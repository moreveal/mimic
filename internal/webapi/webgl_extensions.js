// Extensions register behavior and state transitions together with their names.
// Merely adding a name to the native inventory is not extension support.
const extensionDefinitions=new Map(),extensionSlots=new WeakMap();
const extension=(name,tag,constants={},methods={},enable=()=>{})=>{
 const prototype=Object.create(Object.prototype),constructor={[tag](){throw new TypeError('Illegal constructor')}}[tag];
 if(typeof markNative==='function')markNative(constructor,tag);
 Object.defineProperty(prototype,'constructor',{value:constructor,writable:true,configurable:true});Object.defineProperty(prototype,Symbol.toStringTag,{value:tag,configurable:true});
 for(const [key,value]of Object.entries(constants))Object.defineProperty(prototype,key,{value,enumerable:true});
 for(const [key,record]of Object.entries(methods)){const fn={[key](...args){const slot=extensionSlots.get(this),owner=slot?.owner;if(!owner||slot.name!==name)throw new TypeError('Illegal invocation');if(args.length<record.length)throw new TypeError('Not enough arguments');return record.call(owner,...args)}}[key];Object.defineProperty(fn,'length',{value:record.length});if(typeof markNative==='function')markNative(fn,key);Object.defineProperty(prototype,key,{value:fn,enumerable:true,writable:true,configurable:true})}
 extensionDefinitions.set(name,{prototype,enable});
};
extension('WEBGL_debug_renderer_info','WebGLDebugRendererInfo',{UNMASKED_VENDOR_WEBGL:37445,UNMASKED_RENDERER_WEBGL:37446},{},s=>{s.debug=true});
extension('EXT_texture_filter_anisotropic','EXTTextureFilterAnisotropic',{TEXTURE_MAX_ANISOTROPY_EXT:34046,MAX_TEXTURE_MAX_ANISOTROPY_EXT:34047});
extension('EXT_color_buffer_half_float','EXTColorBufferHalfFloat',{RGBA16F_EXT:34842,RGB16F_EXT:34843,FRAMEBUFFER_ATTACHMENT_COMPONENT_TYPE_EXT:33297,UNSIGNED_NORMALIZED_EXT:35863});
if(kind==='webgl2')extension('EXT_color_buffer_float','EXTColorBufferFloat');else extension('OES_standard_derivatives','OESStandardDerivatives',{FRAGMENT_SHADER_DERIVATIVE_HINT_OES:35723});
extension('KHR_parallel_shader_compile','KHRParallelShaderCompile',{COMPLETION_STATUS_KHR:37297});
const timerName=kind==='webgl2'?'EXT_disjoint_timer_query_webgl2':'EXT_disjoint_timer_query';
const queryClock=performance.now.bind(performance),timerQueryPrototype=Object.create(Object.prototype);
Object.defineProperty(timerQueryPrototype,Symbol.toStringTag,{value:'WebGLTimerQueryEXT',configurable:true});
const createQuery=s=>{const value=Object.create(kind==='webgl2'?WebGLQuery.prototype:timerQueryPrototype);resources.set(value,{owner:s,type:kind==='webgl2'?'WebGLQuery':'WebGLTimerQueryEXT',deleted:false,bound:false,target:0,available:false,result:0,pending:false});return value};
const query=(s,value)=>resource(s,value,kind==='webgl2'?'WebGLQuery':'WebGLTimerQueryEXT');
const validQueryTarget=(s,target)=>target===35007&&s.extensions.has(timerName)||kind==='webgl2'&&[35887,36202,35975].includes(target);
const activeQueries=s=>s.activeQueries||(s.activeQueries=new Map());
const beginQuery=(s,target,value)=>{target=Number(target)>>>0;if(!validQueryTarget(s,target)){error(s,1280);return}const q=query(s,value);if(!q)return;if(activeQueries(s).has(target)||q.active||q.target&&q.target!==target){error(s,1282);return}q.target=target;q.bound=true;q.active=true;q.pending=false;q.available=false;q.samplesPassed=false;q.started=queryClock();activeQueries(s).set(target,value)};
const endQuery=(s,target)=>{target=Number(target)>>>0;if(!validQueryTarget(s,target)){error(s,1280);return}const value=activeQueries(s).get(target);if(!value){error(s,1282);return}const q=resources.get(value);activeQueries(s).delete(target);q.active=false;q.pending=true;q.elapsed=Math.max(0,Math.floor((queryClock()-q.started)*1e6));(s.pendingQueries||(s.pendingQueries=new Set())).add(q)};
const getQuery=(s,target,pname)=>{target=Number(target)>>>0;pname=Number(pname)>>>0;if(pname===34916&&s.extensions.has(timerName)&&[35007,36392].includes(target))return target===35007?64:0;if(!validQueryTarget(s,target)||pname!==34917){error(s,1280);return null}return activeQueries(s).get(target)||null};
const getQueryResult=(s,value,pname)=>{const q=query(s,value);if(!q)return null;pname=Number(pname)>>>0;if((!q.bound&&kind==='webgl2')||q.active){error(s,1282);return null}if(pname===34919)return q.available;if(pname===34918)return q.result;error(s,1280);return null};
const deleteQuery=(s,value)=>{if(value===null)return;const prior=resources.get(value);if(prior?.owner===s&&prior.deleted)return;const q=query(s,value);if(!q)return;q.deleted=true;q.pending=false;s.pendingQueries?.delete(q);if(q.active){activeQueries(s).delete(q.target);q.active=false}};
const isQuery=(s,value)=>{const q=resources.get(value);return !!q&&q.owner===s&&q.type===(kind==='webgl2'?'WebGLQuery':'WebGLTimerQueryEXT')&&q.bound&&!q.deleted&&q.generation===(s.generation||0)};
const queryCounter=(s,value,target)=>{const q=query(s,value);if(!q)return;if(Number(target)!==36392){error(s,1280);return}if(q.active||q.target&&q.target!==36392){error(s,1282);return}q.target=36392;q.bound=true;q.available=false;q.pending=true;q.elapsed=0;(s.pendingQueries||(s.pendingQueries=new Set())).add(q)};
const queryMethods={createQueryEXT:{length:0,call:createQuery},deleteQueryEXT:{length:1,call:deleteQuery},isQueryEXT:{length:1,call:isQuery},beginQueryEXT:{length:2,call:beginQuery},endQueryEXT:{length:1,call:endQuery},getQueryEXT:{length:2,call:getQuery},getQueryObjectEXT:{length:2,call:getQueryResult},queryCounterEXT:{length:2,call:queryCounter}};
const queryConstants={QUERY_COUNTER_BITS_EXT:34916,TIME_ELAPSED_EXT:35007,TIMESTAMP_EXT:36392,GPU_DISJOINT_EXT:36795};
if(kind==='webgl')Object.assign(queryConstants,{CURRENT_QUERY_EXT:34917,QUERY_RESULT_EXT:34918,QUERY_RESULT_AVAILABLE_EXT:34919});
extension(timerName,kind==='webgl2'?'EXTDisjointTimerQueryWebGL2':'EXTDisjointTimerQuery',queryConstants,kind==='webgl2'?{queryCounterEXT:queryMethods.queryCounterEXT}:queryMethods);
if(kind==='webgl2')for(const [name,fn]of Object.entries({createQuery,deleteQuery,isQuery,beginQuery,endQuery,getQuery,getQueryParameter:getQueryResult}))method(name,fn);
const flushQueries=s=>{storage(s);const pending=[...(s.pendingQueries||[])];setTimeout(()=>{for(const q of pending)if(q.pending&&!q.deleted){q.pending=false;q.available=true;q.result=[35007,36392].includes(q.target)?q.elapsed:q.target===35975?0:Number(!!q.samplesPassed);s.pendingQueries.delete(q)}},0)};
method('flush',flushQueries);method('finish',flushQueries);
const restoreState=s=>{
 s.generation=(s.generation||0)+1;s.lost=false;s.error=0;s.restoreAllowed=false;
 s.extensions.clear();s.debug=false;s.bindings.clear();s.program=null;s.attribs=null;s.drawFramebuffer=null;s.readFramebuffer=null;s.renderbuffer=null;
 s.color=[0,0,0,0];s.mask=[true,true,true,true];s.viewport=[0,0,s.dim.width,s.dim.height];s.scissor=s.viewport.slice();s.pack=4;
 s.depthClear=1;s.stencilClear=0;s.depthMask=true;s.depthValue=1;s.stencilValue=0;s.stencilMaskFront=s.stencilMaskBack=4294967295;s.derivativeHint=s.mipmapHint=4352;
 for(const key of s.enabled.keys())s.enabled.set(key,key===3024);
 s.textureUnits?.clear();s.activeTexture=0;s.unpackAlignment=4;s.pixels=null;s.samplePixels=null;s.drawCommands=[];s.activeQueries?.clear();s.pendingQueries?.clear();s.drawingBufferColorSpace=s.unpackColorSpace='srgb';
};
extension('WEBGL_lose_context','WebGLLoseContext',{}, {
 loseContext:{length:0,call:s=>{if(s.lost){s.error=1282;return}s.lost=true;s.error=37442;s.generation=(s.generation||0)+1;s.restoreAllowed=false;for(const q of s.pendingQueries||[])q.pending=false;setTimeout(()=>{const event=new Event('webglcontextlost',{cancelable:true});Object.defineProperty(event,'statusMessage',{value:'',enumerable:true});s.canvas.dispatchEvent(event);s.restoreAllowed=event.defaultPrevented},0)}},
 restoreContext:{length:0,call:s=>{if(!s.lost||!s.restoreAllowed){s.error=1282;return}setTimeout(()=>{if(!s.lost)return;restoreState(s);const event=new Event('webglcontextrestored',{cancelable:true});Object.defineProperty(event,'statusMessage',{value:'',enumerable:true});s.canvas.dispatchEvent(event)},0)}}
});


extension('WEBGL_debug_shaders','WebGLDebugShaders',{}, {getTranslatedShaderSource:{length:1,call:(s,value)=>{const shader=resource(s,value,'WebGLShader');if(!shader)return '';return shader.compiled&&shader.evaluation?JSON.stringify(shader.evaluation,(key,value)=>value instanceof Set?[...value]:value):''}}});
