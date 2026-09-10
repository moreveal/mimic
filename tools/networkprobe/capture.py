"""Capture the pinned headful Chrome's loopback TLS/H3 NetLog.

Build tools/networkprobe first. A temporary loopback certificate is added to
CurrentUser Root for the capture and removed in the finally block.
"""
import argparse
import hashlib
import asyncio
import json
import pathlib
import subprocess
import threading
import urllib.request
import websockets

async def main():
    p = argparse.ArgumentParser()
    p.add_argument('--chrome', required=True)
    p.add_argument('--server', required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--port', type=int, default=19453)
    p.add_argument('--cdp-port', type=int, default=19454)
    p.add_argument('--force-quic', action='store_true', help='allow loopback H3 with a locally trusted root')
    p.add_argument('--raw-h3', action='store_true')
    p.add_argument('--mimic', action='store_true')
    p.add_argument('--tcp-only', action='store_true')
    args = p.parse_args()
    out = pathlib.Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=False)
    cert = out / 'loopback.cer'
    server = subprocess.Popen([args.server, '-port', str(args.port), '-cert', str(cert), *(['-raw-h3'] if args.raw_h3 else []), *(['-tcp-only'] if args.tcp_only else [])], stdout=subprocess.PIPE, text=True)
    chrome = None
    thumbprint = None
    records = []
    reader = None
    try:
        ready = json.loads(server.stdout.readline())
        reader = threading.Thread(target=lambda: records.extend(server.stdout), daemon=True)
        reader.start()
        thumbprint = hashlib.sha1(cert.read_bytes()).hexdigest()
        subprocess.run(['certutil', '-user', '-addstore', 'Root', str(cert)], check=True, stdout=subprocess.DEVNULL)
        flags = [f'--user-data-dir={out / "profile"}', f'--remote-debugging-port={args.cdp_port}',
                 '--no-first-run', '--no-default-browser-check', '--window-size=1280,800',
                 f'--log-net-log={out / "netlog.json"}', '--net-log-capture-mode=Everything']
        if args.force_quic:
            flags.append(f'--origin-to-force-quic-on=localhost:{args.port}')
        chrome = subprocess.Popen([args.chrome, *flags, 'about:blank'])
        for _ in range(100):
            try:
                with urllib.request.urlopen(f'http://127.0.0.1:{args.cdp_port}/json/version', timeout=1) as r:
                    version = json.load(r)
                break
            except OSError:
                await asyncio.sleep(.1)
        else:
            raise RuntimeError('Chrome CDP did not start')
        if version['Browser'] != 'Chrome/152.0.7977.82':
            raise RuntimeError(f'Wrong reference: {version["Browser"]}')
        async with websockets.connect(version['webSocketDebuggerUrl']) as ws:
            async def rpc(i,method,params=None,session=None):
                msg={'id':i,'method':method,'params':params or {}}
                if session:msg['sessionId']=session
                await ws.send(json.dumps(msg))
                while True:
                    response=json.loads(await ws.recv())
                    if response.get('id')==i:
                        if 'error' in response:raise RuntimeError(response['error'])
                        return response['result']
            target=await rpc(1,'Target.createTarget',{'url':ready['url']})
            attached=await rpc(2,'Target.attachToTarget',{'targetId':target['targetId'],'flatten':True})
            await asyncio.sleep(1)
            observed=await rpc(3,'Runtime.evaluate',{'expression':'({viewport:{width:innerWidth,height:innerHeight,deviceScaleFactor:devicePixelRatio},window:{width:outerWidth,height:outerHeight},secureContextState:isSecureContext,isolationState:crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus(),origin:location.origin})','returnByValue':True},attached['sessionId'])
            browser_version=await rpc(4,'Browser.getVersion')
        metadata={'version':version,'browserVersion':browser_version,'flags':flags,'origin':ready['url'],'browserMode':'headful','observations':observed['result']['value'],'chromeSHA256':hashlib.sha256(pathlib.Path(args.chrome).read_bytes()).hexdigest(),'probeSHA256':hashlib.sha256(pathlib.Path(args.server).read_bytes()).hexdigest()}
        (out / 'metadata.json').write_text(json.dumps(metadata,indent=2))
        await asyncio.sleep(15)
        async with websockets.connect(version['webSocketDebuggerUrl']) as ws:
            await ws.send(json.dumps({'id':1, 'method':'Browser.close'}))
        chrome.wait(timeout=10)
        if args.mimic:
            await asyncio.sleep(.2)
            boundary=len(records)
            result=subprocess.run([args.server,'-client-origin',ready['url'],'-root',str(cert)],capture_output=True,text=True,timeout=30)
            (out/'mimic-client.json').write_text(result.stdout+result.stderr)
            await asyncio.sleep(.2)
            (out/'mimic-wire.jsonl').write_text(''.join(records[boundary:]))
            (out/'chrome-wire.jsonl').write_text(''.join(records[:boundary]))
            result.check_returncode()
    finally:
        if chrome is not None and chrome.poll() is None:
            chrome.terminate()
        server.terminate()
        server.wait(timeout=10)
        if reader:
            reader.join(timeout=10)
        (out / 'requests.jsonl').write_text(''.join(records))
        if thumbprint:
            subprocess.run(['certutil', '-user', '-delstore', 'Root', thumbprint], check=True, stdout=subprocess.DEVNULL)

if __name__ == '__main__':
    asyncio.run(main())
