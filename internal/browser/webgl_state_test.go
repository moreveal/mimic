package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestWebGLStateMatchesFrozenOracle(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/webgl_state_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/webgl-state-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result json.RawMessage `json:"result"`
	}
	if err = json.Unmarshal(capture, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, "(()=>{const actual="+string(source)+",expected="+string(oracle.Result)+";return Object.keys(expected).every(k=>JSON.stringify(actual[k])===JSON.stringify(expected[k]))})()", true)
	})
}

func TestWebGLResourceStorageAndClear(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{const canvas=new OffscreenCanvas(2,2),gl=canvas.getContext('webgl2',{antialias:false}),b=gl.createBuffer();if(gl.isBuffer(b))throw Error('unused buffer');gl.bindBuffer(gl.ARRAY_BUFFER,b);if(!gl.isBuffer(b)||gl.getParameter(gl.ARRAY_BUFFER_BINDING)!==b)throw Error('binding identity');const input=new Uint8Array([1,2,3,4]);gl.bufferData(gl.ARRAY_BUFFER,input,gl.STATIC_DRAW);input[0]=99;gl.bufferSubData(gl.ARRAY_BUFFER,1,new Uint8Array([8,9]));const bytes=new Uint8Array(4);gl.getBufferSubData(gl.ARRAY_BUFFER,0,bytes);if(String(bytes)!=='1,8,9,4')throw Error('upload ownership '+bytes);gl.clearColor(1,0,0,1);gl.clear(gl.COLOR_BUFFER_BIT);gl.enable(gl.SCISSOR_TEST);gl.scissor(1,0,1,1);gl.clearColor(0,1,0,1);gl.clear(gl.COLOR_BUFFER_BIT);const pixels=new Uint8Array(16);gl.readPixels(0,0,2,2,gl.RGBA,gl.UNSIGNED_BYTE,pixels);if(String(pixels)!=='255,0,0,255,0,255,0,255,255,0,0,255,255,0,0,255')throw Error('clear storage '+pixels);const repeated=new Uint8Array(16);gl.readPixels(0,0,2,2,gl.RGBA,gl.UNSIGNED_BYTE,repeated);if(String(repeated)!==String(pixels))throw Error('unstable read');gl.deleteBuffer(b);return !gl.isBuffer(b)&&gl.getParameter(gl.ARRAY_BUFFER_BINDING)===null&&gl.getError()===0})()`, true)
	})
}

func TestWebGLResourceOwnershipAndUnsupportedExecution(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{const a=new OffscreenCanvas(1,1).getContext('webgl'),b=new OffscreenCanvas(1,1).getContext('webgl');const buffer=a.createBuffer();b.bindBuffer(b.ARRAY_BUFFER,buffer);if(b.getError()!==b.INVALID_OPERATION||b.getParameter(b.ARRAY_BUFFER_BINDING)!==null)throw Error('foreign ownership');const shader=a.createShader(a.VERTEX_SHADER);a.shaderSource(shader,'void main(){gl_Position=vec4(0.0);}');if(!a.getShaderSource(shader).includes('gl_Position'))throw Error('source missing');a.compileShader(shader);if(!a.getShaderParameter(shader,a.COMPILE_STATUS))throw Error('arithmetic shader rejected');a.drawArrays(a.TRIANGLES,0,3);if(a.getError()!==a.INVALID_OPERATION)throw Error('missing program');a.shaderSource(shader,'uniform sampler2D tex;void main(){gl_Position=texture2D(tex,vec2(0.0));}');a.compileShader(shader);if(!a.getShaderParameter(shader,a.COMPILE_STATUS))throw Error('2D sampler rejected');a.shaderSource(shader,'uniform samplerCube tex;void main(){gl_Position=textureCube(tex,vec3(0.0));}');try{a.compileShader(shader)}catch(e){return e.name==='NotSupportedError'&&!a.getShaderParameter(shader,a.COMPILE_STATUS)}return false})()`, true)
	})
}

func TestWebGLWorkerProfileAndClearParity(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `new Promise((resolve,reject)=>{function sample(){const gl=new OffscreenCanvas(1,1).getContext('webgl');const ext=gl.getExtension('WEBGL_debug_renderer_info');gl.clearColor(0,0,1,1);gl.clear(gl.COLOR_BUFFER_BIT);const bytes=new Uint8Array(4);gl.readPixels(0,0,1,1,gl.RGBA,gl.UNSIGNED_BYTE,bytes);return JSON.stringify([gl.getParameter(ext.UNMASKED_VENDOR_WEBGL),gl.getParameter(ext.UNMASKED_RENDERER_WEBGL),gl.getParameter(gl.MAX_TEXTURE_SIZE),Array.from(bytes)])}const expected=sample(),url=URL.createObjectURL(new Blob(['onmessage=()=>postMessage(('+sample.toString()+')())'])),worker=new Worker(url);worker.onerror=e=>reject(Error(e.message));worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data===expected)};worker.postMessage(null)})`, true)
	})
}

func TestWebGLCanvasSnapshotUsesDrawingBuffer(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{const canvas=new OffscreenCanvas(1,2),gl=canvas.getContext('webgl',{antialias:false});gl.clearColor(1,0,0,1);gl.clear(gl.COLOR_BUFFER_BIT);gl.enable(gl.SCISSOR_TEST);gl.scissor(0,0,1,1);gl.clearColor(0,1,0,1);gl.clear(gl.COLOR_BUFFER_BIT);const image=canvas.transferToImageBitmap(),dest=new OffscreenCanvas(1,2),ctx=dest.getContext('2d');ctx.drawImage(image,0,0);const pixels=ctx.getImageData(0,0,1,2).data;if(String(pixels)!=='255,0,0,255,0,255,0,255')throw Error('orientation or snapshot '+pixels);const reset=new Uint8Array(8);gl.readPixels(0,0,1,2,gl.RGBA,gl.UNSIGNED_BYTE,reset);return String(reset)==='0,0,0,0,0,0,0,0'})()`, true)
	})
}
