"""Fresh-target, bounded differential sweep of checked-in standalone probes.

Uses dedicated CDP instances and an empty loopback fixture; each probe gets a
new target, closed even after failure. The optional --mimic-binary mode launches
and terminates only its own Mimic subprocess, one per probe.
"""
import argparse
import asyncio
import hashlib
import json
import pathlib
import re
import socket
import subprocess
import threading
import time
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import websockets


def blocked(probe):
    code, name = probe['expression'], probe['name']
    patterns = {
        'requires parsed-template fixture, unavailable in standalone empty document': r"querySelector\('#parsed-template'\)",
        'external network or ICE': r'RTCPeerConnection|stun:|https://www.browserscan|\bfetch\s*\(|new WebSocket|sendBeacon\(',
        'device, permission prompt, or user state': r'getCurrentPosition\(|watchPosition\(|clipboard\.(?:read|write)|getUserMedia\(|getDisplayMedia\(|enumerateDevices\(|getDevices\(|getPorts\(|requestDevice\(|requestPort\(|credentials\.|keyboard\.|wakeLock\.request|xr\.requestSession|login\.setStatus|requestPermission\(',
    }
    return next((reason for reason, pattern in patterns.items() if re.search(pattern, code)), None)


class Page(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b'<!doctype html><html><head><title>Compatibility sweep</title></head><body></body></html>'
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


class CDP:
    def __init__(self, socket):
        self.socket, self.seq = socket, 0

    async def call(self, method, params=None):
        self.seq += 1
        ident = self.seq
        await self.socket.send(json.dumps({'id': ident, 'method': method, 'params': params or {}}))
        while True:
            value = json.loads(await self.socket.recv())
            if value.get('id') == ident:
                return value


def get_json(url):
    return json.load(urllib.request.urlopen(url, timeout=10))


async def run_endpoint(label, port, probes, out, origin, timeout, await_promise=True):
    base = f'http://127.0.0.1:{port}'
    metadata = await asyncio.to_thread(get_json, base + '/json/version')
    (out / (label + '-metadata.json')).write_text(json.dumps(metadata, indent=2), encoding='utf-8')
    results = []
    async with websockets.connect(metadata['webSocketDebuggerUrl'], max_size=32*1024*1024) as socket:
        browser = CDP(socket)
        for index, probe in enumerate(probes):
            result = {'id': probe['id'], 'name': probe['name'], 'source': probe['source'], 'sha256': probe['sha256']}
            target = None
            started = time.monotonic()
            try:
                created = await asyncio.wait_for(browser.call('Target.createTarget', {'url': 'about:blank'}), timeout)
                target = created['result']['targetId']
                tabs = await asyncio.to_thread(get_json, base + '/json/list')
                endpoint = next(t['webSocketDebuggerUrl'] for t in tabs if t['id'] == target)
                async with websockets.connect(endpoint, max_size=32*1024*1024) as page_socket:
                    page = CDP(page_socket)
                    await asyncio.wait_for(page.call('Page.enable'), timeout)
                    await asyncio.wait_for(page.call('Page.navigate', {'url': origin}), timeout)
                    for _ in range(100):
                        ready = await asyncio.wait_for(page.call('Runtime.evaluate', {'expression': 'location.href === '+json.dumps(origin)+' && document.readyState === "complete"', 'returnByValue': True}), timeout)
                        if ready.get('result', {}).get('result', {}).get('value') is True:
                            break
                        await asyncio.sleep(.02)
                    else:
                        raise RuntimeError('local fixture navigation not complete')
                    result['raw'] = await asyncio.wait_for(page.call('Runtime.evaluate', {'expression': probe['expression'], 'awaitPromise': await_promise, 'returnByValue': True, 'userGesture': False}), timeout)
                    result['status'] = 'exception' if 'exceptionDetails' in result['raw'].get('result', {}) else ('protocol_error' if 'error' in result['raw'] else 'ok')
            except Exception as error:
                result.update(status='harness_error', error=type(error).__name__+': '+str(error))
            finally:
                if target:
                    try:
                        await asyncio.wait_for(browser.call('Target.closeTarget', {'targetId': target}), timeout)
                    except Exception:
                        pass
            result['elapsedSeconds'] = round(time.monotonic()-started, 4)
            results.append(result)
            with (out / (label + '.jsonl')).open('a', encoding='utf-8') as stream:
                stream.write(json.dumps(result, ensure_ascii=False)+'\n')
            if (index+1) % 20 == 0:
                print(label, index+1, '/', len(probes), flush=True)
    return results


async def isolated_mimic(binary, probes, out, origin, timeout, await_promise=True):
    results = []
    for index, probe in enumerate(probes):
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        log_path = out / ('mimic-process-'+str(index)+'.log')
        with log_path.open('wb') as log:
            process = subprocess.Popen([str(pathlib.Path(binary).resolve()), '-listen', '127.0.0.1:'+str(port)], stdout=log, stderr=log, creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0))
            with (out / 'mimic-processes.jsonl').open('a') as stream:
                stream.write(json.dumps({'index': index, 'id': probe['id'], 'pid': process.pid, 'port': port})+'\n')
            try:
                for _ in range(100):
                    try:
                        await asyncio.to_thread(get_json, f'http://127.0.0.1:{port}/json/version')
                        break
                    except Exception:
                        await asyncio.sleep(.05)
                batch = await run_endpoint('mimic', port, [probe], out, origin, timeout, await_promise=await_promise)
                results.extend(batch)
            finally:
                process.terminate()
                try:
                    await asyncio.to_thread(process.wait, 5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    await asyncio.to_thread(process.wait)
        if (index+1) % 20 == 0:
            print('isolated mimic', index+1, '/', len(probes), flush=True)
    return results


def observation(row):
    raw = row.get('raw', {}).get('result', {})
    value = raw.get('result', {})
    if row['status'] != 'ok':
        exception = raw.get('exceptionDetails', {}).get('exception', {})
        return {'status': row['status'], 'exception': exception.get('className'), 'description': exception.get('description', '').split('\n')[0], 'error': row.get('error')}
    return {key: value[key] for key in ['type', 'subtype', 'value', 'unserializableValue', 'description'] if key in value and not (key == 'description' and 'value' in value)}


def differences(a, b, path='$'):
    if type(a) is not type(b):
        return [path]
    if isinstance(a, dict):
        return [p for key in sorted(a.keys() | b.keys()) for p in ([path+'.'+key] if key not in a or key not in b else differences(a[key], b[key], path+'.'+key))]
    if isinstance(a, list):
        if len(a) != len(b):
            return [path+'.length']
        return [p for i, (x, y) in enumerate(zip(a, b)) for p in differences(x, y, path+f'[{i}]')]
    return [] if a == b else [path]


async def main(args):
    out = pathlib.Path(args.output)
    out.mkdir(parents=True, exist_ok=True)
    if any(out.glob('*.jsonl')):
        raise RuntimeError('Output already contains results; use a new directory to preserve evidence')
    probes, skipped = [], []
    for source in sorted(pathlib.Path(args.repo, 'compatibility').glob('probes*.json')):
        if '.expected.' in source.name:
            continue
        data = json.loads(source.read_text(encoding='utf-8-sig'))
        rows = data if isinstance(data, list) else [{'name': key, 'expression': '('+value+')()'} for key, value in data.items()]
        for index, row in enumerate(rows):
            row = dict(row, source=source.name, id=source.stem+':'+str(index))
            row['sha256'] = hashlib.sha256(row['expression'].encode()).hexdigest()
            reason = blocked(row)
            if reason:
                skipped.append(dict(row, reason=reason))
            else:
                probes.append(row)
    (out / 'manifest.json').write_text(json.dumps({'executed': probes, 'skipped': skipped, 'timeoutSeconds': args.timeout, 'fixture': 'empty HTML document on same loopback origin; fresh target for every probe'}, indent=2), encoding='utf-8')
    server = ThreadingHTTPServer(('127.0.0.1', args.http_port), Page)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    origin = f'http://127.0.0.1:{args.http_port}/'
    async def chrome_batch(label, port):
        if args.reuse_chrome:
            data = [json.loads(line) for line in pathlib.Path(args.reuse_chrome, label+'.jsonl').read_text(encoding='utf-8').splitlines()]
            assert [(r['id'], r['sha256']) for r in data] == [(r['id'], r['sha256']) for r in probes]
            (out / (label+'.jsonl')).write_text('\n'.join(json.dumps(row) for row in data)+'\n', encoding='utf-8')
            return data
        return await run_endpoint(label, port, probes, out, origin, args.timeout)
    mimic_batch = isolated_mimic(args.mimic_binary, probes, out, origin, args.timeout) if args.mimic_binary else run_endpoint('mimic', args.mimic, probes, out, origin, args.timeout)
    batches = await asyncio.gather(chrome_batch('chrome_headful', args.chrome), chrome_batch('chrome_headless', args.headless), mimic_batch)
    summary = []
    for probe, c, h, m in zip(probes, *batches):
        cv, hv, mv = map(observation, [c, h, m])
        paths = differences(cv, mv)
        profile_paths = differences(cv, hv)
        classification = 'match' if not paths else ('profile_sensitive' if set(paths) & set(profile_paths) else 'candidate')
        if any(x['status'] == 'harness_error' for x in [c, h, m]):
            classification = 'harness_error'
        elif paths and m['status'] != c['status']:
            classification = 'exception_mismatch'
        elif paths and re.search(r'performance|timing|display|^navigator identity$|intl|^connection$|quota|storage.estimate|synchronous execution clock', probe['name'], re.I):
            classification = 'environment_or_timing_review'
        summary.append({'id': probe['id'], 'name': probe['name'], 'class': classification, 'paths': paths, 'chrome': cv, 'headless': hv, 'mimic': mv})
    (out / 'comparison.json').write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding='utf-8')
    counts = {key: sum(row['class'] == key for row in summary) for key in sorted({row['class'] for row in summary})}
    (out / 'counts.json').write_text(json.dumps(dict(executed=len(probes), uniqueExpressions=len({p['sha256'] for p in probes}), skipped=len(skipped), classifications=counts), indent=2), encoding='utf-8')
    table = ['# Standalone fresh-page sweep', '', 'Exact values and CDP responses are retained in comparison.json and endpoint JSONL files. Classifications are triage, not established root causes. Chrome headful/headless disagreement is a profile sensitivity signal; timing and environment values require controlled reruns. Exceptions in both runtimes are recorded, not counted as compatibility success.', '', '| Probe | Classification | Differing JSON paths |', '|---|---|---|']
    for row in summary:
        table.append('| '+row['id']+' — '+row['name'].replace('|', '\\|')+' | '+row['class']+' | '+', '.join(row['paths'][:12]).replace('|', '\\|')+(' …' if len(row['paths']) > 12 else '')+' |')
    (out / 'README.md').write_text('\n'.join(table)+'\n', encoding='utf-8')
    print(json.dumps(counts), flush=True)
    server.shutdown()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', default='.')
    parser.add_argument('--output', required=True)
    parser.add_argument('--chrome', type=int, default=19343)
    parser.add_argument('--headless', type=int, default=19344)
    parser.add_argument('--mimic', type=int, default=19350)
    parser.add_argument('--http-port', type=int, default=19351)
    parser.add_argument('--timeout', type=float, default=8)
    parser.add_argument('--mimic-binary', help='Isolate each Mimic probe in an owned subprocess; preserves default snapshot mode')
    parser.add_argument('--reuse-chrome', help='Reuse exact-expression Chrome JSONL evidence from a previous sweep')
    asyncio.run(main(parser.parse_args()))
