"""Run a shared, site-independent corpus against frozen Chrome and Mimic.

Only newly created pages are used. Chrome gets a disposable browser context;
Mimic must be a dedicated test instance (its CDP has no context isolation).
The loopback HTTP server records received requests, independently of CDP.
"""
import argparse
import asyncio
import hashlib
import json
import pathlib
import threading
import time
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import websockets

ROOT = pathlib.Path(__file__).resolve().parents[2]


def differences(a, b, path=""):
    """Keep absent, null, false, numbers, and array order distinct."""
    numbers = type(a) in (int, float) and type(b) in (int, float)
    if type(a) is not type(b) and not numbers:
        return [{"path": path or "/", "chrome": a, "mimic": b}]
    if isinstance(a, dict):
        rows = []
        for key in sorted(a.keys() | b.keys()):
            child = path + "/" + key.replace("~", "~0").replace("/", "~1")
            if key not in a or key not in b:
                rows.append({"path": child, "chrome": a.get(key), "mimic": b.get(key),
                             "missing": "chrome" if key not in a else "mimic"})
            else:
                rows.extend(differences(a[key], b[key], child))
        return rows
    if isinstance(a, list) and len(a) == len(b):
        return [row for i, (x, y) in enumerate(zip(a, b))
                for row in differences(x, y, path + "/" + str(i))]
    return [] if a == b else [{"path": path or "/", "chrome": a, "mimic": b}]


def cookie_metadata(rows):
    """Preserve effective attributes; exclude clock-dependent expiry timestamps."""
    fields = ('name', 'value', 'domain', 'path', 'secure', 'httpOnly',
              'sameSite', 'session', 'partitionKey', 'partitionKeyOpaque')
    values = [{k: row[k] for k in fields if k in row} for row in rows]
    return sorted(values, key=lambda row: json.dumps(row, sort_keys=True))


def probe_errors(result):
    return [i for i, row in enumerate(result.get('observations', [])) if 'exception' in row]


def network_sequence(records):
    """Serial fixture requests only; connection IDs become first-use ordinals."""
    connections, rows = {}, []
    for record in records:
        if not record['path'].startswith(('/echo', '/redirect/')):
            continue
        connection = connections.setdefault(record['connection'], len(connections) + 1)
        rows.append({k: record[k] for k in ('path', 'method', 'body', 'protocol')} | {'connection': connection})
    return rows


class CDP:
    async def connect(self, base):
        self.version = json.load(urllib.request.urlopen(base + "/json/version", timeout=10))
        self.ws = await websockets.connect(self.version['webSocketDebuggerUrl'], max_size=32 << 20)
        self.seq, self.events, self.pending = 0, [], {}
        self.reader = asyncio.create_task(self.read())
        return self

    async def read(self):
        async for raw in self.ws:
            event = json.loads(raw)
            if 'id' in event:
                future = self.pending.get(event['id'])
                if future and not future.done():
                    future.set_result(event)
            else:
                self.events.append(event)

    async def call(self, method, params=None, session=None):
        self.seq += 1
        seq = self.seq
        future = asyncio.get_running_loop().create_future()
        self.pending[seq] = future
        message = dict(id=seq, method=method, params=params or {})
        if session:
            message['sessionId'] = session
        try:
            await self.ws.send(json.dumps(message))
            response = await asyncio.wait_for(future, 40)
            if 'error' in response:
                raise RuntimeError(f"{method}: {response['error']}")
            return response['result']
        finally:
            self.pending.pop(seq, None)

    async def close(self):
        await self.ws.close()
        await asyncio.gather(self.reader, return_exceptions=True)


class FixtureServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self):
        super().__init__(('127.0.0.1', 0), Handler)
        self.records, self.connections, self.lock = [], {}, threading.Lock()
        self.label = "setup"


class Handler(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'

    def log_message(self, *args):
        pass

    def do_POST(self):
        self.do_GET()

    def do_GET(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        path = urllib.parse.urlsplit(self.path)
        query = urllib.parse.parse_qs(path.query)
        with self.server.lock:
            key = self.client_address
            connection = self.server.connections.setdefault(key, len(self.server.connections) + 1)
            received = dict(label=self.server.label, path=self.path, method=self.command,
                            headers=list(self.headers.raw_items()), body=body.decode('utf8', errors='replace'),
                            protocol=self.request_version, connection=connection, observed=time.monotonic())
            self.server.records.append(received)
        status, headers = 200, {}
        content_type = 'text/html; charset=utf-8'
        if path.path == '/echo':
            if query.get('delay'):
                time.sleep(min(1.0, max(0.0, float(query['delay'][0]))))
            # Exclude transport-dependent header ordering from the semantic value;
            # the independent raw record above retains it in full.
            content_type = 'application/json'
            payload = json.dumps(dict(method=self.command, body=received['body'],
                                      headers={k.lower(): v for k, v in self.headers.items()},
                                      protocol=self.request_version)).encode()
        elif path.path.startswith('/redirect/'):
            status = int(path.path.rsplit('/', 1)[1])
            headers['Location'] = '/echo'
            payload = b''
        elif path.path == '/frame':
            code = query.get('code', ['return null'])[0]
            ident = query.get('id', ['frame'])[0]
            script = ('Promise.resolve().then(async()=>{' + code + '}).then(value=>'
                      'parent.postMessage({id:' + json.dumps(ident) + ',value},"*"),'
                      'e=>parent.postMessage({id:' + json.dumps(ident) + ',error:{name:e.name,message:e.message}},"*"));')
            payload = ('<!doctype html><body><script>' + script.replace('</script', '<\\/script') + '</script>').encode()
        elif path.path == '/set':
            headers['Set-Cookie'] = query.get('cookie', [''])[0]
            payload = b'ok'
        elif path.path == '/cookie-redirect':
            status = int(query.get('status', ['302'])[0])
            headers['Location'] = query.get('to', ['/navigation'])[0]
            payload = b''
        elif path.path == '/navigation':
            value = dict(method=self.command, cookie=self.headers.get('Cookie', ''),
                         origin=self.headers.get('Origin', ''), site=self.headers.get('Sec-Fetch-Site', ''))
            if 'set' in query:
                headers['Set-Cookie'] = query['set'][0]
            payload = ('<!doctype html><body><script>globalThis.receivedNavigation=' +
                       json.dumps(value).replace('<', '\\u003c') + '</script>').encode()
        else:
            payload = b'<!doctype html><html><head><title>Compatibility fixture</title></head><body></body></html>'
        self.send_response(status)
        self.send_header('Content-Type', content_type)
        self.send_header('Content-Length', str(len(payload)))
        self.send_header('Cache-Control', 'no-store')
        for name, value in headers.items():
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(payload)


async def run_case(client, native, case, server):
    context, target = None, None
    results = []
    outcome = {'observations': results}
    try:
        params = {'url': 'about:blank'}
        if native:
            context = (await client.call('Target.createBrowserContext'))['browserContextId']
            params['browserContextId'] = context
        target = (await client.call('Target.createTarget', params))['targetId']
        session = (await client.call('Target.attachToTarget', {'targetId': target, 'flatten': True}))['sessionId']
        await client.call('Runtime.enable', session=session)
        if not native:
            # Cookie scope outlives a target in Mimic. The CLI explicitly requires
            # a dedicated process so this cannot erase a user's browsing session.
            await client.call('Network.clearBrowserCookies', session=session)
            await client.call('Network.clearBrowserCache', session=session)
        for step in case.get('steps', [case]):
            host = step.get('host', '127.0.0.1')
            url = f'http://{host}:{server.server_port}/'
            await client.call('Page.navigate', {'url': url}, session)
            for _ in range(100):
                ready = await client.call('Runtime.evaluate', {'expression': 'location.href===' + json.dumps(url) + '&&document.readyState==="complete"', 'returnByValue': True}, session)
                if ready.get('result', {}).get('value') is True:
                    break
                await asyncio.sleep(.02)
            else:
                raise TimeoutError('document did not complete')
            if 'scriptNavigation' in step:
                nav = step['scriptNavigation']
                destination = f"http://{nav.get('host', '127.0.0.1')}:{server.server_port}/navigation"
                if nav.get('set'):
                    destination += '?' + urllib.parse.urlencode({'set': nav['set']})
                action = destination
                if nav.get('via'):
                    action = f"http://{nav['via']}:{server.server_port}/cookie-redirect?" + urllib.parse.urlencode({'status': nav.get('status', 302), 'to': destination})
                method = nav.get('method', 'GET')
                surface = await client.call('Runtime.evaluate', {'expression': 'typeof document.createElement("form").submit', 'returnByValue': True}, session)
                if surface.get('result', {}).get('value') != 'function':
                    raise RuntimeError('HTMLFormElement.submit is unavailable; script-navigation probe cannot run')
                script = ('setTimeout(()=>{const f=document.createElement("form");f.method=' + json.dumps(method) +
                          ';f.action=' + json.dumps(action) + ';if(f.method==="get")for(const [name,value] of new URL(f.action).searchParams){const input=document.createElement("input");input.name=name;input.value=value;f.append(input)}document.body.append(f);f.submit()},10);true')
                await client.call('Runtime.evaluate', {'expression': script}, session)
                for _ in range(300):
                    try:
                        ready = await client.call('Runtime.evaluate', {'expression': 'location.hostname===' + json.dumps(nav.get('host', '127.0.0.1')) + '&&location.pathname==="/navigation"&&document.readyState==="complete"', 'returnByValue': True}, session)
                        if ready.get('result', {}).get('value') is True:
                            break
                    except RuntimeError as error:
                        if not any(word in str(error) for word in ('context', 'navigat')):
                            raise
                    await asyncio.sleep(.02)
                else:
                    raise TimeoutError('script navigation did not complete')
            source = (ROOT / step['file']).read_text(encoding='utf8')
            normalize = (ROOT / 'compatibility/corpus/normalize.js').read_text(encoding='utf8')
            wrapped = '(async()=>{try{const value=await (' + source + ');return {value:(' + normalize + ')(value)}}catch(e){return {exception:{name:e.name,message:e.message}}}})()'
            response = await client.call('Runtime.evaluate', {'expression': wrapped, 'awaitPromise': True, 'returnByValue': True}, session)
            if 'exceptionDetails' in response:
                raise RuntimeError(str(response['exceptionDetails']))
            if 'value' not in response.get('result', {}):
                raise RuntimeError('CDP did not return a by-value observation')
            observation = response['result']['value']
            if case.get('cookies'):
                # Network.getAllCookies is scoped to the target's browser context.
                cookies = await client.call('Network.getAllCookies', session=session)
                observation['cookies'] = cookie_metadata(cookies['cookies'])
            results.append(observation)
    except Exception as error:
        outcome = {'harnessError': str(error), 'partialObservations': results}
        if not native and target:
            try:
                trace = await client.call('Mimic.getTrace', session=session)
                outcome['diagnostics'] = [event for event in trace.get('events', [])
                                          if event.get('kind') in ('error', 'exception', 'semantic-missing', 'unsupported')][-20:]
            except Exception:
                pass  # Keep the original failure when the diagnostic channel is gone.
    finally:
        try:
            if context:
                await client.call('Target.disposeBrowserContext', {'browserContextId': context})
            elif target:
                await client.call('Target.closeTarget', {'targetId': target})
        except Exception as error:
            outcome['harnessError'] = outcome.get('harnessError', '') + ' Cleanup: ' + str(error)
    return outcome


async def main(args):
    corpus = json.loads(args.corpus.read_text(encoding='utf8'))
    args.output.mkdir(parents=True, exist_ok=False)
    server = FixtureServer()
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    clients, captures = {}, {}
    try:
        for name, base in [('chrome', args.chrome), ('mimic', args.mimic)]:
            clients[name] = await CDP().connect(base)
            if name == 'chrome' and clients[name].version['Browser'] not in ('Chrome/' + args.reference_version, 'HeadlessChrome/' + args.reference_version):
                raise RuntimeError('Reference must be Chrome ' + args.reference_version)
        rows = []
        for case in corpus:
            if args.only and case['id'] not in args.only:
                continue
            values = {}
            for name, client in clients.items():
                server.label = name + '/' + case['id']
                start = len(server.records)
                values[name] = await run_case(client, name == 'chrome', case, server)
                if case['id'] == 'network':
                    values[name]['transport'] = network_sequence(server.records[start:])
            # Only the fixture's ephemeral port is normalized, identically on both sides.
            normalized = {k: json.loads(json.dumps(v).replace(':' + str(server.server_port), ':<port>')) for k, v in values.items()}
            diff = differences(normalized['chrome'], normalized['mimic'])
            valid = all('harnessError' not in v for v in values.values())
            row = dict(id=case['id'], category=case['category'], valid=valid,
                       incomplete={k: probe_errors(v) for k, v in values.items() if probe_errors(v)},
                       differences=diff, **normalized)
            rows.append(row)
            print(case['id'], 'ERROR' if not valid else f'{len(diff)} differences', flush=True)
        fixtures = {step['file'] for case in corpus for step in case.get('steps', [case])}
        fixtures.add('compatibility/corpus/normalize.js')
        report = dict(versions={k: v.version for k, v in clients.items()}, cases=rows,
                      frozenReference=args.reference_version == '152.0.7977.82',
                      corpusSHA256=hashlib.sha256(args.corpus.read_bytes()).hexdigest(),
                      fixtures={p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in sorted(fixtures)},
                      limitations=['Loopback HTTP/1.1 wire oracle; TLS and QUIC are not measured.',
                                   'Mimic context is shared by its test-only process; Chrome context is fresh per case.',
                                   'Server timestamps are raw; runtime probes compare causal relations, not absolute durations.',
                                   'A top-level probe exception marks incomplete coverage, even if both runtimes throw.'])
        (args.output / 'report.json').write_text(json.dumps(report, indent=2), encoding='utf8')
        return int(any(not r['valid'] or r['incomplete'] or r['differences'] for r in rows))
    finally:
        for name, client in clients.items():
            captures[name] = client.events
            await client.close()
        (args.output / 'cdp-events.json').write_text(json.dumps(captures), encoding='utf8')
        (args.output / 'received-http.json').write_text(json.dumps(server.records, indent=2), encoding='utf8')
        server.shutdown()
        server.server_close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--chrome', default='http://127.0.0.1:9343')
    parser.add_argument('--reference-version', default='152.0.7977.82', help='Explicit alternate reference; report marks non-frozen comparisons')
    parser.add_argument('--mimic', required=True, help='Dedicated test instance; never point at a user browsing session')
    parser.add_argument('--corpus', type=pathlib.Path, default=ROOT / 'compatibility/corpus/index.json')
    parser.add_argument('--output', type=pathlib.Path, required=True, help='New output directory')
    parser.add_argument('--only', nargs='+')
    raise SystemExit(asyncio.run(main(parser.parse_args())))
