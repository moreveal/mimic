package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGPUDeviceReadonlyStateAfterSurfaceInitialization(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body>gpu fixture") }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		result, err := p.Evaluate(ctx, `(async()=>{
 const adapter=await navigator.gpu.requestAdapter(),device=await adapter.requestDevice();
 if(!(device instanceof GPUDevice)||!(device instanceof EventTarget))return 'brand';
 for(const key of ['adapterInfo','features','limits','queue','lost']){
 const descriptor=Object.getOwnPropertyDescriptor(GPUDevice.prototype,key);
 if(!descriptor||typeof descriptor.get!=='function'||descriptor.set!==undefined||Object.hasOwn(device,key))return 'shape '+key;
 const value=device[key];if(value!==device[key])return 'identity '+key;
 let threw=false;try{(()=>{'use strict';device[key]=null})()}catch(e){threw=e instanceof TypeError}if(!threw||device[key]!==value)return 'readonly '+key;
 }
 if(device.adapterInfo===adapter.info||device.limits===adapter.limits||typeof device.features.has!=='function')return 'state';
 device.pushErrorScope('validation');if(!(device.lost instanceof Promise)||await device.popErrorScope()!==null)return 'promises';
 await device.queue.onSubmittedWorkDone();device.queue.submit([]);
 let events=0;device.addEventListener('probe',()=>events++);device.dispatchEvent(new Event('probe'));if(events!==1)return 'event target';
 const handler=()=>{};device.onuncapturederror=handler;if(device.onuncapturederror!==handler)return 'handler';device.onuncapturederror=17;if(device.onuncapturederror!==null)return 'handler normalization';
 return true;
 })()`)
		if err != nil || result != true {
			t.Fatalf("GPUDevice state: %v %v", result, err)
		}
	})
}
