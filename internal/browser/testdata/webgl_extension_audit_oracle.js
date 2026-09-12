(async()=>{
const out={};
for(const kind of ['webgl','webgl2']){
 const canvas=new OffscreenCanvas(4,4),g=canvas.getContext(kind,{antialias:false}),r=out[kind]={names:g.getSupportedExtensions(),extensions:{}};
 const texture=()=>{const t=g.createTexture();g.bindTexture(g.TEXTURE_2D,t);return t};
 const framebuffer=format=>{const f=g.createFramebuffer(),b=g.createRenderbuffer();g.bindFramebuffer(g.FRAMEBUFFER,f);g.bindRenderbuffer(g.RENDERBUFFER,b);g.renderbufferStorage(g.RENDERBUFFER,format,4,4);const storageError=g.getError();g.framebufferRenderbuffer(g.FRAMEBUFFER,g.COLOR_ATTACHMENT0,g.RENDERBUFFER,b);return [storageError,g.checkFramebufferStatus(g.FRAMEBUFFER),g.getFramebufferAttachmentParameter(g.FRAMEBUFFER,g.COLOR_ATTACHMENT0,33297),g.getError()]};
 const shader=(directive,body='void main(){gl_FragColor=vec4(1.0);}')=>{const s=g.createShader(g.FRAGMENT_SHADER);g.shaderSource(s,(kind==='webgl2'?'#version 300 es\n':'')+'#extension '+directive+' : require\nprecision mediump float;\n'+(kind==='webgl2'?body.replace('void main()', 'out vec4 color;void main()').replaceAll('gl_FragColor','color'):body));g.compileShader(s);return g.getShaderParameter(s,g.COMPILE_STATUS)};
 for(const name of r.names){const e=g.getExtension(name),entry=r.extensions[name]={tag:Object.prototype.toString.call(e),same:e===g.getExtension(name.toLowerCase())};
  try{let v;
   switch(name){
    case 'EXT_color_buffer_half_float':v=framebuffer(34842);break;
    case 'EXT_color_buffer_float':case 'WEBGL_color_buffer_float':v=framebuffer(34836);break;
    case 'EXT_render_snorm':v=framebuffer(36759);break;
    case 'EXT_texture_norm16':v=framebuffer(e.RGBA16_EXT);break;
    case 'EXT_blend_minmax':g.blendEquation(e.MIN_EXT);v=g.getParameter(g.BLEND_EQUATION_RGB);break;
    case 'EXT_float_blend':{g.getExtension(kind==='webgl'?'WEBGL_color_buffer_float':'EXT_color_buffer_float');const state=framebuffer(34836),p=g.createProgram();for(const [type,source]of [[g.VERTEX_SHADER,'void main(){gl_Position=vec4(0.0,0.0,0.0,1.0);}'],[g.FRAGMENT_SHADER,'precision mediump float;void main(){gl_FragColor=vec4(1.0);}']]){const sh=g.createShader(type);g.shaderSource(sh,source);g.compileShader(sh);g.attachShader(p,sh)}g.linkProgram(p);g.useProgram(p);g.enable(g.BLEND);g.drawArrays(g.TRIANGLES,0,3);v=[state,g.getProgramParameter(p,g.LINK_STATUS),g.getError()];g.useProgram(null);break;}
    case 'WEBGL_blend_func_extended':g.blendFunc(e.SRC1_COLOR_WEBGL,g.ONE);v=[g.getParameter(g.BLEND_SRC_RGB),g.getParameter(e.MAX_DUAL_SOURCE_DRAW_BUFFERS_WEBGL)];break;
    case 'EXT_depth_clamp':g.enable(e.DEPTH_CLAMP_EXT);v=g.isEnabled(e.DEPTH_CLAMP_EXT);break;
    case 'EXT_clip_control':e.clipControlEXT(e.UPPER_LEFT_EXT,e.ZERO_TO_ONE_EXT);v=[g.getParameter(e.CLIP_ORIGIN_EXT),g.getParameter(e.CLIP_DEPTH_MODE_EXT)];break;
    case 'EXT_polygon_offset_clamp':e.polygonOffsetClampEXT(1,2,3);v=[g.getParameter(g.POLYGON_OFFSET_FACTOR),g.getParameter(g.POLYGON_OFFSET_UNITS),g.getParameter(e.POLYGON_OFFSET_CLAMP_EXT)];break;
    case 'WEBGL_polygon_mode':e.polygonModeWEBGL(g.FRONT_AND_BACK,e.LINE_WEBGL);v=g.getParameter(e.POLYGON_MODE_WEBGL);break;
    case 'WEBGL_provoking_vertex':e.provokingVertexWEBGL(e.FIRST_VERTEX_CONVENTION_WEBGL);v=g.getParameter(e.PROVOKING_VERTEX_WEBGL);break;
    case 'WEBGL_clip_cull_distance':g.enable(e.CLIP_DISTANCE0_WEBGL);v=[g.isEnabled(e.CLIP_DISTANCE0_WEBGL),g.getParameter(e.MAX_CLIP_DISTANCES_WEBGL),g.getParameter(e.MAX_CULL_DISTANCES_WEBGL)];break;
    case 'OES_shader_multisample_interpolation':v=[g.getParameter(e.MIN_FRAGMENT_INTERPOLATION_OFFSET_OES),g.getParameter(e.MAX_FRAGMENT_INTERPOLATION_OFFSET_OES),g.getParameter(e.FRAGMENT_INTERPOLATION_OFFSET_BITS_OES)];break;
    case 'OVR_multiview2':v=g.getParameter(e.MAX_VIEWS_OVR);break;
    case 'EXT_texture_mirror_clamp_to_edge':texture();g.texParameteri(g.TEXTURE_2D,g.TEXTURE_WRAP_S,e.MIRROR_CLAMP_TO_EDGE_EXT);v=g.getTexParameter(g.TEXTURE_2D,g.TEXTURE_WRAP_S);break;
    case 'WEBGL_stencil_texturing':texture();g.texParameteri(g.TEXTURE_2D,e.DEPTH_STENCIL_TEXTURE_MODE_WEBGL,e.STENCIL_INDEX_WEBGL);v=g.getTexParameter(g.TEXTURE_2D,e.DEPTH_STENCIL_TEXTURE_MODE_WEBGL);break;
    case 'EXT_texture_filter_anisotropic':texture();g.texParameterf(g.TEXTURE_2D,e.TEXTURE_MAX_ANISOTROPY_EXT,2);v=[g.getTexParameter(g.TEXTURE_2D,e.TEXTURE_MAX_ANISOTROPY_EXT),g.getParameter(e.MAX_TEXTURE_MAX_ANISOTROPY_EXT)];break;
    case 'OES_standard_derivatives':g.hint(e.FRAGMENT_SHADER_DERIVATIVE_HINT_OES,g.NICEST);v=g.getParameter(e.FRAGMENT_SHADER_DERIVATIVE_HINT_OES);break;
    case 'OES_vertex_array_object':{const a=e.createVertexArrayOES();e.bindVertexArrayOES(a);v=[e.isVertexArrayOES(a),g.getParameter(e.VERTEX_ARRAY_BINDING_OES)===a];e.deleteVertexArrayOES(a);break;}
    case 'ANGLE_instanced_arrays':e.vertexAttribDivisorANGLE(0,2);v=g.getVertexAttrib(0,e.VERTEX_ATTRIB_ARRAY_DIVISOR_ANGLE);break;
    case 'OES_element_index_uint':{const b=g.createBuffer();g.bindBuffer(g.ELEMENT_ARRAY_BUFFER,b);g.bufferData(g.ELEMENT_ARRAY_BUFFER,new Uint32Array([0]),g.STATIC_DRAW);v=g.getBufferParameter(g.ELEMENT_ARRAY_BUFFER,g.BUFFER_SIZE);break;}
    case 'OES_texture_float':case 'OES_texture_half_float':texture();g.texImage2D(g.TEXTURE_2D,0,g.RGBA,1,1,0,g.RGBA,name==='OES_texture_float'?g.FLOAT:e.HALF_FLOAT_OES,null);v=g.getError();break;
    case 'OES_texture_float_linear':case 'OES_texture_half_float_linear':texture();g.texParameteri(g.TEXTURE_2D,g.TEXTURE_MIN_FILTER,g.LINEAR);v=g.getTexParameter(g.TEXTURE_2D,g.TEXTURE_MIN_FILTER);break;
    case 'WEBGL_depth_texture':texture();g.texImage2D(g.TEXTURE_2D,0,g.DEPTH_COMPONENT,2,2,0,g.DEPTH_COMPONENT,g.UNSIGNED_SHORT,null);v=g.getError();break;
    case 'EXT_sRGB':texture();g.texImage2D(g.TEXTURE_2D,0,e.SRGB_ALPHA_EXT,1,1,0,e.SRGB_ALPHA_EXT,g.UNSIGNED_BYTE,null);v=g.getError();break;
    case 'OES_fbo_render_mipmap':{const t=texture(),f=g.createFramebuffer();g.texImage2D(g.TEXTURE_2D,1,g.RGBA,2,2,0,g.RGBA,g.UNSIGNED_BYTE,null);g.bindFramebuffer(g.FRAMEBUFFER,f);g.framebufferTexture2D(g.FRAMEBUFFER,g.COLOR_ATTACHMENT0,g.TEXTURE_2D,t,1);v=g.checkFramebufferStatus(g.FRAMEBUFFER);break;}
    case 'WEBGL_draw_buffers':g.bindFramebuffer(g.FRAMEBUFFER,null);e.drawBuffersWEBGL([g.NONE]);v=[g.getParameter(e.DRAW_BUFFER0_WEBGL),g.getParameter(e.MAX_DRAW_BUFFERS_WEBGL)];e.drawBuffersWEBGL([g.BACK]);break;
    case 'OES_draw_buffers_indexed':e.colorMaskiOES(0,false,true,false,true);v=Array.from(g.getIndexedParameter(g.COLOR_WRITEMASK,0));break;
    case 'WEBGL_multi_draw':e.multiDrawArraysWEBGL(g.TRIANGLES,new Int32Array([0]),0,new Int32Array([0]),0,1);v=g.getError();break;
    case 'WEBGL_debug_renderer_info':v=[typeof g.getParameter(e.UNMASKED_VENDOR_WEBGL),typeof g.getParameter(e.UNMASKED_RENDERER_WEBGL)];break;
    case 'WEBGL_debug_shaders':{const s=g.createShader(g.VERTEX_SHADER);g.shaderSource(s,'void main(){gl_Position=vec4(0.0);}');g.compileShader(s);v=[g.getShaderParameter(s,g.COMPILE_STATUS),e.getTranslatedShaderSource(s).length>0];break;}
    case 'KHR_parallel_shader_compile':{const s=g.createShader(g.VERTEX_SHADER);g.shaderSource(s,'void main(){gl_Position=vec4(0.0);}');g.compileShader(s);v=typeof g.getShaderParameter(s,e.COMPLETION_STATUS_KHR);break;}
    case 'EXT_disjoint_timer_query':case 'EXT_disjoint_timer_query_webgl2':{const w=kind==='webgl',q=w?e.createQueryEXT():g.createQuery();v=[w?e.getQueryEXT(e.TIME_ELAPSED_EXT,e.QUERY_COUNTER_BITS_EXT):g.getQuery(e.TIME_ELAPSED_EXT,e.QUERY_COUNTER_BITS_EXT),g.getParameter(e.GPU_DISJOINT_EXT)];if(w){e.beginQueryEXT(e.TIME_ELAPSED_EXT,q);e.endQueryEXT(e.TIME_ELAPSED_EXT)}else{g.beginQuery(e.TIME_ELAPSED_EXT,q);g.endQuery(e.TIME_ELAPSED_EXT)}g.finish();await new Promise(resolve=>setTimeout(resolve,30));v.push(w?e.getQueryObjectEXT(q,e.QUERY_RESULT_AVAILABLE_EXT):g.getQueryParameter(q,g.QUERY_RESULT_AVAILABLE));break;}
    case 'WEBGL_lose_context':v='deferred-until-after-other-operations';break;
    default:if(name.includes('compression')||name.includes('compressed_texture')){const formats=Array.from(g.getParameter(g.COMPRESSED_TEXTURE_FORMATS));v=Object.keys(Object.getPrototypeOf(e)).filter(k=>typeof e[k]==='number').every(k=>formats.includes(e[k]));}else v=shader('GL_'+name);
   }
   entry.operation=v;entry.error=g.getError();
  }catch(error){entry.exception=error.name;entry.message=error.message;g.getError()}
 }
 const loss=g.getExtension('WEBGL_lose_context');loss.loseContext();r.extensions.WEBGL_lose_context.operation=[g.isContextLost(),g.getError()];
}
return out;
})()