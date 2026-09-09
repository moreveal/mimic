// Shared cancellation and UTF-8 decoder semantics.
    const abortSlots=new WeakMap(),controllerSlots=new WeakMap();
    const abort=(signal,reason)=>{const s=abortSlots.get(signal);if(s.aborted)return;s.aborted=true;s.reason=reason===undefined?new DOMException('signal is aborted without reason','AbortError'):reason;dispatchTrusted(signal,new Event('abort'))};
    class AbortSignal extends EventTarget {
      constructor(token){super();if(token!==hostToken)throw new TypeError('Illegal constructor');abortSlots.set(this,{aborted:false,reason:undefined,onabort:null})}
      get aborted(){return abortSlots.get(this).aborted} get reason(){return abortSlots.get(this).reason}
      get onabort(){return abortSlots.get(this).onabort} set onabort(value){abortSlots.get(this).onabort=value}
      throwIfAborted(){if(this.aborted)throw this.reason}
      static abort(reason){const signal=new AbortSignal(hostToken);abort(signal,reason);return signal}
      static timeout(ms){ms=Number(ms);if(ms<0||!Number.isFinite(ms))throw new RangeError('Invalid timeout');const signal=new AbortSignal(hostToken);setTimeout(()=>abort(signal,new DOMException('signal timed out','TimeoutError')),ms);return signal}
      static any(signals){const result=new AbortSignal(hostToken);for(const signal of signals){if(!(signal instanceof AbortSignal))throw new TypeError('Expected AbortSignal');if(signal.aborted){abort(result,signal.reason);break}signal.addEventListener('abort',()=>abort(result,signal.reason))}return result}
    }
    class AbortController {constructor(){controllerSlots.set(this,new AbortSignal(hostToken))}get signal(){return controllerSlots.get(this)}abort(reason){abort(this.signal,reason)}}
    expose('AbortSignal',AbortSignal);expose('AbortController',AbortController);
    const decoderSlots=new WeakMap();
    class TextDecoder {
      constructor(label='utf-8',options={}){label=String(label).trim().toLowerCase();if(!['utf-8','utf8','unicode-1-1-utf-8'].includes(label))throw new RangeError('Unsupported encoding: '+label);decoderSlots.set(this,{fatal:!!options.fatal,ignoreBOM:!!options.ignoreBOM,bytes:[],bom:false})}
      get encoding(){return 'utf-8'}get fatal(){return decoderSlots.get(this).fatal}get ignoreBOM(){return decoderSlots.get(this).ignoreBOM}
      decode(input,options={}){
        const s=decoderSlots.get(this),bytes=s.bytes.concat(input==null?[]:Array.from(ArrayBuffer.isView(input)?new Uint8Array(input.buffer,input.byteOffset,input.byteLength):new Uint8Array(input)));s.bytes=[];let output='';
        const emit=point=>{if(!s.bom){s.bom=true;if(point===0xfeff&&!s.ignoreBOM)return}output+=String.fromCodePoint(point)};
        const error=()=>{if(s.fatal)throw new TypeError('Invalid encoded data');emit(0xfffd)};
        for(let i=0;i<bytes.length;){const start=i,b=bytes[i++];if(b<128){emit(b);continue}const count=b>=0xc2&&b<=0xdf?1:b>=0xe0&&b<=0xef?2:b>=0xf0&&b<=0xf4?3:0;if(!count){error();continue}
          let point=b&((1<<(6-count))-1),valid=true;
          for(let j=0;j<count;j++){
            if(i===bytes.length){if(options.stream){s.bytes=bytes.slice(start);valid=false;i=bytes.length}else{error();valid=false}break}
            const c=bytes[i],min=j===0&&b===0xe0?0xa0:j===0&&b===0xf0?0x90:0x80,max=j===0&&b===0xed?0x9f:j===0&&b===0xf4?0x8f:0xbf;
            if(c<min||c>max){error();valid=false;break}i++;point=(point<<6)|(c&63);
          }if(valid)emit(point);
        }if(!options.stream)s.bom=false;return output;
      }
    }
    expose('TextDecoder',TextDecoder);
