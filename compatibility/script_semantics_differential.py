"""Independent Chrome 152 ↔ Mimic probe for HTML script execution semantics."""

import argparse
import asyncio
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from pyppeteer import connect


HTML = b"""<!doctype html><body>
<script>window.order=['classic-before']</script>
<script type="application/json">{"mustNotExecute":true}</script>
<script type="application/ld+json">{"alsoMustNotExecute":true}</script>
<script type="text/plain">throw new Error('plain script executed')</script>
<script type="module" src="/entry.js"></script>
<script>window.order.push('classic-after')</script>
</body>"""

RESOURCES = {
    "/": ("text/html; charset=utf-8", HTML),
    "/entry.js": (
        "text/javascript; charset=utf-8",
        b"import {answer} from './dep.js'; import {instance,count} from './shared.js'; window.staticInstance=instance; order.push('module:'+answer); window.moduleDone=true; Promise.all([import('./dynamic.js'),import('./shared.js')]).then(([{value,dynamicInstance,dynamicCount},direct])=>{order.push('dynamic:'+value);window.dynamicInstanceSame=dynamicInstance===staticInstance;window.directInstanceSame=direct.instance===staticInstance;window.sharedEvaluationCount=dynamicCount;window.directEvaluationCount=direct.count;window.dynamicDone=true}).catch(error=>{window.dynamicError=String(error)});setTimeout(()=>import('./delayed.js').then(({value})=>{window.delayedValue=value;window.delayedDone=true}).catch(error=>{window.delayedError=String(error)}),50)",
    ),
    "/dep.js": ("text/javascript; charset=utf-8", b"export const answer=42"),
    "/shared.js": ("text/javascript; charset=utf-8", b"globalThis.sharedModuleEvaluations=(globalThis.sharedModuleEvaluations||0)+1;export const instance={};export const count=globalThis.sharedModuleEvaluations"),
    "/dynamic.js": ("text/javascript; charset=utf-8", b"import {instance,count} from './shared.js';export const value=84;export const dynamicInstance=instance;export const dynamicCount=count"),
    "/delayed.js": ("text/javascript; charset=utf-8", b"export const value=168"),
}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        content_type, body = RESOURCES.get(self.path, ("text/plain", b"missing"))
        self.send_response(200 if self.path in RESOURCES else 404)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args):
        pass


async def observe(endpoint, url):
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    exceptions = []
    page.on("pageerror", lambda error: exceptions.append(str(error)))
    await page.goto(url, {"waitUntil": "load", "timeout": 30_000})
    await asyncio.sleep(1)
    value = await page.evaluate(
        "() => ({order:window.order,moduleDone:window.moduleDone===true,dynamicDone:window.dynamicDone===true,dynamicError:window.dynamicError,delayedDone:window.delayedDone===true,delayedValue:window.delayedValue,delayedError:window.delayedError,dynamicInstanceSame:window.dynamicInstanceSame,directInstanceSame:window.directInstanceSame,sharedEvaluationCount:window.sharedEvaluationCount,directEvaluationCount:window.directEvaluationCount," 
        "types:[...document.scripts].map(script=>script.type)})"
    )
    trace = {"events": []}
    try:
        trace = await page._client.send("Mimic.getTrace")
    except Exception:
        pass
    await page.close()
    await browser.disconnect()
    return {
        "value": value,
        "exceptions": exceptions,
        "runtimeIssues": [
            event for event in trace["events"]
            if event.get("kind") in {"exception", "error", "unsupported", "semantic-missing", "surface-missing"}
        ],
    }


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--mimic", default="http://127.0.0.1:9222")
    args = parser.parse_args()
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    url = f"http://127.0.0.1:{server.server_port}/"
    try:
        chrome = await observe(args.chrome, url)
        mimic = await observe(args.mimic, url)
        print(json.dumps({"url": url, "chrome": chrome, "mimic": mimic}, indent=2))
    finally:
        server.shutdown()


if __name__ == "__main__":
    asyncio.run(main())
