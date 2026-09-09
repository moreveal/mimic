"""Measure classic-script cleanup identity against pinned headful Chrome."""
import argparse
import asyncio
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from pyppeteer import connect
from oracle import capture_metadata, default_profile_id, prepare_page

DOCUMENT = '''<!doctype html><body><script id="entry">
window.observations=[];
window.record=label=>observations.push([label,document.currentScript?.id??null]);
record('sync');Promise.resolve().then(()=>record('then'));
(async()=>{await 0;record('await');await 0;record('await-again')})();
setTimeout(()=>record('timer'),0);
</script><script id="throwing">
Promise.resolve().then(()=>record('after-throw'));throw Error('intentional');
</script><script id="following">record('following');</script>
<script type="module">record('module');await 0;record('module-await');</script></body>'''

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        dynamic = self.path == '/dynamic.js'
        body = "record('dynamic');Promise.resolve().then(()=>record('dynamic-then'));" if dynamic else DOCUMENT
        self.send_response(200)
        self.send_header('Content-Type', 'text/javascript' if dynamic else 'text/html')
        self.end_headers()
        try:
            self.wfile.write(body.encode())
        except (BrokenPipeError, ConnectionAbortedError, ConnectionResetError):
            pass
    def log_message(self, *args):
        pass

async def observe(endpoint, url, chrome):
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    try:
        if chrome:
            await prepare_page(browser, page, 'headful', '1280x800')
        await page.goto(url, {'waitUntil': 'load'})
        await page.evaluate('''()=>new Promise(resolve=>{
            const s=document.createElement('script');s.id='dynamic';s.src='/dynamic.js';
            s.onload=()=>{record('load');resolve()};document.head.appendChild(s);
        })''')
        result = await page.evaluate('''()=>({observations,after:document.currentScript})''')
        if chrome:
            result['metadata'] = await capture_metadata(endpoint, browser, page,
                mode='headful', environment_profile_id=default_profile_id('headful'))
        return result
    finally:
        await page.close()
        await browser.disconnect()

async def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--chrome', required=True)
    parser.add_argument('--mimic', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        url = f'http://127.0.0.1:{server.server_port}/'
        results = {name: await observe(endpoint, url, name == 'chrome')
                   for name, endpoint in [('chrome', args.chrome), ('mimic', args.mimic)]}
        # Timer/network tasks need not have the same interleaving. Each script's
        # own cleanup identity and load ordering must agree.
        results['identityMatches'] = (dict(results['chrome']['observations']) ==
                                      dict(results['mimic']['observations']))
        Path(args.output).write_text(json.dumps(results, indent=2), encoding='utf8')
        print(json.dumps(results, indent=2))
        if not results['identityMatches']:
            raise SystemExit('currentScript identity differs')
    finally:
        server.shutdown()
        server.server_close()

if __name__ == '__main__':
    asyncio.run(main())
