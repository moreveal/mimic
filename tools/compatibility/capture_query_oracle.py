import asyncio,json,pathlib,urllib.request,hashlib,sys,base64
import websockets
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import threading
async def main():
 if '--secure-context' in sys.argv and '--local-echo' in sys.argv:raise ValueError("Choose one document source")
 if '--cookie-echo' in sys.argv and '--local-echo' not in sys.argv:raise ValueError("Cookie echo requires local echo")
 version=json.load(urllib.request.urlopen('http://127.0.0.1:9343/json/version'))
 async with websockets.connect(version['webSocketDebuggerUrl'],max_size=16*1024*1024) as ws:
  seq=0
  async def call(method,params={},session=None):
   nonlocal seq
   seq+=1;request_id=seq;message={'id':request_id,'method':method,'params':params}
   if session:message['sessionId']=session
   await ws.send(json.dumps(message))
   while True:
    response=json.loads(await ws.recv())
    if response.get('method')=='Fetch.requestPaused':
     seq+=1
     await ws.send(json.dumps({'id':seq,'sessionId':response['sessionId'],'method':'Fetch.fulfillRequest','params':{'requestId':response['params']['requestId'],'responseCode':200,'responseHeaders':[{'name':'Content-Type','value':'text/html'}],'body':base64.b64encode(b'<!doctype html><title>Local API oracle</title>').decode()}}))
    if response.get('id')==request_id:
     if 'error' in response:raise RuntimeError(response['error'])
     return response['result']
  server=None
  context=(await call('Target.createBrowserContext'))['browserContextId']
  try:
   target=(await call('Target.createTarget',{'url':'about:blank','browserContextId':context}))['targetId']
   session=(await call('Target.attachToTarget',{'targetId':target,'flatten':True}))['sessionId']
   if '--local-echo' in sys.argv:
    class Handler(BaseHTTPRequestHandler):
     def do_GET(self):
      selected={k.lower():v for k,v in self.headers.items() if k.lower() in ['accept-language','sec-fetch-storage-access','sec-fetch-site','sec-fetch-mode','sec-fetch-dest']+(['cookie'] if '--cookie-echo' in sys.argv else [])}
      body=json.dumps(selected).encode() if self.path.startswith('/echo') else b'<!doctype html><title>Local request oracle</title>'
      if self.path.startswith('/frame'):
       body=('<!doctype html><script>Promise.all([fetch("/echo").then(r=>r.json()),document.hasStorageAccess()]).then(([fetch,access])=>parent.postMessage({navigation:'+json.dumps(selected)+',fetch,access},"*"))</script>').encode()
      self.send_response(302 if self.path.startswith('/redirect') and '--cookie-echo' in sys.argv else 200)
      if '--cookie-echo' in sys.argv:
       if self.path.startswith('/set?name='):
        name=self.path.split('=',1)[1]
        if name.isalpha():self.send_header('Set-Cookie',name+'=1; Path=/')
       if self.path.startswith('/redirect'):
        self.send_header('Set-Cookie','redirected=1; Path=/');self.send_header('Location','/echo')
      self.send_header('Access-Control-Allow-Origin',self.headers.get('Origin','*'));self.send_header('Access-Control-Allow-Credentials','true');self.send_header('Content-Type','application/json' if self.path.startswith('/echo') else 'text/html');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
     def log_message(self,*args):pass
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
    await call('Page.navigate',{'url':f'http://127.0.0.1:{server.server_port}/'},session)
   if '--secure-context' in sys.argv:
    await call('Fetch.enable',{'patterns':[{'urlPattern':'http://mimic-test.localhost/*'}]},session)
    await call('Page.navigate',{'url':'http://mimic-test.localhost/'},session)
   fixture=pathlib.Path(sys.argv[1] if len(sys.argv)>1 else 'internal/browser/testdata/webgl_capabilities_oracle.js')
   value=await asyncio.wait_for(call('Runtime.evaluate',{'expression':fixture.read_text(encoding='utf8'),'returnByValue':True,'awaitPromise':True},session),30)
   if 'exceptionDetails' in value:raise RuntimeError(value['exceptionDetails'])
   out={'browser':version['Browser'],'metadata':{'existingBrowser':True,'isolatedBrowserContext':True,'syntheticSecureContext':'--secure-context' in sys.argv,'localHeaderEcho':'--local-echo' in sys.argv,'cookieEcho':'--cookie-echo' in sys.argv,'fixture':str(fixture).replace('\\','/'),'fixtureSHA256':hashlib.sha256(fixture.read_text(encoding='utf8').encode('utf8')).hexdigest(),'fixtureHashNormalization':'LF'},'result':value['result']['value']}
   pathlib.Path(sys.argv[2] if len(sys.argv)>2 else 'compatibility/captures/semantic-checkpoints/webgl-capabilities-chrome152.json').write_text(json.dumps(out,indent=2),encoding='utf8')
   print('Captured',version['Browser'],list(out['result']))
  finally:
   try:await call('Target.disposeBrowserContext',{'browserContextId':context})
   finally:
    if server:server.shutdown();server.server_close()
asyncio.run(main())
