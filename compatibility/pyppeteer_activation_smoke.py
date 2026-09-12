"""Local end-to-end regression: XPath, CSP waits, real hit testing and dialogs.

Run against an already started Mimic: python compatibility/pyppeteer_activation_smoke.py
No external websites, accounts, screenshots or synthetic HTMLElement.click calls.
"""
import argparse
import asyncio
import time

from aiohttp import ClientSession, web
from pyppeteer import connect


HTML = """<!doctype html><body>
<div hidden><button data-action="open">Hidden duplicate</button></div>
<section style="position:relative;z-index:3"><button id="open" data-action="open" command="show-modal" commandfor="auth">Sign in</button></section>
<dialog id="auth"><h1>Sign in</h1><input type="email"></dialog>
<dialog><select style="font-size:2ex"><option>Unmeasurable hidden control</option></select></dialog>
<script type="module" src="/entry.js"></script>
"""
MODULE = """
if(import.meta.resolve('./unused.js')!==new URL('./unused.js',import.meta.url).href)throw Error('module URL');
try{new Function('return 1')();window.authorBlocked=false}catch(e){window.authorBlocked=e.name==='EvalError'}
window.clickTargets=[];document.addEventListener('click',e=>clickTargets.push([e.target.id,e.isTrusted]));
"""


async def run(endpoint):
    async def serve(request):
        return web.Response(
            text=MODULE if request.path == "/entry.js" else HTML,
            content_type="text/javascript" if request.path == "/entry.js" else "text/html",
            headers={"Content-Security-Policy": "script-src 'self'"},
        )

    app = web.Application()
    app.router.add_get("/{path:.*}", serve)
    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, "127.0.0.1", 0)
    await site.start()
    port = runner.addresses[0][1]
    browser = page = None
    started = time.perf_counter()
    try:
        async with ClientSession() as session:
            async with session.get(endpoint + "/json/version") as response:
                info = await response.json()
        browser = await connect(browserWSEndpoint=info["webSocketDebuggerUrl"], defaultViewport=None)
        page = await browser.newPage()
        await page.goto(f"http://127.0.0.1:{port}/", {"waitUntil": "domcontentloaded", "timeout": 10000})
        assert await page.evaluate("window.authorBlocked") is True
        xpath = await page.waitForXPath('//button[@id="open"]', {"timeout": 3000})
        await xpath.dispose()
        button = await page.waitForFunction("""() => Array.from(document.querySelectorAll('[data-action=open]')).find(e => {
            const r=e.getBoundingClientRect();return r.width>0&&r.height>0;
        })""", {"timeout": 3000})
        await button.asElement().click()
        await page.waitForFunction("document.getElementById('auth').matches(':modal')", {"timeout": 3000})
        assert await page.evaluate("clickTargets") == [["open", True]]

        # Reading a node's quad must not force subsequent input to that node.
        # An overlay installed afterwards must receive the real pointer click.
        await page.evaluate("""() => {
            document.getElementById('auth').close();
            const cover=document.createElement('div');cover.id='cover';
            cover.style.cssText='position:fixed;left:0;top:0;width:1000px;height:1000px;z-index:999';
            document.body.append(cover);
        }""")
        await button.asElement().click()
        assert await page.evaluate("document.getElementById('auth').open") is False
        assert await page.evaluate("clickTargets[clickTargets.length-1]") == ["cover", True]
        assert await page.evaluate("""() => new Promise(resolve => {
            setTimeout(()=>{throw new Error('independent task sentinel')},0);
            setTimeout(()=>resolve(19),5);
        })""") == 19
        print(f"PASS: XPath, scoped CSP, visible button, modal and occlusion ({time.perf_counter()-started:.3f}s)")
    finally:
        if page is not None:
            await page.close()
        if browser is not None:
            await browser.disconnect()
        await runner.cleanup()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", default="http://127.0.0.1:9223")
    asyncio.run(run(parser.parse_args().endpoint.rstrip("/")))
