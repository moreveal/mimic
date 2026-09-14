"""Observational CDP site journeys; not a benchmark or a full application test.

Start a fresh process, navigate one Page, subscribe to preview, and check an
independent Page. Try an ordinary departure before using stopLoading to recover.
Store raw captures under .build: they may contain transient site cookies/tokens.
Requires aiohttp and psutil, like live_browser_comparison.py.

Example (use a freshly built executable and its actual source revision):
  python tools/compatibility/site_journey.py --binary .build/mimic.exe \
    --out .build/site-journey --binary-revision <commit> --preview --follow
"""
import argparse
import asyncio
import contextlib
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from datetime import datetime, timezone

import aiohttp
import psutil

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'tools/compatibility'))
from live_browser_comparison import CDP

SITES = {
    'homedepot': 'https://www.homedepot.com/',
    'lowes': 'https://www.lowes.com/',
    'macys': 'https://www.macys.com/',
    'royalalberthall': 'https://www.royalalberthall.com/',
    'flyscoot': 'https://www.flyscoot.com/',
}
PROBE = r'''(() => {
  let text='';
  const stack=document.body?[document.body]:[];
  while(stack.length && text.length<6000){
    const n=stack.pop();
    if(n.nodeType===3){text+=(n.nodeValue||'')+' ';continue;}
    if(n.nodeType!==1 || /^(SCRIPT|STYLE|NOSCRIPT|TEMPLATE)$/.test(n.nodeName))continue;
    const children=n.childNodes;
    for(let i=children.length-1;i>=0;i--)stack.push(children[i]);
  }
  const links=Array.from(document.querySelectorAll('a[href]')).slice(0,300)
    .map(a=>({url:a.href,text:(a.textContent||'').trim().slice(0,100)}));
  return {url:location.href,title:document.title,readyState:document.readyState,
    text:text.replace(/\s+/g,' ').trim().slice(0,6000),
    nodes:document.querySelectorAll('*').length,links};
})()'''

def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')

async def attempt(cdp, method, params=None, sid=None, timeout=4):
    start=time.perf_counter()
    try:
        result=await cdp.send(method, params, sid, timeout=timeout)
        return {'ms':round((time.perf_counter()-start)*1000,2),'result':result}
    except Exception as exc:
        return {'ms':round((time.perf_counter()-start)*1000,2),'error':repr(exc)}

async def preview_read(ws, folder, packets):
    async for msg in ws:
        if msg.type == aiohttp.WSMsgType.TEXT:
            packet=json.loads(msg.data)
            html=packet.pop('html','')
            if html:
                (folder/'preview.html').write_text(html,encoding='utf-8')
            packets.append({'time':time.monotonic(),'htmlBytes':len(html),**packet})

async def visit(cdp, sid, spare, url, folder, args, packets):
    folder.mkdir(exist_ok=True)
    if args.engine=='mimic':
        await attempt(cdp,'Mimic.clearTrace',sid=sid,timeout=3)
    start=time.monotonic()
    event_start=len(cdp.events)
    preview_start=len(packets)
    row={'url':url,'samples':[],'started_utc':datetime.now(timezone.utc).isoformat()}
    print('VISIT '+url,flush=True)
    nav=asyncio.create_task(attempt(cdp,'Page.navigate',{'url':url},sid,args.budget+5))
    while time.monotonic()-start < args.budget:
        await asyncio.sleep(2)
        health_methods=[attempt(cdp,'Browser.getVersion',timeout=2),
                        attempt(cdp,'Runtime.evaluate',{'expression':'6*7','returnByValue':True},spare,2)]
        if args.engine=='mimic':
            health_methods.append(attempt(cdp,'Mimic.getStatus',sid=sid,timeout=2))
        health=await asyncio.gather(*health_methods)
        sample={'elapsed':round(time.monotonic()-start,2),'health':health,
                'probe':await attempt(cdp,'Runtime.evaluate',{'expression':PROBE,'returnByValue':True},sid,args.probe_timeout)}
        row['samples'].append(sample)
        write(folder/'progress.json',row)
        value=sample['probe'].get('result',{}).get('result',{}).get('value',{})
        print(json.dumps({'elapsed':sample['elapsed'],'title':value.get('title'),'ready':value.get('readyState'),
                          'nodes':value.get('nodes'),'error':sample['probe'].get('error'),
                          'status':health[-1] if args.engine=='mimic' else None,'previews':len(packets)-preview_start}),flush=True)
        if sample['probe'].get('error') and len(row['samples'])>=2 and not args.keep_waiting:
            break
    if args.engine=='mimic':
        row['diagnostics']=await attempt(cdp,'Mimic.getDiagnostics',sid=sid,timeout=3)
        if args.trace:
            row['trace']=await attempt(cdp,'Mimic.getTrace',sid=sid,timeout=8)
    row['socket']={'closed':cdp.ws.closed,'closeCode':cdp.ws.close_code,'exception':repr(cdp.ws.exception())}
    row['events']=cdp.events[event_start:]
    row['previews']=packets[preview_start:]
    row['navigation']=await nav if nav.done() else {'pending':True}
    row['navigateAway']=await attempt(cdp,'Page.navigate',{'url':args.recovery_url},sid,args.departure_timeout)
    row['recoveryNavigate']=row['navigateAway']
    if row['navigateAway'].get('error') or row['navigateAway'].get('result',{}).get('errorText'):
        row['stop']=await attempt(cdp,'Page.stopLoading',sid=sid,timeout=4)
        row['recoveryNavigate']=await attempt(cdp,'Page.navigate',{'url':args.recovery_url},sid,8)
    row['recoveryEvaluate']=await attempt(cdp,'Runtime.evaluate',{'expression':'document.title','returnByValue':True},sid,4)
    row['recovered']=row['recoveryEvaluate'].get('result',{}).get('result',{}).get('value')=='Recovery'
    if not nav.done():
        nav.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await nav
    write(folder/'result.json',row)
    final=next((s['probe'].get('result',{}).get('result',{}).get('value') for s in reversed(row['samples']) if s['probe'].get('result',{}).get('result',{}).get('value')), {})
    summary={'url':url,'title':final.get('title'),'ready':final.get('readyState'),
             'text':final.get('text','')[:250],'nodes':final.get('nodes'),
             'previews':len(row['previews']),'navigateAway':row['navigateAway'],
             'recovered':row['recovered'],'recovery':row['recoveryEvaluate'],'navigation':row['navigation']}
    write(folder/'summary.json',summary)
    print('RESULT '+json.dumps(summary,ensure_ascii=False),flush=True)
    return row,final

async def run(args):
    class RecoveryHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            body=b'<!doctype html><title>Recovery</title><p id="recovered">alive</p>'
            self.send_response(200)
            self.send_header('Content-Type','text/html')
            self.send_header('Content-Length',str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        def log_message(self,*a): pass
    recovery=ThreadingHTTPServer(('127.0.0.1',0),RecoveryHandler)
    threading.Thread(target=recovery.serve_forever,daemon=True).start()
    args.recovery_url=f'http://127.0.0.1:{recovery.server_port}/recovery'
    out=args.out.resolve()
    out.mkdir(parents=True,exist_ok=False)
    binary=args.binary.resolve()
    manifest={'utc':datetime.now(timezone.utc).isoformat(),'binary':str(binary),
              'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),
              'declared_binary_revision':args.binary_revision,
              'workspace_head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),
              'harness_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
              'cdp_helper_sha256':hashlib.sha256((ROOT/'tools/compatibility/live_browser_comparison.py').read_bytes()).hexdigest(),
              'args':{k:str(v) for k,v in vars(args).items()}}
    write(out/'manifest.json',manifest)
    if args.engine=='mimic':
        # The checkout HEAD can differ from an older executable under test.
        try:
            build_info=subprocess.run(['go','version','-m',str(binary)],capture_output=True,text=True,timeout=10)
            manifest['binary_build_info']=build_info.stdout
        except (OSError,subprocess.TimeoutExpired) as exc:
            manifest['binary_build_info_error']=repr(exc)
        write(out/'manifest.json',manifest)
    log=(out/'process.log').open('wb')
    if args.engine=='mimic':
        cmd=[str(binary),'-listen',f'127.0.0.1:{args.port}']
        if args.preview: cmd+=['-dev-preview']
    else:
        cmd=[str(binary),'--headless=new',f'--remote-debugging-port={args.port}',
             f'--user-data-dir={out / "chrome-profile"}','--no-first-run','--no-default-browser-check','about:blank']
    proc=subprocess.Popen(cmd,cwd=ROOT,stdout=log,stderr=subprocess.STDOUT,
                          creationflags=getattr(subprocess,'CREATE_NO_WINDOW',0))
    try:
        async with aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=5)) as http:
            endpoint=f'http://127.0.0.1:{args.port}'
            version=None
            for _ in range(60):
                try:
                    async with http.get(endpoint+'/json/version') as r: version=await r.json()
                    break
                except Exception: await asyncio.sleep(.2)
            if version is None:
                raise RuntimeError('CDP startup failed; see process.log')
            manifest['version']=version
            write(out/'manifest.json',manifest)
            ws=await http.ws_connect(version['webSocketDebuggerUrl'],max_msg_size=128*1024*1024)
            cdp=CDP(ws)
            tids=[]
            sids=[]
            for _ in range(2):
                target=await cdp.send('Target.createTarget',{'url':'about:blank'},timeout=10)
                tids.append(target['targetId'])
                attach=await cdp.send('Target.attachToTarget',{'targetId':target['targetId'],'flatten':True},timeout=10)
                sids.append(attach['sessionId'])
            sid,spare=sids
            for method in ['Page.enable','Runtime.enable','Network.enable']:
                await cdp.send(method,session=sid,timeout=10)
            packets=[]
            preview_ws=None
            if args.preview and args.engine=='mimic':
                preview_ws=await http.ws_connect(endpoint.replace('http:','ws:')+'/debug/preview/ws?target='+tids[0],max_msg_size=128*1024*1024)
                reader=asyncio.create_task(preview_read(preview_ws,out,packets))
            for name in args.sites.split(','):
                url=args.url or SITES[name]
                row,final=await visit(cdp,sid,spare,url,out/name,args,packets)
                if args.follow and row['recovered']:
                    from urllib.parse import urlsplit
                    origin=urlsplit(final.get('url',url)).netloc
                    links=[l for l in final.get('links',[]) if urlsplit(l['url']).scheme in ('http','https') and urlsplit(l['url']).netloc==origin and urlsplit(l['url']).path not in ('','/',urlsplit(final.get('url',url)).path) and not any(x in l['url'].lower() for x in ('login','logout','signin','cart','checkout','account'))]
                    if links:
                        row,_=await visit(cdp,sid,spare,links[0]['url'],out/(name+'-follow'),args,packets)
                if not row['recovered']:
                    print('UNRECOVERED; ending this process',flush=True)
                    break
            if preview_ws:
                await preview_ws.close()
                reader.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await reader
            await ws.close()
            await cdp.reader
    finally:
        write(out/'process-status.json',{'exitBeforeCleanup':proc.poll(),
              'aliveBeforeCleanup':proc.poll() is None,'cleanup':'terminate owned diagnostic process tree'})
        with contextlib.suppress(psutil.Error):
            root=psutil.Process(proc.pid)
            for child in root.children(recursive=True):
                with contextlib.suppress(psutil.Error): child.kill()
            root.kill()
        with contextlib.suppress(Exception): proc.wait(timeout=5)
        log.close()
        recovery.shutdown()
        recovery.server_close()

if __name__=='__main__':
    # Live titles/body excerpts may contain characters outside a Windows codepage.
    sys.stdout.reconfigure(encoding='utf-8')
    p=argparse.ArgumentParser()
    p.add_argument('--binary',type=Path,required=True)
    p.add_argument('--binary-revision',help='Source revision used to build the executable; never inferred from checkout HEAD')
    p.add_argument('--out',type=Path,required=True)
    p.add_argument('--engine',choices=['mimic','chrome'],default='mimic')
    p.add_argument('--sites',default=','.join(SITES))
    p.add_argument('--url')
    p.add_argument('--budget',type=float,default=25)
    p.add_argument('--probe-timeout',type=float,default=5)
    p.add_argument('--departure-timeout',type=float,default=8,
                   help='Seconds to allow an ordinary navigation before stopLoading recovery')
    p.add_argument('--port',type=int,default=19451)
    p.add_argument('--preview',action='store_true')
    p.add_argument('--follow',action='store_true')
    p.add_argument('--keep-waiting',action='store_true')
    p.add_argument('--trace',action='store_true')
    args=p.parse_args()
    if any(name not in SITES for name in args.sites.split(',')):
        p.error('--sites must contain comma-separated names from '+','.join(SITES))
    if min(args.budget,args.probe_timeout,args.departure_timeout)<=0:
        p.error('budgets and timeouts must be positive')
    asyncio.run(run(args))
