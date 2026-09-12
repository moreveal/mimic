"""Capture document-owned image reuse against pinned Chrome (or a Mimic binary)."""
import argparse
import asyncio
import hashlib
import http.server
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import urllib.request

from pyppeteer import connect

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / 'internal/browser/testdata/image_reuse_oracle.js'


async def run(args):
    if args.output.exists():
        raise FileExistsError(args.output)
    counts = {}

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200)
            if self.path.startswith('/image'):
                counts[self.path] = counts.get(self.path, 0) + 1
                body = b'<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>'
                self.send_header('Content-Type', 'image/svg+xml')
                self.send_header('Cache-Control', 'no-store' if 'nostore' in self.path else 'max-age=600')
            else:
                body = b'<html><body></body></html>'
                self.send_header('Content-Type', 'text/html')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    endpoint = f'http://127.0.0.1:{port}'
    binary = args.binary.resolve()
    with tempfile.TemporaryDirectory(prefix='mimic-image-oracle-') as profile:
        command = ([str(binary), '--headless=new', f'--remote-debugging-port={port}',
                    f'--user-data-dir={profile}', '--no-first-run', 'about:blank']
                   if binary.name == 'chrome.exe' else [str(binary), '-listen', f'127.0.0.1:{port}'])
        process = subprocess.Popen(command, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                   creationflags=subprocess.CREATE_NO_WINDOW if os.name == 'nt' else 0)
        browser = None
        try:
            for _ in range(100):
                try:
                    with urllib.request.urlopen(endpoint + '/json/version', timeout=.3) as response:
                        version = json.load(response)
                    break
                except OSError:
                    await asyncio.sleep(.1)
            browser = await connect(browserURL=endpoint, defaultViewport=None)
            page = await browser.newPage()
            await page.goto(f'http://127.0.0.1:{server.server_port}/')
            rows = []
            for control in ['cache', 'nostore']:
                for loading in ['eager', 'lazy']:
                    result = await page.evaluate(FIXTURE.read_text(), control, loading)
                    rows.append({'control': control, 'loading': loading, 'result': result,
                                 'requests': counts.get('/image?' + control + loading, 0)})
            evidence = {'version': version['Browser'], 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
                        'fixture_sha256': hashlib.sha256(FIXTURE.read_bytes()).hexdigest(), 'rows': rows}
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(json.dumps(evidence, indent=2) + '\n', encoding='utf-8')
            print(json.dumps(rows))
        finally:
            if browser:
                await browser.disconnect()
            process.terminate()
            process.wait(timeout=10)
            server.shutdown()
            server.server_close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    asyncio.run(run(parser.parse_args()))
