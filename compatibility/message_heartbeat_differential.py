"""Compare a cross-origin frame message heartbeat in Chrome 152 and Mimic."""

import argparse
import asyncio
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading

from pyppeteer import connect


PARENT = r"""<!doctype html>
<meta charset="utf-8">
<body>
<script>
const timeline = [];
let order = 0;
const childOrigin = {child_origin};
const frame = document.createElement('iframe');
globalThis.__heartbeatResult = null;
const record = (phase, detail={{}}) => timeline.push(Object.assign({{order: ++order, phase}}, detail));

addEventListener('message', event => {{
  const data = event.data || {{}};
  record('parent-receive', {{
    message: data.phase,
    received: data.received || '',
    seq: data.seq === undefined ? null : data.seq,
    originOK: event.origin === childOrigin,
    sourceOK: event.source === frame.contentWindow,
    ports: event.ports.length,
    dataRealm: Object.getPrototypeOf(data) === Object.prototype
  }});
  queueMicrotask(() => record('parent-microtask', {{message: data.phase, seq: data.seq === undefined ? null : data.seq}}));
  if (data.phase === 'ready') {{
    frame.contentWindow.postMessage({{phase: 'config'}}, childOrigin);
    record('parent-post', {{message: 'config'}});
  }} else if (data.phase === 'heartbeat') {{
    frame.contentWindow.postMessage({{phase: 'heartbeat-reply', seq: data.seq}}, childOrigin);
    record('parent-post', {{message: 'heartbeat-reply', seq: data.seq}});
  }} else if (data.phase === 'done') {{
    setTimeout(() => {{ globalThis.__heartbeatResult = timeline; }}, 0);
  }}
}});

frame.src = {child_url};
document.body.appendChild(frame);
record('parent-appended');
</script>
"""


CHILD = r"""<!doctype html>
<meta charset="utf-8">
<script>
const parentOrigin = {parent_origin};
const send = (phase, detail={{}}) => parent.postMessage(Object.assign({{phase}}, detail), parentOrigin);
addEventListener('message', event => {{
  const data = event.data || {{}};
  send('child-receive', {{
    received: data.phase,
    seq: data.seq === undefined ? null : data.seq,
    originOK: event.origin === parentOrigin,
    sourceOK: event.source === parent,
    ports: event.ports.length,
    dataRealm: Object.getPrototypeOf(data) === Object.prototype
  }});
  queueMicrotask(() => send('child-microtask', {{received: data.phase, seq: data.seq === undefined ? null : data.seq}}));
  if (data.phase === 'config') {{
    setTimeout(() => send('heartbeat', {{seq: 0}}), 0);
  }} else if (data.phase === 'heartbeat-reply') {{
    setTimeout(() => data.seq < 2 ? send('heartbeat', {{seq: data.seq + 1}}) : send('done'), 0);
  }}
}});
send('ready');
Promise.resolve().then(() => send('ready-microtask'));
</script>
"""


class StaticHandler(BaseHTTPRequestHandler):
    body = b""

    def do_GET(self):
        body = type(self).body
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, _format, *_args):
        pass


def handler(body):
    return type("BoundStaticHandler", (StaticHandler,), {"body": body})


@contextmanager
def probe_servers():
    child = ThreadingHTTPServer(("127.0.0.1", 0), handler(b""))
    parent = ThreadingHTTPServer(("127.0.0.1", 0), handler(b""))
    child_origin = f"http://127.0.0.1:{child.server_port}"
    parent_origin = f"http://127.0.0.1:{parent.server_port}"
    child.RequestHandlerClass.body = CHILD.format(parent_origin=json.dumps(parent_origin)).encode()
    parent.RequestHandlerClass.body = PARENT.format(
        child_origin=json.dumps(child_origin), child_url=json.dumps(child_origin + "/child")
    ).encode()
    threads = [threading.Thread(target=server.serve_forever, daemon=True) for server in (child, parent)]
    for thread in threads:
        thread.start()
    try:
        yield parent_origin + "/parent"
    finally:
        for server in (child, parent):
            server.shutdown()
        for thread in threads:
            thread.join()


async def capture(endpoint, url, timeout):
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    error = ""
    try:
        await page.goto(url, {"waitUntil": "load", "timeout": int(timeout * 1000)})
    except Exception as exc:
        error = str(exc)
    deadline = asyncio.get_running_loop().time() + timeout
    timeline = None
    while asyncio.get_running_loop().time() < deadline:
        timeline = await page.evaluate("globalThis.__heartbeatResult || null", force_expr=True)
        if timeline is not None:
            break
        await asyncio.sleep(0.02)
    await page.close()
    await browser.disconnect()
    return {"error": error or ("probe timed out" if timeline is None else ""), "timeline": timeline or []}


def normalized_timeline(capture):
    return [{key: value for key, value in item.items() if key != "order"} for item in capture["timeline"]]


async def run(args):
    with probe_servers() as url:
        chrome, mimic = await asyncio.gather(
            capture(args.chrome, url, args.timeout), capture(args.mimic, url, args.timeout)
        )
    chrome_timeline = normalized_timeline(chrome)
    mimic_timeline = normalized_timeline(mimic)
    first = next((
        index for index in range(max(len(chrome_timeline), len(mimic_timeline)))
        if index >= len(chrome_timeline) or index >= len(mimic_timeline)
        or chrome_timeline[index] != mimic_timeline[index]
    ), None)
    print(json.dumps({
        "match": chrome_timeline == mimic_timeline,
        "firstDivergence": None if first is None else {
            "index": first,
            "chrome": chrome_timeline[first] if first < len(chrome_timeline) else None,
            "mimic": mimic_timeline[first] if first < len(mimic_timeline) else None,
        },
        "chrome": chrome,
        "mimic": mimic,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--mimic", default="http://127.0.0.1:19222")
    parser.add_argument("--timeout", type=float, default=10)
    asyncio.run(run(parser.parse_args()))
