(()=>{
 const result={};
 const parameters=[32883,33000,33001,33901,34024,34045,34467,34852,35071,35076,35077,35371,35373,35374,35375,35376,35377,35379,35380,35657,35658,35659,35739,35968,35978,35979,36063,36183,36203,37137,37154,37157,37447,35738,3386,34921,36348,36349,36347,33902,34047,35723,36795,35661,35660,34930,34076,3408,3410,3411,3412,3413,3414,3415,32936,32937,2963,2968,36004,36005];
 const describe=value=>({type:Object.prototype.toString.call(value).slice(8,-1),value:ArrayBuffer.isView(value)?Array.from(value):value});
 for(const kind of ['webgl','webgl2']){
  const gl=new OffscreenCanvas(4,3).getContext(kind,{antialias:false});const constants={};for(let p=Object.getPrototypeOf(gl);p;p=Object.getPrototypeOf(p))for(const key of Object.getOwnPropertyNames(p)){const d=Object.getOwnPropertyDescriptor(p,key);if(typeof d.value==='number')constants[d.value]??=key}
  const entry={parameters:{},samples:{},invalid:[],extensions:gl.getSupportedExtensions()};
  for(const p of parameters){const value=gl.getParameter(p);entry.parameters[p]={name:constants[p],...describe(value),error:gl.getError()}}
  if(kind==='webgl2'){
   const formats=['R8','R8_SNORM','R16F','R32F','R8UI','R8I','R16UI','R16I','R32UI','R32I','RG8','RG8_SNORM','RG16F','RG32F','RG8UI','RG8I','RG16UI','RG16I','RG32UI','RG32I','RGB8','SRGB8','RGB565','R11F_G11F_B10F','RGB9_E5','RGB16F','RGB32F','RGB8UI','RGB8I','RGB16UI','RGB16I','RGB32UI','RGB32I','RGBA8','SRGB8_ALPHA8','RGBA8_SNORM','RGB5_A1','RGBA4','RGB10_A2','RGBA16F','RGBA32F','RGBA8UI','RGBA8I','RGB10_A2UI','RGBA16UI','RGBA16I','RGBA32UI','RGBA32I','DEPTH_COMPONENT16','DEPTH_COMPONENT24','DEPTH_COMPONENT32F','DEPTH24_STENCIL8','DEPTH32F_STENCIL8','STENCIL_INDEX8'];
   for(const name of formats){const value=gl.getInternalformatParameter(gl.RENDERBUFFER,gl[name],gl.SAMPLES);entry.samples[gl[name]]={name,...describe(value),error:gl.getError()}}
   for(const args of [[0,gl.RGBA8,gl.SAMPLES],[gl.FRAMEBUFFER,gl.RGBA8,gl.SAMPLES],[gl.RENDERBUFFER,0,gl.SAMPLES],[gl.RENDERBUFFER,gl.RGBA8,0]]){entry.invalid.push({args,...describe(gl.getInternalformatParameter(...args)),error:gl.getError()})}
  }
  entry.extensionResults={};
  for(const name of ['EXT_texture_filter_anisotropic','OES_standard_derivatives','EXT_color_buffer_float','EXT_color_buffer_half_float']){const ext=gl.getExtension(name);const item={available:!!ext,parameters:{},samples:{}};for(const p of [34047,35723])item.parameters[p]={...describe(gl.getParameter(p)),error:gl.getError()};if(kind==='webgl2')for(const format of [33325,33326,33327,33328,34842,34836,35898])item.samples[format]={...describe(gl.getInternalformatParameter(36161,format,32937)),error:gl.getError()};entry.extensionResults[name]=item}
  entry.precision={};for(const shader of [35632,35633,0])for(const precision of [36336,36337,36338,36339,36340,36341,0]){const f=gl.getShaderPrecisionFormat(shader,precision);entry.precision[shader+','+precision]={value:f?{rangeMin:f.rangeMin,rangeMax:f.rangeMax,precision:f.precision}:null,error:gl.getError()}}
  result[kind]=entry;
 }
 return result;
})()
