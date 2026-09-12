"""Run the sandboxed preview DOM regression in an owned Chrome 152 context."""
import argparse
import asyncio
import functools
import http.server
from pathlib import Path
import threading

import aiohttp
from pyppeteer import connect

async def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--chrome',type=int,default=9351)
    parser.add_argument('--preview-url')
    parser.add_argument('--screenshot',type=Path)
    args=parser.parse_args()
    class QuietHandler(http.server.SimpleHTTPRequestHandler):
        def log_message(self,*args):pass
    root=Path(__file__).resolve().parents[2]/'internal/cdp'
    server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(QuietHandler,directory=str(root)))
    threading.Thread(target=server.serve_forever,daemon=True).start()
    async with aiohttp.ClientSession() as client:
        async with client.get(f'http://127.0.0.1:{args.chrome}/json/version') as r:version=await r.json()
    if not version['Browser'].startswith('Chrome/152.'):
        raise RuntimeError('Expected frozen Chrome 152')
    browser=await connect(browserWSEndpoint=version['webSocketDebuggerUrl'],defaultViewport=None)
    context=await browser.createIncognitoBrowserContext()
    try:
        page=await context.newPage()
        await page.goto(f'http://127.0.0.1:{server.server_port}/testdata/dev_preview_dom_test.html')
        await page.waitForFunction("/^(PASS|FAIL):/.test(document.querySelector('#result').textContent)",dict(timeout=20000))
        result=await page.evaluate("document.querySelector('#result').textContent")
        print(result)
        if not result.startswith('PASS:'):raise AssertionError(result)
        if args.preview_url:
            await page.goto(args.preview_url)
            await page.waitForFunction("document.querySelector('#view')?.contentDocument?.querySelector('dialog:modal')",dict(timeout=20000))
            geometry=await page.evaluate("""()=>{const f=document.querySelector('#view'),d=f.contentDocument.querySelector('dialog:modal'),r=d.getBoundingClientRect();return {modal:d.matches(':modal'),open:d.open,x:r.x,y:r.y,right:r.right,bottom:r.bottom,width:r.width,height:r.height,viewport:[f.clientWidth,f.clientHeight]}}""")
            print(geometry)
            if geometry['x']<0 or geometry['y']<0 or geometry['right']>geometry['viewport'][0] or geometry['bottom']>geometry['viewport'][1]:
                raise AssertionError('Live preview modal is outside the viewport')
        if args.screenshot:await page.screenshot(dict(path=str(args.screenshot)))
    finally:
        await context.close()
        await browser.disconnect()
        server.shutdown()

if __name__=='__main__':asyncio.run(main())
