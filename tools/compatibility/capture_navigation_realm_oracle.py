"""Capture the focused navigation probes using the shared Chrome provenance policy."""
import argparse
import asyncio
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import sys
import threading

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'compatibility'))
from oracle import capture_metadata, default_profile_id, prepare_page
from pyppeteer import connect


class Fixture(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b'<!doctype html><title>Navigation realm oracle</title><body><p id="newDocument"></p></body>'
        self.send_response(200)
        self.send_header('Content-Type', 'text/html')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


async def main(endpoint, names):
    server = ThreadingHTTPServer(('127.0.0.1', 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    try:
        for name in names:
            page = await browser.newPage()
            try:
                await prepare_page(browser, page, 'headful', '1280x800')
                await page.goto(f'http://127.0.0.1:{server.server_port}/')
                fixture = ROOT / 'internal/browser/testdata' / (name + '_oracle.js')
                setup = fixture.with_name(name + '_setup.js')
                if setup.exists():
                    await page._client.send('Runtime.evaluate', {'expression': setup.read_text(), 'awaitPromise': True, 'returnByValue': True})
                result = await page._client.send('Runtime.evaluate', {'expression': fixture.read_text(), 'awaitPromise': True, 'returnByValue': True})
                metadata = await capture_metadata(endpoint, browser, page, mode='headful', environment_profile_id=default_profile_id('headful'))
                output = {'captureMetadata': metadata, 'fixtureOrigin': f'http://127.0.0.1:{server.server_port}', 'result': result}
                fixture.with_name(name + '_chrome152.json').write_text(json.dumps(output, indent=2) + '\n')
                print(name, json.dumps(result))
            finally:
                await page.close()
    finally:
        await browser.disconnect()
        server.shutdown()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--chrome-cdp', required=True)
    parser.add_argument('--probes', nargs='+', default=['navigation_realm_ownership', 'navigation_realm_lifecycle'], choices=['navigation_realm_ownership', 'navigation_realm_lifecycle', 'navigation_window_edges', 'navigation_nested_window', 'navigation_cross_window_reflection'])
    args = parser.parse_args()
    asyncio.run(main(args.chrome_cdp, args.probes))
