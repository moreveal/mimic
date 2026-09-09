"""Independent module URL -> fetch -> DOM probe, with no external site code."""
import argparse
import asyncio
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from pyppeteer import connect
from oracle import PINNED_PRODUCT, product

RESOURCES = {
    '/': ('text/html', '''<!doctype html><body><div id="result">pending</div>
<script>globalThis.order=['classic'];globalThis.metaResult={};
document.addEventListener('DOMContentLoaded',()=>order.push('DOMContentLoaded'));</script>
<script type="module" src="/modules/entry.js?entry#fragment"></script>
<script type="module">metaResult.inlineURL=import.meta.url;</script>'''),
    '/modules/entry.js': ('text/javascript', '''import {meta} from './dependency.js?dep#part';
order.push('module');metaResult.url=import.meta.url;metaResult.dependency=meta.url;
metaResult.stable=import.meta===import.meta;metaResult.distinct=meta!==import.meta;
metaResult.nullPrototype=Object.getPrototypeOf(import.meta)===null;
const d=Object.getOwnPropertyDescriptor(import.meta,'url');
metaResult.descriptor=d?{writable:d.writable,enumerable:d.enumerable,configurable:d.configurable}:null;
queueMicrotask(()=>order.push('microtask'));
if(typeof import.meta.url==='string')fetch(new URL('data.json',import.meta.url)).then(r=>r.json()).then(data=>{
document.getElementById('result').textContent=data.value;order.push('fetch');
});else metaResult.failure='module URL is missing';
setTimeout(()=>import('./dynamic.js').then(m=>metaResult.dynamic=m.url),0);'''),
    '/modules/dependency.js': ('text/javascript', 'export const meta=import.meta;'),
    '/modules/dynamic.js': ('text/javascript', 'export const url=import.meta.url;'),
    '/modules/data.json': ('application/json', '{"value":"populated"}'),
}

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        typ, body = RESOURCES.get(self.path.split('?')[0], ('text/plain','missing'))
        self.send_response(200 if self.path.split('?')[0] in RESOURCES else 404)
        self.send_header('Content-Type',typ)
        self.end_headers()
        self.wfile.write(body.encode())
    def log_message(self,*args): pass

async def observe(endpoint,url,include_trace=False):
    browser=await connect(browserURL=endpoint,defaultViewport=None)
    page=await browser.newPage()
    errors=[]
    page.on('pageerror',lambda e:errors.append(str(e)))
    await page.goto(url,{'waitUntil':'load'})
    await asyncio.sleep(1)
    result=await page.evaluate("() => ({meta:metaResult,order,text:document.getElementById('result').textContent})")
    result['errors']=errors
    if include_trace:
        trace=await page._client.send('Mimic.getTrace')
        result['issues']=[e for e in trace['events'] if e['kind'] in ['error','exception']]
    await page.close();await browser.disconnect()
    return result

async def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--chrome',default='http://127.0.0.1:19323')
    parser.add_argument('--mimic',default='http://127.0.0.1:19322')
    parser.add_argument('--output',required=True)
    parser.add_argument('--port',type=int,default=49389)
    args=parser.parse_args()
    if product(args.chrome)!=PINNED_PRODUCT:
        raise RuntimeError('Chrome oracle version drift')
    server=ThreadingHTTPServer(('127.0.0.1',args.port),Handler)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    try:
        url=f'http://127.0.0.1:{server.server_port}/'
        result={name:await observe(endpoint,url,name=='mimic') for name,endpoint in [('chrome',args.chrome),('mimic',args.mimic)]}
        Path(args.output).write_text(json.dumps(result,indent=2),encoding='utf8')
        print(json.dumps(result,indent=2))
    finally: server.shutdown();server.server_close()

if __name__=='__main__': asyncio.run(main())
