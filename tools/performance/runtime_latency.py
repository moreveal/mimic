"""Local CDP latency diagnostic; complements the unchanged fast gate.

Usage: python tools/performance/runtime_latency.py --binary .build/mimic.exe --output .build/latency.json
Requires pyppeteer and psutil. Fixtures and assertions are identical for both binaries.
"""
import argparse
import asyncio
import hashlib
import http.server
import json
import os
from pathlib import Path
import socket
import statistics
import subprocess
import threading
import time
import urllib.request

import psutil
from pyppeteer import connect

HTML = '<!doctype html><html><body>' + ''.join(
    f'<section class="entry"><span>text {i}</span><a id="a{i}" class="link {"pick" if i % 20 == 0 else "other"}">link</a></section>'
    for i in range(600)
) + '</body></html>'
OPERATIONS = {
    'selector': ("() => document.querySelectorAll('section.entry > a.pick').length", 30),
    'live_collection': ("() => {const list=document.getElementsByClassName('pick');let n=0;for(let i=0;i<list.length;i++)if(list[i].parentNode.tagName==='SECTION')n++;return n}", 30),
    'traversal': ("() => {let n=0;for(const node of document.querySelectorAll('a')){if(node.parentNode.parentNode===document.body&&node.parentNode.childNodes[2]===undefined)n++}return n}", 600),
    'mutation': ("() => {const list=document.getElementsByClassName('temporary'),node=document.getElementById('a1');const before=list.length;node.classList.add('temporary');const during=list.length;node.classList.remove('temporary');return [before,during,list.length].join(',')}", '0,1,0'),
}

class Fixture(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body=HTML.encode(); self.send_response(200); self.send_header('Content-Type','text/html'); self.send_header('Content-Length',str(len(body))); self.end_headers(); self.wfile.write(body)
    def log_message(self,*args): pass

async def run(args):
    binary=args.binary.resolve(); output=args.output.resolve()
    if output.exists(): raise FileExistsError(output)
    with socket.socket() as sock: sock.bind(('127.0.0.1',0)); port=sock.getsockname()[1]
    endpoint=f'http://127.0.0.1:{port}'
    fixture=http.server.ThreadingHTTPServer(('127.0.0.1',0),Fixture)
    threading.Thread(target=fixture.serve_forever,daemon=True).start()
    result={'binary':str(binary),'sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'rows':[]}
    process=subprocess.Popen([str(binary),'-listen',f'127.0.0.1:{port}'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,creationflags=subprocess.CREATE_NO_WINDOW if os.name=='nt' else 0)
    browser=None
    try:
        deadline=time.perf_counter()+30
        while True:
            try:
                with urllib.request.urlopen(endpoint+'/json/version',timeout=1) as response: result['version']=json.load(response)
                break
            except OSError:
                if process.poll() is not None or time.perf_counter()>deadline: raise
                await asyncio.sleep(.02)
        start=time.perf_counter(); browser=await connect(browserURL=endpoint,defaultViewport=None);result['connect_ms']=(time.perf_counter()-start)*1000
        rss=lambda:psutil.Process(process.pid).memory_info().rss
        result['rss_before']=rss()
        for wave in range(3):
            start=time.perf_counter(); page=await browser.newPage(); result['rows'].append({'wave':wave,'operation':'new_page','ms':(time.perf_counter()-start)*1000})
            start=time.perf_counter(); await page.goto(f'http://127.0.0.1:{fixture.server_port}/',waitUntil='domcontentloaded'); result['rows'].append({'wave':wave,'operation':'navigate','ms':(time.perf_counter()-start)*1000})
            for operation,(expression,expected) in OPERATIONS.items():
                for iteration in range(6):
                    start=time.perf_counter(); value=await page.evaluate(expression); elapsed=(time.perf_counter()-start)*1000
                    if value!=expected: raise AssertionError((operation,value,expected))
                    result['rows'].append({'wave':wave,'operation':operation,'iteration':iteration,'warmup':iteration==0,'ms':elapsed,'value':value})
            result.setdefault('rss_live',[]).append(rss()); await page.close(); await asyncio.sleep(.25);result.setdefault('rss_recovered',[]).append(rss())
        result['median_ms']={op:statistics.median(r['ms'] for r in result['rows'] if r['operation']==op and not r.get('warmup',False)) for op in ['new_page','navigate',*OPERATIONS]}
        output.parent.mkdir(parents=True,exist_ok=True);output.write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result['median_ms'],indent=2))
    finally:
        if browser: await browser.disconnect()
        process.terminate();process.wait(timeout=10);fixture.shutdown();fixture.server_close()

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--binary',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    asyncio.run(run(parser.parse_args()))
