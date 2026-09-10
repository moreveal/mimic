"""Record one disposable context in an existing Chrome CDP browser.

Requires websockets. This tool never launches/closes the browser or attaches to
existing tabs. Captures contain session credentials and must remain private.
"""
import argparse
import asyncio, base64, hashlib, json, pathlib, time, urllib.request
import websockets
OUT = PORT = URL = events = None
pending = {}
tasks = set()
sessions = {}
scripts = []
bodies = []
errors = []
counter = 0
main_session = None
main_target = None
main_ready = None
capture_active = True
revisit = False
preload_source = None

def save(name, obj):
    (OUT / name).write_text(json.dumps(obj, ensure_ascii=False, indent=2), encoding='utf8')

def spawn(coro):
    t = asyncio.create_task(coro)
    tasks.add(t)
    def done(task):
        tasks.discard(task)
        if not task.cancelled() and task.exception() is not None:
            errors.append({'captureTask': str(task.exception())})
    t.add_done_callback(done)

async def call(method, params=None, session=None):
    global counter
    counter += 1
    n = counter
    f = asyncio.get_running_loop().create_future()
    pending[n] = f
    message = {'id': n, 'method': method, 'params': params or {}}
    if session:
        message['sessionId'] = session
    try:
        await ws.send(json.dumps(message))
        response = await asyncio.wait_for(f, 15)
        if 'error' in response:
            errors.append({'method': method, 'session': session, 'error': response['error']})
        return response
    except Exception as e:
        errors.append({'method': method, 'session': session, 'error': str(e)})
        return {}
    finally:
        pending.pop(n, None)

async def initialize(p):
    global main_session
    sid = p['sessionId']
    info = p['targetInfo']
    sessions[sid] = info
    is_main = info['targetId'] == main_target
    if is_main:
        main_session = sid
    resume_attempted = False
    try:
        for method, args in [('Network.enable', {'maxTotalBufferSize': 200000000, 'maxResourceBufferSize': 50000000, 'maxPostDataSize': 50000000, 'enableDurableMessages': True}), ('Runtime.enable', {}), ('Debugger.enable', {'maxScriptsCacheSize': 100000000}), ('Target.setAutoAttach', {'autoAttach': True, 'waitForDebuggerOnStart': True, 'flatten': True})]:
            response = await call(method, args, sid)
            if 'result' not in response:
                raise RuntimeError(f'{method} failed for {sid}')
        if preload_source is not None and info['type'] in ('page', 'iframe'):
            response = await call('Page.enable', {}, sid)
            if 'result' not in response:
                raise RuntimeError(f'Page.enable failed for {sid}')
            response = await call('Page.addScriptToEvaluateOnNewDocument', {'source': preload_source, 'runImmediately': True}, sid)
            if 'result' not in response:
                raise RuntimeError(f'Preload failed for {sid}')
        resume_attempted = True
        response = await call('Runtime.runIfWaitingForDebugger', {}, sid)
        if 'result' not in response:
            raise RuntimeError(f'Failed to resume {sid}')
        if is_main and not main_ready.done():
            main_ready.set_result(sid)
    except Exception as error:
        if not resume_attempted:
            await call('Runtime.runIfWaitingForDebugger', {}, sid)
        if is_main and not main_ready.done():
            main_ready.set_exception(error)
        raise

async def body(sid, p):
    r = await call('Network.getResponseBody', {'requestId': p['requestId']}, sid)
    name = 'body-' + hashlib.sha256((sid + ':' + p['requestId']).encode()).hexdigest()[:20] + '.json'
    save(name, r)
    bodies.append({'session': sid, 'requestId': p['requestId'], 'file': name})

async def script(sid, p):
    r = await call('Debugger.getScriptSource', {'scriptId': p['scriptId']}, sid)
    # Inspector script ids can be reused after navigation in the same target.
    # Keep the original challenge sources when the destination parses new code.
    identity = ':'.join(str(v) for v in (sid, p['scriptId'], p.get('executionContextId'), p.get('hash')))
    name = 'script-' + hashlib.sha256(identity.encode()).hexdigest()[:20] + '.json'
    save(name, r)
    scripts.append({'session': sid, 'scriptId': p['scriptId'], 'executionContextId': p.get('executionContextId'), 'url': p.get('url'), 'hash': p.get('hash'), 'file': name})

async def post(sid, p):
    r = await call('Network.getRequestPostData', {'requestId': p['requestId']}, sid)
    save('post-' + hashlib.sha256((sid + ':' + p['requestId']).encode()).hexdigest()[:20] + '.json', r)

async def reader():
    async for raw in ws:
        m = json.loads(raw)
        if 'id' in m:
            f = pending.get(m['id'])
            if f and (not f.done()):
                f.set_result(m)
            continue
        m['observedMonotonic'] = time.monotonic()
        events.write(json.dumps(m, ensure_ascii=False) + '\n')
        events.flush()
        method = m.get('method')
        p = m.get('params', {})
        sid = m.get('sessionId')
        if not capture_active:
            continue
        if method == 'Target.attachedToTarget':
            spawn(initialize(p))
        elif method == 'Network.loadingFinished':
            spawn(body(sid, p))
        elif method == 'Debugger.scriptParsed':
            spawn(script(sid, p))
        elif method == 'Network.requestWillBeSent' and p['request'].get('hasPostData'):
            spawn(post(sid, p))

async def drain_tasks(timeout=15):
    deadline = asyncio.get_running_loop().time() + timeout
    while True:
        # Completion callbacks and event dispatch can spawn more work.
        await asyncio.sleep(0)
        if not tasks:
            return
        remaining = deadline - asyncio.get_running_loop().time()
        if remaining <= 0:
            unfinished = list(tasks)
            errors.append({'captureTasksIncomplete': len(unfinished)})
            for task in unfinished:
                task.cancel()
            await asyncio.gather(*unfinished, return_exceptions=True)
            return
        await asyncio.wait(list(tasks), timeout=remaining,
                           return_when=asyncio.FIRST_COMPLETED)

async def main():
    global ws, main_ready, main_target, capture_active
    endpoint, version, nav = {}, {}, {}
    command = {'note': 'Existing browser process; no launch or existing-tab/storage inspection'}
    try:
        endpoint = json.load(urllib.request.urlopen(f'http://127.0.0.1:{PORT}/json/version', timeout=10))
        async with websockets.connect(endpoint['webSocketDebuggerUrl'], max_size=200000000) as ws:
            main_ready = asyncio.get_running_loop().create_future()
            read = asyncio.create_task(reader())
            owned = None
            try:
                version = await call('Browser.getVersion')
                owned = (await call('Target.createBrowserContext', {'disposeOnDetach': True}))['result']['browserContextId']
                main_target = (await call('Target.createTarget', {'url': 'about:blank', 'browserContextId': owned}))['result']['targetId']
                attached = await call('Target.attachToTarget', {'targetId': main_target, 'flatten': True})
                if 'result' not in attached:
                    raise RuntimeError('Could not attach owned target')
                await asyncio.wait_for(main_ready, 60)
                nav = await call('Page.navigate', {'url': URL}, main_session)
                checkpoints = []
                for seconds in (10, 20, 30):
                    await asyncio.sleep(10)
                    state = await call('Runtime.evaluate', {'expression': 'JSON.stringify({url:location.href,title:document.title,ready:document.readyState,text:document.body?.innerText,html:document.documentElement?.outerHTML})', 'returnByValue': True}, main_session)
                    save(f'state-{seconds}.json', state)
                    checkpoints.append(state)
                if revisit:
                    save('cookies-before-revisit.json', await call('Storage.getCookies', {'browserContextId': owned}))
                    save('revisit-navigation.json', await call('Page.navigate', {'url': URL}, main_session))
                    await asyncio.sleep(10)
                    save('state-after-revisit.json', await call('Runtime.evaluate', {'expression': 'JSON.stringify({url:location.href,title:document.title,ready:document.readyState,text:document.body?.innerText})', 'returnByValue': True}, main_session))
                for sid, info in list(sessions.items()):
                    if info['type'] in ('page', 'iframe'):
                        save('dom-' + sid + '.json', await call('DOMSnapshot.captureSnapshot', {'computedStyles': [], 'includeDOMRects': True, 'includePaintOrder': True}, sid))
                        save('storage-' + sid + '.json', await call('Runtime.evaluate', {'expression': 'JSON.stringify({localStorage:Object.assign({},localStorage),sessionStorage:Object.assign({},sessionStorage)})', 'returnByValue': True}, sid))
                screenshot = await call('Page.captureScreenshot', {'format': 'png', 'captureBeyondViewport': True}, main_session)
                if screenshot.get('result', {}).get('data'):
                    (OUT / 'screenshot.png').write_bytes(base64.b64decode(screenshot['result']['data']))
                save('cookies.json', await call('Storage.getCookies', {'browserContextId': owned}))
            finally:
                try:
                    try:
                        await drain_tasks()
                    finally:
                        capture_active = False
                        if owned is not None:
                            await call('Target.disposeBrowserContext', {'browserContextId': owned})
                finally:
                    capture_active = False
                    read.cancel()
                    await asyncio.gather(read, return_exceptions=True)
                    for task in list(tasks):
                        task.cancel()
                    if tasks:
                        await asyncio.gather(*list(tasks), return_exceptions=True)
    except Exception as error:
        errors.append({'capture': str(error)})
    finally:
        events.close()
        save('manifest.json', {
            'url': URL, 'version': version, 'commandLine': command,
            'endpoint': endpoint, 'sessions': sessions, 'scripts': scripts,
            'bodies': bodies, 'errors': errors, 'navigation': nav,
            'revisitSameURL': revisit,
            'preloadSHA256': hashlib.sha256(preload_source.encode()).hexdigest() if preload_source is not None else None,
            'limitations': ['Browser-wide Tracing omitted to avoid recording unrelated targets'],
            'files': [{'name': p.name, 'size': p.stat().st_size,
                       'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
                      for p in OUT.iterdir() if p.is_file()]})

def cli():
    global OUT, PORT, URL, events, revisit, preload_source
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=pathlib.Path, help='New private output directory')
    parser.add_argument('port', type=int, help='Existing local Chrome CDP port')
    parser.add_argument('url', help='Explicit URL to capture')
    parser.add_argument('--revisit', action='store_true', help='After 30 seconds, navigate to the same URL by GET with the same context cookies')
    parser.add_argument('--preload', type=pathlib.Path, help='Optional diagnostic JS, applied only in owned page/frame targets; changes execution and requires an uninstrumented control')
    args = parser.parse_args()
    OUT, PORT, URL = args.output, args.port, args.url
    revisit = args.revisit
    if OUT.exists() and any(OUT.iterdir()):
        parser.error('Output directory must be empty; existing captures are never overwritten')
    OUT.mkdir(parents=True, exist_ok=True)
    if args.preload:
        preload_source = args.preload.read_text(encoding='utf8')
        (OUT / 'diagnostic-preload.js').write_text(preload_source, encoding='utf8')
    events = (OUT / 'events.jsonl').open('w', encoding='utf8')
    asyncio.run(main())

if __name__ == '__main__':
    cli()
