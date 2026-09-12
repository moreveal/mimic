"""Controlled Performance API captures against an explicitly launched browser.

No Chrome flags, profiles, existing tabs or expectations are silently modified.
Each fixture runs in a fresh document. Non-deterministic numbers stay in diagnostic
captures; test fixtures return semantic relations instead.
"""
import argparse, asyncio, hashlib, json, pathlib, sys, threading, time, urllib.parse, urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import pyppeteer

ROOT = pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'compatibility'))

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urllib.parse.urlsplit(self.path)
        query = urllib.parse.parse_qs(parsed.query)
        body = b'<!doctype html><link rel="icon" href="data:,"><title>Performance oracle</title><body></body>'
        mime = 'text/html'
        if parsed.path == '/resource':
            body, mime = b'performance resource', 'text/plain'
            if 'opaque' in query:mime='application/javascript'
        if parsed.path == '/worker.js':
            body, mime = b'onmessage=async e=>{try{postMessage({value:await (0,eval)(e.data)})}catch(e){postMessage({error:String(e)})}}', 'text/javascript'
        if parsed.path == '/lifecycle':body=(ROOT/'internal/browser/testdata/performance_navigation_document.html').read_bytes()
        if parsed.path == '/redirect':
            time.sleep(.01) # Keep redirect intervals above clock-coarsening noise.
            self.send_response(302); self.send_header('Location',query.get('next',['/resource'])[0]);self.send_header('Access-Control-Allow-Origin','*')
            if 'tao' in query:self.send_header('Timing-Allow-Origin',query['tao'][0])
            self.end_headers();return
        self.send_response(200)
        self.send_header('Content-Type',mime)
        self.send_header('Content-Length',str(len(body)))
        self.send_header('Cache-Control','no-store')
        self.send_header('Access-Control-Allow-Origin','*')
        if 'tao' in query:self.send_header('Timing-Allow-Origin',query['tao'][0])
        self.send_header('Server-Timing', 'db;dur=12.5;desc="database, primary", cache;desc="hit", dup;dur=1;dur=7;desc="first";desc="last", neg;dur=-2')
        self.end_headers();self.wfile.write(body)
    def log_message(self,*args): pass

async def run(args):
    version=json.load(urllib.request.urlopen(args.endpoint+'/json/version'))
    if not args.mimic and version['Browser']!='Chrome/152.0.7977.82':raise RuntimeError(version)
    browser=await pyppeteer.connect(browserURL=args.endpoint,defaultViewport=None)
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    origin=f'http://127.0.0.1:{server.server_port}'
    try:
        for fixture in args.fixtures:
            path=ROOT/'internal/browser/testdata'/f'{fixture}_oracle.js'
            source=path.read_text(encoding='utf8')
            page=await browser.newPage()
            try:
                await page.goto(origin,waitUntil='load')
                state=await page.evaluate('''() => ({viewport:{width:innerWidth,height:innerHeight,deviceScaleFactor:devicePixelRatio},window:{left:screenX,top:screenY,width:outerWidth,height:outerHeight,windowState:'normal'},secureContextState:isSecureContext,isolationState:crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus()})''')
                metadata={'schemaVersion':1,'chromeVersion':'152.0.7977.82','chromiumRevision':1669021,'chromiumCommit':'d04cdb24d67b081f6cf80200ffc5233f44b61109','v8Version':version.get('V8-Version','15.2.124.21'),'platform':'windows-x64','browserMode':'headful','commandLineFeatureOverrides':[],'environmentProfileId':'chrome-152-windows-x64-headful-controlled-v1','profileFreshness':'fresh-controlled',**state}
                if args.worker:
                    expression='new Promise((resolve,reject)=>{const w=new Worker("/worker.js");w.onmessage=e=>{w.terminate();e.data.error?reject(Error(e.data.error)):resolve(e.data.value)};w.onerror=e=>reject(Error(e.message));w.postMessage('+json.dumps(source)+')})'
                else:expression=source
                try:
                    value=await asyncio.wait_for(page.evaluate(expression,force_expr=True),45)
                    if args.input:
                        await page._client.send('Input.dispatchKeyEvent',{'type':'keyDown','key':'a','code':'KeyA','windowsVirtualKeyCode':65})
                        await page._client.send('Input.dispatchKeyEvent',{'type':'keyUp','key':'a','code':'KeyA','windowsVirtualKeyCode':65})
                        await asyncio.sleep(.3)
                        value=await page.evaluate('performanceInputResult()',force_expr=True)
                except Exception as e:value={'captureError':str(e)}
                out={'captureMetadata':metadata,'fixtureOrigin':origin,'fixture':str(path.relative_to(ROOT)).replace('\\','/'),'fixtureSHA256':hashlib.sha256(source.encode()).hexdigest(),'worker':args.worker,'result':{'result':{'type':'object','value':value}}}
                if args.mimic:out['captureMetadata']={'runtime':'Mimic','comparisonReference':'Chrome/152.0.7977.82'}
                target=pathlib.Path(args.output)/f'{fixture}{"_worker" if args.worker else ""}_{"mimic" if args.mimic else "chrome152"}.json'
                target.parent.mkdir(parents=True,exist_ok=True)
                target.write_text(json.dumps(out,ensure_ascii=False,indent=2)+'\n',encoding='utf8')
                print(target.name, 'ERROR '+value['captureError'] if isinstance(value,dict) and 'captureError' in value else 'captured',flush=True)
            finally:await page.close()
    finally:
        await browser.disconnect();server.shutdown();server.server_close()

if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('fixtures',nargs='+')
    parser.add_argument('--endpoint',default='http://127.0.0.1:9357')
    parser.add_argument('--output',default=str(ROOT/'internal/browser/testdata'))
    parser.add_argument('--worker',action='store_true');parser.add_argument('--mimic',action='store_true')
    parser.add_argument('--input',action='store_true')
    asyncio.run(run(parser.parse_args()))
