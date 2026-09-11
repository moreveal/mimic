"""Bounded CDP differential discovery; runtime implementation is never modified."""
from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
from pathlib import Path
import sys
from urllib.parse import urlparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread

from pyppeteer import connect
from oracle import PINNED_PRODUCT, product, prepare_page, capture_metadata
from fuzz_probes import generate_probes, discovery_expression, surface_probes, render_probe

MISSING = {"$missing": True}


def differences(left, right, path=()):
    """Typed JSON comparison; missing keys are distinct from null and bool from int."""
    if type(left) is not type(right):
        yield path, left, right
    elif isinstance(left, dict):
        for key in sorted(left.keys() | right.keys()):
            if key not in left or key not in right:
                yield path + (key,), left.get(key, MISSING), right.get(key, MISSING)
            else:
                yield from differences(left[key], right[key], path + (key,))
    elif isinstance(left, list):
        if len(left) != len(right):
            yield path, left, right
        else:
            for i, (a, b) in enumerate(zip(left, right)):
                yield from differences(a, b, path + (i,))
    elif left != right:
        yield path, left, right


def at_path(value, path):
    for key in path:
        try:
            value = value[key]
        except (KeyError, IndexError, TypeError):
            return MISSING
    return value


def same(left, right):
    return not any(differences(left, right))


async def reduce_setup(probe, predicate):
    """Deletion ddmin followed by a fixed-point single deletion check."""
    current = dict(probe, setup=list(probe['setup']))
    n = 2
    while current['setup']:
        size = max(1, (len(current['setup']) + n - 1) // n)
        reduced = False
        for start in range(0, len(current['setup']), size):
            candidate = dict(current, setup=current['setup'][:start] + current['setup'][start + size:])
            if await predicate(candidate):
                current, n, reduced = candidate, max(2, n - 1), True
                break
        if not reduced:
            if size == 1:
                break
            n = min(len(current['setup']), n * 2)
    return current


class Fixture(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b'<!doctype html><html><head><title>compat-fuzz</title></head><body></body></html>'
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionAbortedError, ConnectionResetError):
            pass  # A timed-out probe may close its target during the response.

    def log_message(self, *_):
        pass


class Endpoint:
    def __init__(self, endpoint, url, timeout):
        self.endpoint, self.url, self.timeout = endpoint, url, timeout

    async def evaluate(self, expression):
        # A timed-out target must not contaminate subsequent protocol sessions.
        browser = await connect(browserURL=self.endpoint, defaultViewport=None)
        page = None
        pending = []
        async def bounded(coro):
            task = asyncio.create_task(coro)
            pending.append(task)
            done, _ = await asyncio.wait([task], timeout=self.timeout)
            if not done:
                raise TimeoutError('CDP operation exceeded probe timeout')
            return task.result()
        try:
            page = await bounded(browser.newPage())
            await bounded(page.goto(self.url, {'waitUntil': 'load', 'timeout': int(self.timeout * 1000)}))
            return await bounded(page.evaluate(expression, force_expr=True))
        finally:
            try:
                if page is not None:
                    await bounded(page.close())
            finally:
                await browser.disconnect()
                # Disconnect resolves outstanding protocol operations without
                # cancelling their callback futures (pyppeteer 2 limitation).
                for task in pending:
                    if task.done() and not task.cancelled():
                        task.exception()
                    else:
                        task.add_done_callback(lambda t: t.exception() if not t.cancelled() else None)


def projected_repro(probe, path):
    return ('// Run on the fixture origin listed in result.json. Returns a Promise.\n'
            '(async () => {\n  let value = await ' + render_probe(probe) + ';\n'
            '  for (const key of ' + json.dumps(path) + ') {\n'
            '    if (value === null || value === undefined || !Object.prototype.hasOwnProperty.call(value, key)) return {"$missing":true};\n'
            '    value = value[key];\n  }\n  return value;\n})()\n')


async def run(args):
    if product(args.chrome_cdp) != PINNED_PRODUCT:
        raise RuntimeError('Chrome oracle drift: expected ' + PINNED_PRODUCT)
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=False)
    server = ThreadingHTTPServer(('127.0.0.1', args.fixture_port), Fixture)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    origin = f'http://127.0.0.1:{server.server_port}'
    cross = f'http://localhost:{server.server_port}'
    browsers = []
    try:
        for endpoint in (args.chrome_cdp, args.mimic_cdp):
            browsers.append(await connect(browserURL=endpoint, defaultViewport=None))
        page = await browsers[0].newPage()
        try:
            await prepare_page(browsers[0], page, args.chrome_mode, '1280x800')
            await page.goto(origin)
            metadata = await capture_metadata(args.chrome_cdp, browsers[0], page,
                mode=args.chrome_mode, environment_profile_id=args.profile_id)
            # Endpoints supplied by callers do not prove a fresh profile.
            metadata['profileFreshness'] = 'caller-declared' if args.fresh_profile else 'unverified'
        finally:
            await page.close()
        endpoints = [Endpoint(e, origin, args.timeout) for e in (args.chrome_cdp, args.mimic_cdp)]
        def materialize(probe):
            def subst(s):
                return s.replace('__FUZZ_CROSS_ORIGIN__', cross).replace('__FUZZ_ORIGIN__', origin)
            return dict(probe, setup=[subst(s) for s in probe['setup']], expression=subst(probe['expression']))
        async def pair(probe):
            expression = render_probe(probe)
            # Sequential targets avoid focus competition in Chrome-vs-Chrome
            # controls; concurrency itself is not an observation under test.
            return [await e.evaluate(expression) for e in endpoints]
        probes = [json.loads(Path(args.replay).read_text(encoding='utf-8'))['probe']] if args.replay else generate_probes()
        discovery_count = 0
        if not args.corpus_only and not args.replay:
            discoveries = await asyncio.gather(*(e.evaluate(discovery_expression()) for e in endpoints))
            merged = {key: sorted(set(discoveries[0].get(key, [])) | set(discoveries[1].get(key, []))) for key in discoveries[0].keys() | discoveries[1].keys()}
            discovered = surface_probes(merged)
            discovery_count = len(discovered)
            probes += discovered[:args.max_surface_probes]
        probes = [materialize(p) for p in probes if not args.filter or args.filter in p['id']]
        summary = {'checks': 0, 'matched': 0, 'unstable': 0, 'errors': 0, 'divergences': 0, 'uniqueGroups': 0,
                   'surfaceGenerated': discovery_count, 'surfaceLimit': args.max_surface_probes}
        groups = {}
        errors = []
        for probe in probes:
            summary['checks'] += 1
            try:
                left, right = await pair(probe)
                leaves = list(differences(left, right))
                if not leaves:
                    summary['matched'] += 1
                    continue
                # Exact repeatability is required; transport failures are never semantic differences.
                stable = True
                for _ in range(args.repeats - 1):
                    if not same(await pair(probe), [left, right]):
                        stable = False
                        break
                if not stable:
                    summary['unstable'] += 1
                    continue
                path, expected, actual = leaves[0]
                signature = json.dumps([probe['category'], path, expected, actual], sort_keys=True, ensure_ascii=True)
                key = hashlib.sha256(signature.encode()).hexdigest()[:16]
                if key in groups:
                    summary['divergences'] += 1
                    groups[key]['members'].append(probe['id'])
                    continue
                async def preserves(candidate):
                    for _ in range(args.repeats):
                        try:
                            a, b = await pair(candidate)
                        except Exception:
                            return False
                        if not same(at_path(a, path), expected) or not same(at_path(b, path), actual):
                            return False
                    return True
                minimized = await reduce_setup(probe, preserves)
                a, b = await pair(minimized)
                if not same(at_path(a, path), expected) or not same(at_path(b, path), actual):
                    summary['unstable'] += 1
                    continue
                record = {'id': key, 'category': probe['category'], 'members': [probe['id']],
                    'path': path, 'chrome': at_path(a, path), 'mimic': at_path(b, path),
                    'probe': minimized, 'originalSetupStatements': len(probe['setup']),
                    'minimality': '1-minimal setup deletion for fixed observation and exact result pair; not global JS minimum',
                    'fixtureOrigin': origin, 'crossOrigin': cross}
                summary['divergences'] += 1
                groups[key] = record
                summary['uniqueGroups'] = len(groups)
                group_dir = output / key
                group_dir.mkdir()
                (group_dir / 'repro.js').write_text(projected_repro(minimized, path), encoding='utf-8')
                (group_dir / 'result.json').write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
            except Exception as error:
                summary['errors'] += 1
                errors.append({'probe': probe['id'], 'error': type(error).__name__ + ': ' + str(error)})
            if summary['checks'] % 10 == 0:
                print(json.dumps(summary), file=sys.stderr, flush=True)
        summary['uniqueGroups'] = len(groups)
        for key, record in groups.items():
            (output / key / 'result.json').write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
        report = {'schemaVersion': 1, 'captureMetadata': metadata, 'mimicProduct': product(args.mimic_cdp),
                  'summary': summary, 'groups': [{'id': k, 'category': r['category'], 'members': r['members']} for k, r in groups.items()],
                  'errors': errors, 'fixtureOrigin': origin}
        (output / 'summary.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
        print(json.dumps(summary, indent=2))
        return 2 if errors else 0
    finally:
        for browser in browsers:
            try:
                await browser.disconnect()
            except Exception as error:
                print('CDP cleanup: ' + repr(error), file=sys.stderr)
        server.shutdown()
        server.server_close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--chrome-cdp', required=True)
    parser.add_argument('--mimic-cdp', required=True)
    parser.add_argument('--output', default='compat-fuzz-results')
    parser.add_argument('--chrome-mode', choices=['headful', 'headless'], default='headful')
    parser.add_argument('--profile-id', default='compat-fuzz-caller-uncontrolled')
    parser.add_argument('--fresh-profile', action='store_true')
    parser.add_argument('--fixture-port', type=int, default=0)
    parser.add_argument('--timeout', type=float, default=8)
    parser.add_argument('--repeats', type=int, default=2)
    parser.add_argument('--max-surface-probes', type=int, default=100)
    parser.add_argument('--corpus-only', action='store_true')
    parser.add_argument('--filter', default='')
    parser.add_argument('--replay', help='Replay a saved result.json on its original fixture port')
    args = parser.parse_args()
    if args.replay and args.fixture_port == 0:
        args.fixture_port = urlparse(json.loads(Path(args.replay).read_text(encoding='utf-8'))['fixtureOrigin']).port
    if args.repeats < 2 or args.max_surface_probes < 0 or args.timeout <= 0:
        parser.error('repeats must be >=2, surface limit >=0, timeout >0')
    return asyncio.run(run(args))


if __name__ == '__main__':
    sys.exit(main())
