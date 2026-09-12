"""Frozen headful Chrome Trusted Types/CSP probes; no Chromium implementation input."""
import argparse
import asyncio
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import sys
import threading
from urllib.parse import urlparse, parse_qs

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'compatibility'))
from oracle import capture_metadata, default_profile_id, prepare_page
from pyppeteer import connect

POLICIES = {
    'open': '',
    'required': "require-trusted-types-for 'script'",
    'restricted': "require-trusted-types-for 'script'; trusted-types allowed default",
    'none': "trusted-types 'none'",
    'duplicates': "trusted-types allowed default 'allow-duplicates'",
    'noeval': "script-src 'self' 'unsafe-inline'; require-trusted-types-for 'script'",
    'wildcard': "trusted-types * 'allow-duplicates'",
    'reportonly': "require-trusted-types-for 'script'; trusted-types 'none'",
}


class Fixture(BaseHTTPRequestHandler):
    def do_GET(self):
        name = parse_qs(urlparse(self.path).query).get('policy', ['open'])[0]
        body = b'<!doctype html><title>Trusted Types oracle</title><body><div id="box"></div><iframe id="blank"></iframe></body>'
        javascript = urlparse(self.path).path == '/trusted-import.js'
        if javascript:
            body = b'globalThis.imported = (globalThis.imported || 0) + 1;'
        self.send_response(200)
        self.send_header('Content-Type', 'text/javascript' if javascript else 'text/html')
        if POLICIES[name]:
            self.send_header('Content-Security-Policy-Report-Only' if name == 'reportonly' else 'Content-Security-Policy', POLICIES[name])
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


async def main(args):
    server = ThreadingHTTPServer(('127.0.0.1', args.port), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    output = {'fixtureOrigin': f'http://127.0.0.1:{server.server_port}', 'cases': {}}
    try:
        for policy, probe, mode in [
            ('open', 'sinks', 'plain'), ('required', 'sinks', 'plain'),
            ('required', 'sinks', 'default'), ('required', 'sinks', 'trusted'),
            ('required', 'sinks', 'wrong'), ('required', 'sinks', 'null'),
            ('required', 'sinks', 'throw'), ('noeval', 'sinks', 'default'), ('noeval', 'sinks', 'plain'), ('noeval', 'sinks', 'null'), ('noeval', 'sinks', 'throw'), ('noeval', 'sinks', 'trusted'),
            ('open', 'policy', ''), ('restricted', 'policy', ''),
            ('none', 'policy', ''), ('duplicates', 'policy', ''),
            ('required', 'realms', ''), ('open', 'realms', ''),
            ('noeval', 'bypass', ''), ('noeval', 'bypass', 'before'), ('required', 'binding', ''), ('required', 'worker', ''), ('required', 'dynamic', ''), ('required', 'edges', ''), ('required', 'execution', ''), ('open', 'lifecycle', ''),
            ('wildcard', 'policy', ''), ('reportonly', 'sinks', 'default'), ('reportonly', 'sinks', 'plain'), ('reportonly', 'sinks', 'null'), ('reportonly', 'sinks', 'throw'), ('open', 'meta', ''),
        ]:
            page = await browser.newPage()
            try:
                if not args.mimic:
                    await prepare_page(browser, page, 'headful', '1280x800')
                if probe == 'bypass' and mode == 'before':
                    await page._client.send('Page.setBypassCSP', {'enabled': True})
                await page.goto(output['fixtureOrigin'] + '/?policy=' + policy)
                if probe == 'bypass':
                    await page._client.send('Page.setBypassCSP', {'enabled': True})
                source = (ROOT / 'internal/browser/testdata' / f'trusted_types_{probe}_oracle.js').read_text()
                source = source.replace('"$MODE"', json.dumps(mode))
                result = await page._client.send('Runtime.evaluate', {
                    'expression': source, 'awaitPromise': True, 'returnByValue': True,
                    'allowUnsafeEvalBlockedByCSP': False,
                })
                if 'exceptionDetails' in result:
                    raise RuntimeError((policy, probe, mode, result))
                key = ':'.join([policy, probe, mode]).rstrip(':')
                output['cases'][key] = result['result'].get('value')
                print(key, flush=True)
                if not args.mimic and 'captureMetadata' not in output:
                    output['captureMetadata'] = await capture_metadata(args.endpoint, browser, page,
                        mode='headful', environment_profile_id=default_profile_id('headful'))
            finally:
                await page.close()
    finally:
        await browser.disconnect()
        server.shutdown()
    Path(args.output).write_text(json.dumps(output, indent=2) + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--endpoint', default='http://127.0.0.1:9532')
    parser.add_argument('--port', type=int, default=19532)
    parser.add_argument('--mimic', action='store_true')
    parser.add_argument('--output', default=str(ROOT / 'internal/browser/testdata/trusted_types_chrome152.json'))
    asyncio.run(main(parser.parse_args()))
