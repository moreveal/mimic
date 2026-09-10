"""Run the shared image preload regression in pinned headful Chrome on loopback."""
import argparse
import asyncio
import json
import hashlib
import pathlib
import subprocess
import threading
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import websockets

CASES = [dict(name='inflight'), dict(name='completed', completed=True),
         dict(name='anonymous', linkCrossOrigin='', imageCrossOrigin='anonymous'),
         dict(name='mismatch', imageCrossOrigin='anonymous'),
         dict(name='credentials-mismatch', linkCrossOrigin='anonymous', imageCrossOrigin='use-credentials'),
         dict(name='destination-mismatch', preloadAs='fetch'),
         dict(name='script', destination='script'),
         dict(name='style', destination='style'),
         dict(name='fetch', destination='fetch', linkCrossOrigin='anonymous'),
         dict(name='fetch-mismatch', destination='fetch'),
         dict(name='xhr', destination='xhr', preloadAs='fetch', linkCrossOrigin='anonymous'),
         dict(name='script-completed', destination='script', completed=True),
         dict(name='style-completed', destination='style', completed=True),
         dict(name='fetch-completed', destination='fetch', completed=True, linkCrossOrigin='anonymous'),
         dict(name='image-status404', fail=True), dict(name='image-status404-completed', fail=True, completed=True),
         dict(name='image-invalid', invalid=True), dict(name='image-invalid-completed', invalid=True, completed=True)]

async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--chrome', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    out = pathlib.Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=False)
    fixture = pathlib.Path('internal/browser/testdata/image_preload.js').read_text()
    states = {}
    lock = threading.Lock()
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_): pass
        def do_GET(self):
            u = urllib.parse.urlsplit(self.path)
            q = urllib.parse.parse_qs(u.query)
            name = q.get('case', [''])[0]
            with lock:
                state = states.setdefault(name, dict(count=0, started=threading.Event(), release=threading.Event()))
                if u.path == '/ci': state['count'] += 1
            body = b'<body></body>'
            content_type = 'text/html'
            if u.path == '/ci':
                state['started'].set()
                if q.get('completed') != ['true'] and not state['release'].wait(10):
                    self.send_error(504); return
                content_type = 'image/svg+xml'
                body = b'<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>'
                if q.get('as') == ['script']:
                    content_type, body = 'text/javascript', b'globalThis.preloadedScriptExecuted = true;'
                elif q.get('as') == ['style']:
                    content_type, body = 'text/css', b'body { color: rgb(1, 2, 3); }'
                elif q.get('as') == ['fetch']:
                    content_type, body = 'text/plain', b'preloaded fetch'
                if q.get('invalid') == ['true']: body = b'not an image'
            elif u.path == '/started':
                if not state['started'].wait(10): self.send_error(504); return
            elif u.path == '/release': state['release'].set()
            self.send_response(404 if u.path == '/ci' and q.get('fail') == ['true'] else 200)
            self.send_header('Content-Type', content_type)
            self.send_header('Cache-Control', 'no-store')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    chrome = subprocess.Popen([args.chrome, f'--user-data-dir={out / "profile"}',
                               '--remote-debugging-port=19464', '--no-first-run',
                               '--no-default-browser-check', 'about:blank'],
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        for _ in range(100):
            try:
                with urllib.request.urlopen('http://127.0.0.1:19464/json/version', timeout=1) as response:
                    version = json.load(response)
                break
            except OSError: await asyncio.sleep(.1)
        else: raise RuntimeError('CDP did not start')
        if version['Browser'] != 'Chrome/152.0.7977.82': raise RuntimeError(version)
        async with websockets.connect(version['webSocketDebuggerUrl']) as ws:
            counter = 0
            async def rpc(method, params=None, session=None):
                nonlocal counter
                counter += 1
                message = dict(id=counter, method=method, params=params or {})
                if session: message['sessionId'] = session
                await ws.send(json.dumps(message))
                while True:
                    reply = json.loads(await ws.recv())
                    if reply.get('id') == counter:
                        if 'error' in reply: raise RuntimeError(reply)
                        return reply['result']
            results = []
            for cycle in range(3):
                tab = await rpc('Target.createTarget', dict(url=f'http://127.0.0.1:{server.server_port}/'))
                session = (await rpc('Target.attachToTarget', dict(targetId=tab['targetId'], flatten=True)))['sessionId']
                for _ in range(100):
                    ready = await rpc('Runtime.evaluate', dict(expression='location.protocol==="http:" && document.readyState==="complete"', returnByValue=True), session)
                    if ready['result'].get('value'): break
                    await asyncio.sleep(.05)
                await rpc('Runtime.evaluate', dict(expression=fixture), session)
                for case in CASES:
                    options = dict(case, name=f'{case["name"]}-{cycle}')
                    value = await rpc('Runtime.evaluate', dict(expression='runImagePreloadCase('+json.dumps(options)+')', awaitPromise=True, returnByValue=True), session)
                    if 'exceptionDetails' in value: raise RuntimeError(value)
                    results.append(dict(options=options, result=value['result'].get('value'), requests=states[options['name']]['count']))
                await rpc('Target.closeTarget', dict(targetId=tab['targetId']))
            (out/'result.json').write_text(json.dumps(dict(browser=version['Browser'], chromeSHA256=hashlib.sha256(pathlib.Path(args.chrome).read_bytes()).hexdigest(), fixtureSHA256=hashlib.sha256(fixture.encode()).hexdigest(), mode='headful', cacheControl='no-store', cycles=results), indent=2)+'\n')
            print(json.dumps(results, indent=2))
            await rpc('Browser.close')
    finally:
        if chrome.poll() is None:
            chrome.terminate()
        chrome.wait(timeout=10)
        server.shutdown()
        server.server_close()

if __name__ == '__main__': asyncio.run(main())
