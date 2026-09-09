"""Run unmodified, hash-recorded WPT files from the pinned Blink checkout.

This is a compatibility runner, not part of the frozen performance harness.
The server fetches dependencies lazily from the same revision. No test result
is counted as passing when the harness fails or does not complete.
"""
import argparse
import asyncio
import base64
import hashlib
import json
import html
import re
import subprocess
import sys
import threading
import time
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from pyppeteer import connect
from oracle import PINNED_PRODUCT, endpoint_version, capture_metadata

ROOT = Path(__file__).resolve().parents[1]
LOCK = json.loads((ROOT / 'chrome/152/target.json').read_text())
BASE = ('https://chromium.googlesource.com/chromium/src/+/' +
        LOCK['chromium_commit'] + '/third_party/blink/web_tests/external/wpt/')
DOMAINS = {
    'forms': [
        'html/semantics/forms/the-input-element/valueMode.html',
        'html/semantics/forms/the-input-element/input-value-invalidstateerr.html',
        'html/semantics/forms/the-textarea-element/value-defaultValue-textContent.html',
        'html/semantics/forms/the-select-element/select-value.html',
        'html/semantics/forms/the-select-element/selected-index.html',
    ],
    'templates': [
        'template-element/content-attribute.html',
        'template-element/template-content.html',
        'template-element/template-content-hierarcy.html',
        'additions-to-the-steps-to-clone-a-node/template-clone-children.html',
        'innerhtml-on-templates/innerhtml.html',
    ],
    'custom-elements': [
        'CustomElementRegistry.html', 'CustomElementRegistry-getName.html',
        'HTMLElement-constructor.html', 'attribute-changed-callback.html',
        'connected-callbacks.html', 'disconnected-callbacks.html', 'upgrading.html',
        'custom-element-reaction-queue.html',
    ],
    'fetch': [
        'response-init-001.any.js', 'response-init-002.any.js',
        'response-init-contenttype.any.js', 'response-clone.any.js',
        'response-consume-empty.any.js', 'response-static-error.any.js',
        'response-static-json.any.js', 'response-static-redirect.any.js',
    ],
    'encoding': [
        'api-basics.any.js', 'api-invalid-label.any.js', 'api-surrogates-utf8.any.js',
        'textdecoder-arguments.any.js', 'textdecoder-fatal.any.js',
        'textdecoder-ignorebom.any.js', 'textdecoder-streaming.any.js',
        'textdecoder-utf16-surrogates.any.js', 'encodeInto.any.js',
    ],
    'nodes': [
        'CharacterData-data.html', 'CharacterData-appendData.html',
        'CharacterData-deleteData.html', 'CharacterData-insertData.html',
        'CharacterData-replaceData.html', 'CharacterData-substringData.html',
        'CharacterData-surrogates.html', 'Node-cloneNode.html',
        'Node-childNodes.html', 'Node-appendChild.html', 'Node-removeChild.html',
        'Node-replaceChild.html', 'Element-firstElementChild.html',
        'Element-lastElementChild.html', 'Element-nextElementSibling.html',
        'Element-previousElementSibling.html',
    ],
    'selectors': [
        'ParentNode-querySelector-All.html', 'ParentNode-querySelector-escapes.html',
        'ParentNode-querySelector-scope.html', 'ParentNode-querySelectors-exclusive.html',
        'ParentNode-querySelectors-space-and-dash-attribute-value.html',
        'ParentNode-querySelectorAll-removed-elements.html',
        'querySelector-empty-id.html', 'querySelector-id-nth-child.html',
        'DocumentFragment-querySelectorAll-after-modification.html',
    ],
    'mutations': [
        'MutationObserver-attributes.html', 'MutationObserver-callback-arguments.html',
        'MutationObserver-characterData.html', 'MutationObserver-childList.html',
        'MutationObserver-disconnect.html', 'MutationObserver-takeRecords.html',
        'MutationObserver-textContent.html', 'MutationObserver-sanity.html',
    ],
}
DOMAIN_ROOTS = {
    'forms': '',
    'templates': 'html/semantics/scripting-1/the-template-element/',
    'custom-elements': 'custom-elements/', 'fetch': 'fetch/api/response/',
    'encoding': 'encoding/',
}
HOOK = """() => {
  globalThis.__wpt_results = []; globalThis.__wpt_done = null;
  for (const [name, callback] of [
    ['add_result_callback', t => __wpt_results.push({name:t.name,status:t.status,message:t.message})],
    ['add_completion_callback', (tests,status) => {__wpt_results=tests.map(t=>({name:t.name,status:t.status,message:t.message}));__wpt_done={status:status.status,message:status.message}}]
  ]) { let value; Object.defineProperty(globalThis,name,{configurable:true,
    get(){return value},set(fn){value=fn;queueMicrotask(()=>fn(callback))}}); }
  let setup; Object.defineProperty(globalThis,'setup',{configurable:true,
    get(){return setup},set(fn){setup=fn;queueMicrotask(()=>fn({output:OUTPUT_ENABLED}))}});
}"""


async def run(args, server, receipt):
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    product = endpoint_version(args.endpoint).get('Browser')
    if args.chrome and product != PINNED_PRODUCT:
        raise RuntimeError(f'Unexpected Chrome: {product}')
    report = dict(domain=args.domain, product=product, chromium_commit=LOCK['chromium_commit'],
                  wpt_tree=LOCK['wpt_revision'], binary=receipt,
                  test_source_revision=args.source_revision, test_source_origin=args.source_base,
                  harness_output=args.harness_output, tests=[])
    if args.domain == 'forms':
        report['excluded_files'] = [{'path':'html/semantics/forms/the-input-element/email-set-value.html',
            'reason':'Requires test_driver.send_keys and native text selection; this runner has no testdriver backend.'}]
    try:
        for filename in (args.files or DOMAINS[args.domain]):
            path = DOMAIN_ROOTS.get(args.domain, 'dom/nodes/') + filename
            page = await browser.newPage()
            await page.evaluateOnNewDocument(HOOK.replace('OUTPUT_ENABLED', str(args.harness_output).lower()))
            if args.chrome and 'oracle' not in report:
                report['oracle'] = await capture_metadata(args.endpoint, browser, page,
                    mode='headful', environment_profile_id=args.profile_id)
                report['oracle']['profileFreshness'] = 'reused-controlled'
            errors = []
            console = []
            page._client.on('Runtime.exceptionThrown', lambda event: errors.append(event.get('exceptionDetails',{})))
            page._client.on('Runtime.consoleAPICalled', lambda event: console.append(event))
            result = dict(path=path)
            started = time.monotonic()
            try:
                try:
                    served_path = path + '.runner.html' if path.endswith('.js') else path
                    await page.goto(f'http://127.0.0.1:{server.server_port}/{served_path}',
                                    {'waitUntil':'load', 'timeout':args.timeout * 1000})
                except Exception as error:
                    result['navigation_error'] = str(error)
                for _ in range(args.timeout * 5):
                    state = await page.evaluate('({done:__wpt_done,results:__wpt_results})')
                    if state['done'] is not None:
                        break
                    await asyncio.sleep(.2)
                result.update(state)
            except Exception as error:
                result['runner_error'] = str(error)
            result['errors'] = errors
            result['console'] = console
            if not args.chrome:
                try:
                    trace = await page._client.send('Mimic.getTrace')
                    result['trace'] = [event for event in trace.get('events',[]) if event.get('kind') != 'cdp']
                except Exception as error:
                    result['trace_error'] = str(error)
            result['completed'] = result.get('done') is not None
            result['elapsed_seconds'] = round(time.monotonic() - started, 3)
            result['harness_ok'] = result['completed'] and result['done']['status'] == 0
            result['status_counts'] = {str(code):sum(t['status']==code for t in result.get('results',[])) for code in range(4)}
            result['counted_passes'] = result['status_counts']['0'] if result['harness_ok'] else 0
            report['tests'].append(result)
            sources = args.output.parent/(args.output.stem+'-sources.jsonl')
            if sources.exists():
                report['sources'] = list({entry['path']:entry for entry in
                    (json.loads(line) for line in sources.read_text(encoding='utf-8').splitlines())}.values())
            report['summary'] = dict(files=len(report['tests']),
                harness_ok=sum(t['harness_ok'] for t in report['tests']),
                passed=sum(t['counted_passes'] for t in report['tests']),
                failed=sum(t['status_counts']['1'] for t in report['tests']))
            args.output.write_text(json.dumps(report, indent=2), encoding='utf-8')
            counts = {code:sum(t['status']==code for t in result.get('results',[])) for code in range(4)}
            print(path, counts, result.get('done'), flush=True)
            await page.close()
    finally:
        await browser.disconnect()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--domain', choices=DOMAINS, required=True)
    parser.add_argument('--endpoint', required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--cache', type=Path, default=ROOT/'.build/wpt-pinned')
    parser.add_argument('--chrome', action='store_true')
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--sha256', required=True)
    parser.add_argument('--timeout', type=int, default=15)
    parser.add_argument('--files', nargs='+', help='Explicit files within the selected domain')
    parser.add_argument('--harness-output', action='store_true', help='Enable WPT HTML output (diagnostic only)')
    parser.add_argument('--profile-id', default='chrome152-domain-tests-reused')
    parser.add_argument('--manifest', type=Path, help='Pinned upstream WPT Git revision and SHA-256 file manifest')
    args = parser.parse_args()
    args.source_revision = LOCK['chromium_commit']
    args.source_base = BASE
    args.expected_sources = {}
    if args.manifest:
        manifest = json.loads(args.manifest.read_text(encoding='utf-8'))
        if not re.fullmatch('[0-9a-f]{40}', manifest['revision']):
            raise RuntimeError('Manifest requires an immutable WPT Git commit')
        args.source_revision = manifest['revision']
        args.source_base = 'https://raw.githubusercontent.com/web-platform-tests/wpt/'+args.source_revision+'/'
        args.expected_sources = manifest['files']
    actual = hashlib.sha256(args.binary.read_bytes()).hexdigest()
    if actual.lower() != args.sha256.lower():
        raise RuntimeError('Binary SHA-256 mismatch; stopped')
    receipt = dict(path=str(args.binary.resolve()), sha256=actual)
    endpoint = urllib.parse.urlsplit(args.endpoint)
    if sys.platform == 'win32' and endpoint.hostname in ('127.0.0.1', 'localhost', '::1'):
        port = endpoint.port or 80
        command = (f"$ownerIds = @(Get-NetTCPConnection -LocalPort {port} -State Listen | "
                   "Select-Object -ExpandProperty OwningProcess -Unique); "
                   "if ($ownerIds.Count -ne 1) { throw 'Endpoint has no unique listener' }; "
                   "Get-CimInstance Win32_Process -Filter ('ProcessId = ' + $ownerIds[0]) | "
                   "Select-Object ProcessId,ExecutablePath,CreationDate | ConvertTo-Json -Compress")
        listener = json.loads(subprocess.check_output(
            ['powershell', '-NoProfile', '-NonInteractive', '-Command', command], text=True))
        if Path(listener['ExecutablePath']).resolve() != args.binary.resolve():
            raise RuntimeError('Endpoint process executable differs from --binary; stopped')
        receipt['listener'] = listener
    args.output.parent.mkdir(parents=True, exist_ok=True)
    source_log = args.output.parent/(args.output.stem+'-sources.jsonl')
    source_log.write_text('', encoding='utf-8')
    cache_lock = threading.Lock()
    verification_file = args.cache/('.verified-'+args.source_revision+'.json')
    verified = json.loads(verification_file.read_text()) if verification_file.exists() else {}

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *unused):
            pass

        def do_GET(self):
            path = urllib.parse.urlsplit(self.path).path.lstrip('/')
            wrapper = path.endswith('.js.runner.html')
            if wrapper:
                path = path[:-len('.runner.html')]
            if path == 'favicon.ico':
                self.send_response(204); self.end_headers(); return
            file = (args.cache/path).resolve()
            if not file.is_relative_to(args.cache.resolve()) or not path:
                self.send_error(404); return
            try:
                with cache_lock:
                    data = file.read_bytes() if file.exists() else None
                    digest = hashlib.sha256(data).hexdigest() if data is not None else None
                    if digest is None or verified.get(path) != digest:
                        raw = urllib.request.urlopen(args.source_base+path+('' if args.manifest else '?format=TEXT'), timeout=30).read()
                        pinned_data = raw if args.manifest else base64.b64decode(raw)
                        if data is not None and data != pinned_data:
                            raise RuntimeError('Cached source differs from pinned upstream: '+path)
                        data = pinned_data
                        file.parent.mkdir(parents=True, exist_ok=True)
                        file.write_bytes(data)
                        verified[path] = hashlib.sha256(data).hexdigest()
                        verification_file.write_text(json.dumps(verified, indent=2), encoding='utf-8')
                    if path in args.expected_sources and hashlib.sha256(data).hexdigest() != args.expected_sources[path]:
                        raise RuntimeError('Pinned source differs from manifest SHA-256: '+path)
                    with (args.output.parent/(args.output.stem+'-sources.jsonl')).open('a', encoding='utf-8') as log:
                        log.write(json.dumps(dict(path=path, sha256=hashlib.sha256(data).hexdigest(), source=args.source_base+path))+'\n')
                self.send_response(200)
                if wrapper:
                    # WPT's window variants load the unmodified .any.js after
                    # testharness and the file's declared META script dependencies.
                    dependencies = re.findall(r'^//\s*META:\s*script=(.+)$', data.decode('utf-8'), re.M)
                    scripts = ['/resources/testharness.js', *dependencies, '/'+path]
                    data = ('<!doctype html><meta charset="utf-8">'+''.join(
                        '<script src="'+html.escape(src.strip(), quote=True)+'"></script>' for src in scripts)).encode()
                self.send_header('Content-Type', 'text/javascript' if path.endswith('.js') and not wrapper else 'text/html; charset=utf-8')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers(); self.wfile.write(data)
            except Exception as error:
                try:
                    self.send_error(404, str(error))
                except (BrokenPipeError, ConnectionError):
                    pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        asyncio.run(run(args, server, receipt))
    finally:
        server.shutdown()


if __name__ == '__main__':
    main()
