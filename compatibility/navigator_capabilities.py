"""Generic, exact-version capability observations; never visits external sites."""
import argparse
import asyncio
import json
import threading
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pyppeteer import connect
from oracle import (MODES, PINNED_PRODUCT, capture_metadata, default_profile_id,
                    prepare_page, product)

MEMBERS = "vendorSub productSub appCodeName doNotTrack userActivation scheduling geolocation bluetooth clipboard credentials ink devicePosture hid keyboard managed mediaCapabilities mediaSession mediaDevices permissions presentation serial serviceWorker virtualKeyboard wakeLock usb windowControlsOverlay xr storageBuckets locks connection storage webkitTemporaryStorage webkitPersistentStorage login protectedAudience deprecatedRunAdAuctionEnforcesKAnonymity".split()

SHAPE = r"""(names) => {
 const describe = d => ({enumerable:d.enumerable,configurable:d.configurable,
   writable:d.writable??null,get:d.get?Function.prototype.toString.call(d.get):null,
   set:d.set?Function.prototype.toString.call(d.set):null,
   getterShape:d.get?{name:d.get.name,length:d.get.length,keys:Reflect.ownKeys(d.get).map(String).sort()}:null,
   setterShape:d.set?{name:d.set.name,length:d.set.length,keys:Reflect.ownKeys(d.set).map(String).sort()}:null});
 const result={secure:isSecureContext,isolated:crossOriginIsolated, members:{}};
 for(const name of names){
   const row=result.members[name]={exposed:name in navigator};if(!row.exposed)continue;
   let owner=navigator;while(owner&&!Object.hasOwn(owner,name))owner=Object.getPrototypeOf(owner);
   row.owner=owner===Navigator.prototype?'Navigator.prototype':owner===navigator?'navigator':owner?.constructor?.name;
   row.descriptor=describe(Object.getOwnPropertyDescriptor(owner,name));
   try {const value=navigator[name];row.type=typeof value;
     if(value===null||typeof value!=='object')row.value=value;
     else {row.tag=Object.prototype.toString.call(value);row.constructor=value.constructor.name;
       row.constructorIdentity=value.constructor===globalThis[row.constructor];
       row.sameObject=value===navigator[name];row.ownKeys=Reflect.ownKeys(value).map(String);
       const proto=Object.getPrototypeOf(value);
       row.prototype=Object.getOwnPropertyNames(proto).sort();
       row.prototypeDescriptors=Object.fromEntries(row.prototype.map(name=>{
         const d=Object.getOwnPropertyDescriptor(proto,name), shape=describe(d);
         if(typeof d.value==='function'){shape.name=d.value.name;shape.length=d.value.length;shape.source=Function.prototype.toString.call(d.value);shape.functionKeys=Reflect.ownKeys(d.value).map(String).sort();
           const p=Object.getOwnPropertyDescriptor(d.value,'prototype');shape.prototypeProperty=p?{writable:p.writable,enumerable:p.enumerable,configurable:p.configurable}:null;}
         return [name,shape];
       }));
       row.chain=[];for(let p=proto;p;p=Object.getPrototypeOf(p))row.chain.push(Object.prototype.toString.call(p));}
   }catch(e){row.error={name:e.name,message:e.message}}
   const d=Object.getOwnPropertyDescriptor(owner,name);
   if(d.get)try{d.get.call({});row.illegalReceiver=null}catch(e){row.illegalReceiver={name:e.name,message:e.message}}
 }
 return result;
}"""

OPERATIONS = {
 'activation':'({active:navigator.userActivation.isActive,sticky:navigator.userActivation.hasBeenActive,pending:navigator.scheduling.isInputPending()})',
 'geolocation.current':"new Promise(resolve=>navigator.geolocation.getCurrentPosition(p=>resolve({unexpectedPosition:true}),e=>resolve({code:e.code,message:e.message})))",
 'connection':'({effectiveType:navigator.connection.effectiveType,downlink:navigator.connection.downlink,rtt:navigator.connection.rtt,saveData:navigator.connection.saveData})',
 'bluetooth.available':'navigator.bluetooth.getAvailability()',
 'bluetooth.devices':'navigator.bluetooth.getDevices()',
 'hid.devices':'navigator.hid.getDevices()', 'usb.devices':'navigator.usb.getDevices()',
 'serial.ports':'navigator.serial.getPorts()',
 'posture':'navigator.devicePosture.type',
 'credentials.get':'navigator.credentials.get({})',
 'credentials.create':'navigator.credentials.create({})',
 'credentials.prevent':'navigator.credentials.preventSilentAccess()',
 'permissions.geo':"navigator.permissions.query({name:'geolocation'}).then(p=>({state:p.state,name:p.name,tag:Object.prototype.toString.call(p)}))",
 'permissions.unknown':"navigator.permissions.query({name:'not-a-permission'})",
 'clipboard.read':'navigator.clipboard.readText().then(()=>({resolved:true}))',
 'media.enumerate':'navigator.mediaDevices.enumerateDevices().then(ds=>ds.map(d=>d.toJSON()))',
 'media.constraints':'navigator.mediaDevices.getSupportedConstraints()',
 'media.getUserMedia':'navigator.mediaDevices.getUserMedia({video:true})',
 'media.invalid':'navigator.mediaDevices.getUserMedia({})',
 'media.decoding':"navigator.mediaCapabilities.decodingInfo({type:'file',video:{contentType:'video/unknown',width:640,height:480,bitrate:1000,framerate:30}})",
 'media.mp4':"navigator.mediaCapabilities.decodingInfo({type:'file',video:{contentType:'video/mp4; codecs=\"avc1.42E01E\"',width:640,height:480,bitrate:1000,framerate:30}})",
 'media.session':'({metadata:navigator.mediaSession.metadata,playbackState:navigator.mediaSession.playbackState})',
 'presentation':'({defaultRequest:navigator.presentation.defaultRequest,receiver:navigator.presentation.receiver})',
 'storage.estimate':'navigator.storage.estimate()', 'storage.persisted':'navigator.storage.persisted()',
 'buckets.keys':'navigator.storageBuckets.keys()', 'locks.query':'navigator.locks.query()',
 'quota.temporary':'new Promise((resolve,reject)=>navigator.webkitTemporaryStorage.queryUsageAndQuota((usage,quota)=>resolve({usage,quota}),reject))',
 'quota.persistent':'new Promise((resolve,reject)=>navigator.webkitPersistentStorage.queryUsageAndQuota((usage,quota)=>resolve({usage,quota}),reject))',
 'quota.identity':'navigator.webkitTemporaryStorage===navigator.webkitPersistentStorage',
 'serviceWorker.controller':'navigator.serviceWorker.controller',
 'serviceWorker.registrations':'navigator.serviceWorker.getRegistrations()',
 'serviceWorker.registration':'navigator.serviceWorker.getRegistration()',
 'xr.vr':"navigator.xr.isSessionSupported('immersive-vr')", 'xr.ar':"navigator.xr.isSessionSupported('immersive-ar')",
 'xr.request':"navigator.xr.requestSession('immersive-vr')",
 'overlay':'({visible:navigator.windowControlsOverlay.visible,rect:navigator.windowControlsOverlay.getTitlebarAreaRect().toJSON()})',
 'keyboard.unlock':'navigator.keyboard.unlock()',
 'keyboard.lock':'navigator.keyboard.lock()',
 'keyboard.layout':'navigator.keyboard.getLayoutMap().then(m=>Object.fromEntries(m))',
 'bluetooth.request':'navigator.bluetooth.requestDevice({acceptAllDevices:true})',
 'hid.request':'navigator.hid.requestDevice({filters:[]})',
 'usb.request':'navigator.usb.requestDevice({filters:[]})',
 'serial.request':'navigator.serial.requestPort()',
 'buckets.lifecycle':"(async()=>{const b=await navigator.storageBuckets.open('capability-test');const r={name:b.name,persisted:await b.persisted(),expires:await b.expires(),tag:Object.prototype.toString.call(b)};await navigator.storageBuckets.delete('capability-test');return r})()",
 'locks.lifecycle':"navigator.locks.request('capability-test',async lock=>({name:lock.name,mode:lock.mode,tag:Object.prototype.toString.call(lock)}))",
 'login.status':"navigator.login.setStatus('logged-out')",
 'wakeLock.request':"navigator.wakeLock.request().then(async s=>{const r={type:s.type,released:s.released,tag:Object.prototype.toString.call(s)};await s.release();return r})",
 'ink':'navigator.ink.requestPresenter({presentationArea:document.body}).then(p=>({expectedImprovement:p.expectedImprovement,tag:Object.prototype.toString.call(p)}))',
 'managed':'navigator.managed.getManagedConfiguration([])',
 'virtualKeyboard':'({overlaysContent:navigator.virtualKeyboard.overlaysContent,rect:navigator.virtualKeyboard.boundingRect.toJSON()})',
 'protectedAudience':"navigator.protectedAudience.queryFeatureSupport('unknown')",
}

catalog=json.loads((Path(__file__).resolve().parents[1]/'chrome/152/generated/webapi.json').read_text())
permission_names=next(d['values'] for d in catalog['declarations'] if d['name']=='PermissionName')
OPERATIONS['permissions.all']="Promise.all("+json.dumps(permission_names)+".map(async name=>{try{return [name,(await navigator.permissions.query({name})).state]}catch(e){return [name,{name:e.name,message:e.message}]}})).then(Object.fromEntries)"
OPERATIONS={'permissions.initial':OPERATIONS['permissions.all'],**OPERATIONS}

async def observe(endpoint, url, operations=True, mode='headful', window_size='1280x800', oracle=True):
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    try:
        if oracle: await prepare_page(browser, page, mode, window_size)
        if url != 'about:blank': await page.goto(url)
        if oracle and mode == 'headful': await page.bringToFront()
        async def evaluate(target, function, *args):
            # Pyppeteer evaluate() silently sets userGesture=true. That changes
            # the very activation and capability semantics being measured.
            response=await target._client.send('Runtime.evaluate', {'expression':f'({function})(...{json.dumps(args)})', 'awaitPromise':True,'returnByValue':True,'userGesture':False})
            if 'exceptionDetails' in response: raise RuntimeError(response['exceptionDetails'])
            return response['result'].get('value')
        result = await evaluate(page, SHAPE, MEMBERS)
        result['operations'] = {}
        for name, expression in (OPERATIONS.items() if operations else []):
            operation_page = await browser.newPage() if oracle else page
            try:
                if oracle:
                    await browser._connection.send('Browser.resetPermissions')
                    await prepare_page(browser, operation_page, mode, window_size)
                if url != 'about:blank': await operation_page.goto(url)
                if oracle and mode == 'headful': await operation_page.bringToFront()
                result['operations'][name] = await evaluate(operation_page, r"""async (source)=>{
                  try {const value=await Promise.race([eval(source),new Promise(resolve=>setTimeout(()=>resolve({pending:true}),1500))]);
                    return {value:value===undefined?{undefined:true}:value};
                  }catch(e){return {error:{name:e.name,message:e.message}}}
                }""", expression)
            finally:
                if oracle: await operation_page.close()
        if oracle and mode == 'headful': await page.bringToFront()
        context = await evaluate(page, "() => ({secureContext:isSecureContext,crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus()})")
        metadata = None
        if oracle:
            metadata = await capture_metadata(
                endpoint, browser, page, mode=mode,
                environment_profile_id='temporary', context_states={'current': context})
        return result, context, metadata
    finally:
        await page.close()
        await browser.disconnect()

async def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--endpoint',default='http://127.0.0.1:9333')
    parser.add_argument('--mimic',action='store_true')
    parser.add_argument('--output',required=True)
    parser.add_argument('--write-probes',action='store_true')
    parser.add_argument('--browser-mode',choices=MODES,default='headful')
    parser.add_argument('--environment-profile-id',default='')
    parser.add_argument('--feature-override',action='append',default=[])
    parser.add_argument('--window-size',default='1280x800')
    args=parser.parse_args()
    if args.write_probes:
        Path(__file__).with_name('probes-navigator-capabilities.json').write_text(json.dumps([{'name':name,'expression':expression} for name,expression in OPERATIONS.items()],indent=2)+'\n',encoding='utf-8')
    observed=product(args.endpoint)
    if not args.mimic and observed!=PINNED_PRODUCT:
        raise RuntimeError(f'oracle drift: {observed}')
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200);self.send_header('Content-Type','text/html')
            if self.path.startswith('/isolated'):
                self.send_header('Cross-Origin-Opener-Policy','same-origin')
                self.send_header('Cross-Origin-Embedder-Policy','require-corp')
            self.end_headers()
            self.wfile.write(b'<!doctype html><title>Capability fixture</title><body>fixture</body>')
        def log_message(self,*args): pass
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    try:
        observations = {}
        contexts = {}
        metadata = None
        for name, url, operations in [
            ('secure', f'http://127.0.0.1:{server.server_port}/', True),
            ('isolated', f'http://127.0.0.1:{server.server_port}/isolated', False),
            ('opaque', 'about:blank', True),
        ]:
            observations[name], contexts[name], metadata = await observe(
                args.endpoint, url, operations, args.browser_mode, args.window_size, not args.mimic)
        profile_id = args.environment_profile_id or default_profile_id(args.browser_mode)
        data={'product':observed, **observations}
        if args.mimic:
            data['environmentSelection'] = {'browserMode': args.browser_mode,
              'environmentProfileId': profile_id}
            data['comparisonOracle'] = {'capture': 'navigator-chrome152.json',
              'browserMode': args.browser_mode}
        else:
            metadata['environmentProfileId'] = profile_id
            metadata['commandLineFeatureOverrides'] = sorted(set(
                metadata['commandLineFeatureOverrides'] + args.feature_override))
            metadata['secureContextState'] = {name:value['secureContext'] for name,value in contexts.items()}
            metadata['isolationState'] = {name:value['crossOriginIsolated'] for name,value in contexts.items()}
            data['captureMetadata'] = metadata
        Path(args.output).write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n',encoding='utf-8')
    finally: server.shutdown()

if __name__=='__main__': asyncio.run(main())
