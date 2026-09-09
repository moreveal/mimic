import asyncio,json,pathlib,urllib.request,hashlib,sys
import websockets
async def main():
 version=json.load(urllib.request.urlopen('http://127.0.0.1:9343/json/version'))
 async with websockets.connect(version['webSocketDebuggerUrl'],max_size=16*1024*1024) as ws:
  seq=0
  async def call(method,params={},session=None):
   nonlocal seq
   seq+=1;message={'id':seq,'method':method,'params':params}
   if session:message['sessionId']=session
   await ws.send(json.dumps(message))
   while True:
    response=json.loads(await ws.recv())
    if response.get('id')==seq:
     if 'error' in response:raise RuntimeError(response['error'])
     return response['result']
  context=(await call('Target.createBrowserContext'))['browserContextId']
  try:
   target=(await call('Target.createTarget',{'url':'about:blank','browserContextId':context}))['targetId']
   session=(await call('Target.attachToTarget',{'targetId':target,'flatten':True}))['sessionId']
   fixture=pathlib.Path(sys.argv[1] if len(sys.argv)>1 else 'internal/browser/testdata/webgl_capabilities_oracle.js')
   value=await call('Runtime.evaluate',{'expression':fixture.read_text(encoding='utf8'),'returnByValue':True},session)
   if 'exceptionDetails' in value:raise RuntimeError(value['exceptionDetails'])
   out={'browser':version['Browser'],'metadata':{'existingBrowser':True,'isolatedBrowserContext':True,'fixture':str(fixture).replace('\\','/'),'fixtureSHA256':hashlib.sha256(fixture.read_text(encoding='utf8').encode('utf8')).hexdigest(),'fixtureHashNormalization':'LF'},'result':value['result']['value']}
   pathlib.Path(sys.argv[2] if len(sys.argv)>2 else 'compatibility/captures/semantic-checkpoints/webgl-capabilities-chrome152.json').write_text(json.dumps(out,indent=2),encoding='utf8')
   print('Captured',version['Browser'],list(out['result']))
  finally:await call('Target.disposeBrowserContext',{'browserContextId':context})
asyncio.run(main())
