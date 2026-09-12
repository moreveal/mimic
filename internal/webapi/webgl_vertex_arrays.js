// Vertex arrays own pointer/enabled/divisor and element-buffer state. Generic
// attribute values belong to the context and survive vertex-array switches.
const vertexState=s=>{if(!s.defaultVertexState)s.defaultVertexState={attribs:new Map(),element:null};return s.vertexArray?resources.get(s.vertexArray):s.defaultVertexState};
const bufferBinding=(s,t)=>t===34963?vertexState(s).element:s.bindings.get(t);
const setBufferBinding=(s,t,value)=>{if(t===34963)vertexState(s).element=value;else s.bindings.set(t,value)};
const vaoPrototype=kind==='webgl2'?WebGLVertexArrayObject.prototype:Object.create(Object.prototype);
if(kind==='webgl')Object.defineProperty(vaoPrototype,Symbol.toStringTag,{value:'WebGLVertexArrayObjectOES',configurable:true});
const vaoType=kind==='webgl2'?'WebGLVertexArrayObject':'WebGLVertexArrayObjectOES';
const createVertexArray=s=>{const value=Object.create(vaoPrototype);resources.set(value,{owner:s,type:vaoType,deleted:false,bound:false,attribs:new Map(),element:null});return value};
const bindVertexArray=(s,value)=>{if(value!==null){const r=resource(s,value,vaoType);if(!r)return;r.bound=true}s.vertexArray=value};
const isVertexArray=(s,value)=>{const r=resources.get(value);return !!r&&r.owner===s&&r.type===vaoType&&r.bound&&!r.deleted&&r.generation===(s.generation||0)};
const deleteVertexArray=(s,value)=>{if(value===null)return;const prior=resources.get(value);if(prior?.owner===s&&prior.type===vaoType&&prior.deleted)return;const r=resource(s,value,vaoType);if(!r)return;r.deleted=true;r.attribs.clear();r.element=null;if(s.vertexArray===value)s.vertexArray=null};
if(kind==='webgl')extension('OES_vertex_array_object','OESVertexArrayObject',{VERTEX_ARRAY_BINDING_OES:34229},{createVertexArrayOES:{length:0,call:createVertexArray},bindVertexArrayOES:{length:1,call:bindVertexArray},deleteVertexArrayOES:{length:1,call:deleteVertexArray},isVertexArrayOES:{length:1,call:isVertexArray}});
else for(const [name,fn]of Object.entries({createVertexArray,bindVertexArray,deleteVertexArray,isVertexArray}))method(name,fn);
if(kind==='webgl')extension('OES_element_index_uint','OESElementIndexUint');
const elementIndices=(s,count,type,offset)=>{count=Number(count)|0;type=Number(type)>>>0;offset=Number(offset);if(count<0||offset<0){error(s,1281);return null}const size={5121:1,5123:2,5125:4}[type];if(!size||type===5125&&kind==='webgl'&&!s.extensions.has('OES_element_index_uint')){error(s,1280);return null}if(offset%size){error(s,1282);return null}const value=bufferBinding(s,34963),b=resources.get(value);if(!b||b.deleted||offset+count*size>b.data.length){error(s,1282);return null}if(count>65536)fail('index observation limit');const view=new DataView(b.data.buffer,b.data.byteOffset,b.data.byteLength),indices=Array.from({length:count},(_,i)=>size===1?view.getUint8(offset+i):size===2?view.getUint16(offset+i*2,true):view.getUint32(offset+i*4,true));if(kind==='webgl2'&&indices.includes(2**(size*8)-1))fail('primitive restart observations');return indices};
const drawElements=(s,mode,count,type,offset)=>{if(!resources.get(s.program)?.linked){error(s,1282);return}const indices=elementIndices(s,count,type,offset);if(indices)drawVertices(s,mode,0,indices.length,indices)};
method('drawElements',drawElements);
const vertexAttribDivisor=(s,index,value)=>{const a=attrib(s,index);if(a)a.divisor=Number(value)>>>0};
// Validate every instance before publishing any of its pending commands. No
// author callbacks run during this internal transaction; preserve the sticky error.
const drawBatch=(s,run)=>{const pending=s.drawCommands||[],priorError=s.error;s.drawCommands=[];s.error=0;try{run();if(!s.error)pending.push(...s.drawCommands)}finally{s.error=priorError||s.error;s.drawCommands=pending}};
const drawArraysInstanced=(s,mode,first,count,instances)=>{if(!resources.get(s.program)?.linked){error(s,1282);return}if((Number(first)|0)<0||(Number(count)|0)<0){error(s,1281);return}if((Number(mode)>>>0)>6){error(s,1280);return}instances=Number(instances)|0;if(instances<0){error(s,1281);return}if(instances>4096)fail('instance observation limit');drawBatch(s,()=>{for(let instance=0;instance<instances;instance++){drawVertices(s,mode,first,count,null,instance);if(s.error)break}})};
const drawElementsInstanced=(s,mode,count,type,offset,instances)=>{if(!resources.get(s.program)?.linked){error(s,1282);return}instances=Number(instances)|0;if(instances<0){error(s,1281);return}if(instances>4096)fail('instance observation limit');const indices=elementIndices(s,count,type,offset);if(!indices)return;drawBatch(s,()=>{for(let instance=0;instance<instances;instance++){drawVertices(s,mode,0,indices.length,indices,instance);if(s.error)break}})};
if(kind==='webgl')extension('ANGLE_instanced_arrays','ANGLEInstancedArrays',{VERTEX_ATTRIB_ARRAY_DIVISOR_ANGLE:35070},{vertexAttribDivisorANGLE:{length:2,call:vertexAttribDivisor},drawArraysInstancedANGLE:{length:4,call:drawArraysInstanced},drawElementsInstancedANGLE:{length:5,call:drawElementsInstanced}});
else for(const [name,fn]of Object.entries({vertexAttribDivisor,drawArraysInstanced,drawElementsInstanced}))method(name,fn);
