"""Compare the generic child browsing-context path in Chrome 152 and Mimic.

The probe owns its HTTP origin and records both page-observable ordering and
the raw CDP frame/lifecycle/context stream.  Volatile protocol identifiers are
normalized before the two captures are compared.
"""

import argparse
import asyncio
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading
from urllib.parse import parse_qs, urlsplit

from pyppeteer import connect


PARENT = r"""<!doctype html>
<meta charset="utf-8">
<title>frame differential parent</title>
<script>
  const timeline = [];
  let sequence = 0;
  let dynamicFrame = null;
  let firstDynamicWindow = null;
  let firstDynamicArray = null;
  let dynamicLoads = 0;
  globalThis.__frameProbeResult = null;
  const finish = value => { globalThis.__frameProbeResult = value; };

  function record(label, detail = {}) {
    timeline.push(Object.assign({sequence: ++sequence, label}, detail));
  }

  addEventListener('message', event => {
    const parser = document.getElementById('parser');
    let source = 'other';
    if (parser && event.source === parser.contentWindow) source = 'parser';
    if (dynamicFrame && event.source === dynamicFrame.contentWindow) source = 'dynamic';
    record('message:' + String(event.data && event.data.phase), {
      readyState: document.readyState,
      origin: event.origin,
      source,
      ports: event.ports.length,
      dataRealm: event.data && Object.getPrototypeOf(event.data) === Object.prototype,
      initRealm: event.data && event.data.initRealm,
      initReadyState: event.data && event.data.initReadyState,
      initArrayLocal: event.data && event.data.initArrayLocal
    });
    if (String(event.data && event.data.phase).endsWith('-port') && event.ports[0]) {
      event.ports[0].postMessage({ping: event.data.phase});
    }
  });

  document.addEventListener('DOMContentLoaded', () => {
    record('parent-dom-content-loaded', {readyState: document.readyState});
    queueMicrotask(() => record('parent-dom-content-loaded-microtask', {readyState: document.readyState}));
    dynamicFrame = document.createElement('iframe');
    dynamicFrame.id = 'dynamic';
    dynamicFrame.src = '/child?kind=dynamic-one';
    dynamicFrame.addEventListener('load', () => {
      dynamicLoads++;
      record('dynamic-owner-load-' + dynamicLoads, {
        childReadyState: dynamicFrame.contentDocument && dynamicFrame.contentDocument.readyState,
        childTitle: dynamicFrame.contentDocument && dynamicFrame.contentDocument.title
      });
      if (dynamicLoads === 1) {
        firstDynamicWindow = dynamicFrame.contentWindow;
        firstDynamicArray = firstDynamicWindow.Array;
        record('dynamic-first-identities', {
          parent: firstDynamicWindow.parent === window,
          top: firstDynamicWindow.top === window,
          localArray: firstDynamicArray !== Array,
          contentDocument: dynamicFrame.contentDocument === firstDynamicWindow.document
        });
        dynamicFrame.src = '/child?kind=dynamic-two';
        return;
      }
      record('dynamic-second-identities', {
        stableWindowProxy: firstDynamicWindow === dynamicFrame.contentWindow,
        freshArray: firstDynamicArray !== dynamicFrame.contentWindow.Array,
        parent: dynamicFrame.contentWindow.parent === window,
        top: dynamicFrame.contentWindow.top === window,
        contentDocument: dynamicFrame.contentDocument === dynamicFrame.contentWindow.document
      });
      const retainedWindow = dynamicFrame.contentWindow;
      record('dynamic-before-remove', {
        connected: dynamicFrame.isConnected,
        contentWindow: !!dynamicFrame.contentWindow
      });
      dynamicFrame.remove();
      record('dynamic-removed', {
        contentWindowNull: dynamicFrame.contentWindow === null,
        contentDocumentNull: dynamicFrame.contentDocument === null,
        retainedArrayType: typeof retainedWindow.Array
      });
      setTimeout(() => finish({
        timeline,
        parser: {
          contentWindow: !!document.getElementById('parser').contentWindow,
          contentDocument: !!document.getElementById('parser').contentDocument,
          parent: document.getElementById('parser').contentWindow.parent === window,
          top: document.getElementById('parser').contentWindow.top === window,
          localArray: document.getElementById('parser').contentWindow.Array !== Array,
          title: document.getElementById('parser').contentDocument.title
        },
        parentInit: {
          realm: globalThis.__frameProbeInit && globalThis.__frameProbeInit.realm,
          readyState: globalThis.__frameProbeInit && globalThis.__frameProbeInit.readyState,
          arrayLocal: globalThis.__frameProbeInit && globalThis.__frameProbeInit.array === Array
        },
        performance: performance.getEntriesByType('resource').map(entry => ({
          path: new URL(entry.name).pathname + new URL(entry.name).search,
          initiatorType: entry.initiatorType
        })).filter(entry => entry.path.startsWith('/child') || entry.path.startsWith('/child-script'))
      }), 25);
    });
    document.body.appendChild(dynamicFrame);
    record('dynamic-appended', {hasWindow: !!dynamicFrame.contentWindow, hasDocument: !!dynamicFrame.contentDocument});
  });

  addEventListener('load', () => record('parent-load', {
    readyState: document.readyState,
    dynamicPresent: !!dynamicFrame,
    dynamicConnected: !!(dynamicFrame && dynamicFrame.isConnected),
    dynamicContentWindow: !!(dynamicFrame && dynamicFrame.contentWindow)
  }));
</script>
<iframe id="parser" src="/child?kind=parser"></iframe>
<script>
  record('parent-parser-after-iframe', {readyState: document.readyState});
  Promise.resolve().then(() => record('parent-parser-microtask', {readyState: document.readyState}));
</script>
"""


CHILD = r"""<!doctype html>
<meta charset="utf-8">
<title>child-{kind}</title>
<script>
  const childKind = {kind_json};
  const send = phase => parent.postMessage({{
    phase: childKind + '-' + phase,
    initRealm: globalThis.__frameProbeInit && globalThis.__frameProbeInit.realm,
    initReadyState: globalThis.__frameProbeInit && globalThis.__frameProbeInit.readyState,
    initArrayLocal: globalThis.__frameProbeInit && globalThis.__frameProbeInit.array === Array
  }}, '*');
  send('inline');
  Promise.resolve().then(() => send('inline-microtask'));
  document.addEventListener('DOMContentLoaded', () => {{
    send('dom-content-loaded');
    queueMicrotask(() => send('dom-content-loaded-microtask'));
  }});
  addEventListener('load', () => send('window-load'));
</script>
<script src="/child-script?kind={kind}"></script>
<body>{kind}</body>
"""


CHILD_SCRIPT = r"""
parent.postMessage({phase: childKind + '-external'}, '*');
Promise.resolve().then(() => parent.postMessage({phase: childKind + '-external-microtask'}, '*'));
const channel = new MessageChannel();
channel.port1.onmessage = event => parent.postMessage({phase: childKind + '-port-reply', portData: event.data && event.data.ping}, '*');
parent.postMessage({phase: childKind + '-port'}, '*', [channel.port2]);
"""


class ProbeHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlsplit(self.path)
        query = parse_qs(parsed.query)
        kind = query.get("kind", [""])[0]
        if parsed.path in ("/", "/parent"):
            body = PARENT.encode()
        elif parsed.path == "/child":
            body = CHILD.format(kind=kind, kind_json=json.dumps(kind)).encode()
        elif parsed.path == "/child-script":
            body = CHILD_SCRIPT.encode()
        elif parsed.path == "/favicon.ico":
            body = b""
        else:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/javascript" if parsed.path == "/child-script" else "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, _format, *_args):
        pass


@contextmanager
def probe_server():
    server = ThreadingHTTPServer(("127.0.0.1", 0), ProbeHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/parent"
    finally:
        server.shutdown()
        thread.join()


def normalizer():
    frame_names = {}
    context_names = {}
    next_frame = 0
    next_context = 0

    def frame_name(frame_id):
        nonlocal next_frame
        if not frame_id:
            return ""
        if frame_id not in frame_names:
            next_frame += 1
            frame_names[frame_id] = "frame-" + str(next_frame)
        return frame_names[frame_id]

    def context_name(context_id):
        nonlocal next_context
        if context_id is None:
            return ""
        if context_id not in context_names:
            next_context += 1
            context_names[context_id] = "context-" + str(next_context)
        return context_names[context_id]

    def normalize(method, params):
        if method == "Page.frameAttached":
            return {"method": method, "frame": frame_name(params.get("frameId")), "parent": frame_name(params.get("parentFrameId"))}
        if method == "Page.frameNavigated":
            frame = params.get("frame", {})
            parsed = urlsplit(frame.get("url", ""))
            return {
                "method": method,
                "frame": frame_name(frame.get("id")),
                "parent": frame_name(frame.get("parentId")),
                "path": parsed.path + (("?" + parsed.query) if parsed.query else ""),
                "origin": frame.get("securityOrigin", ""),
                "mimeType": frame.get("mimeType", ""),
            }
        if method == "Page.frameDetached":
            return {"method": method, "frame": frame_name(params.get("frameId")), "reason": params.get("reason", "")}
        if method == "Page.lifecycleEvent":
            return {"method": method, "frame": frame_name(params.get("frameId")), "name": params.get("name", "")}
        if method == "Runtime.executionContextCreated":
            context = params.get("context", {})
            aux = context.get("auxData", {})
            return {
                "method": method,
                "context": context_name(context.get("id")),
                "frame": frame_name(aux.get("frameId")),
                "origin": context.get("origin", ""),
                "default": aux.get("isDefault"),
                "type": aux.get("type", ""),
            }
        if method == "Runtime.executionContextDestroyed":
            return {"method": method, "context": context_name(params.get("executionContextId"))}
        if method == "Network.requestWillBeSent":
            request = params.get("request", {})
            parsed = urlsplit(request.get("url", ""))
            return {
                "method": method,
                "frame": frame_name(params.get("frameId")),
                "path": parsed.path + (("?" + parsed.query) if parsed.query else ""),
                "type": params.get("type", ""),
                "documentLoader": params.get("requestId") == params.get("loaderId"),
            }
        if method == "Network.responseReceived":
            response = params.get("response", {})
            parsed = urlsplit(response.get("url", ""))
            return {
                "method": method,
                "frame": frame_name(params.get("frameId")),
                "path": parsed.path + (("?" + parsed.query) if parsed.query else ""),
                "type": params.get("type", ""),
            }
        return {"method": method}

    return normalize


async def capture(endpoint, url, timeout):
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    client = page._client
    normalize = normalizer()
    protocol = []
    methods = [
        "Page.frameAttached", "Page.frameNavigated", "Page.frameDetached",
        "Page.lifecycleEvent", "Page.domContentEventFired", "Page.loadEventFired",
        "Runtime.executionContextCreated", "Runtime.executionContextDestroyed",
        "Runtime.executionContextsCleared",
        "Network.requestWillBeSent", "Network.responseReceived",
    ]
    for method in methods:
        client.on(method, lambda params, method=method: protocol.append(normalize(method, params)))
    await client.send("Page.setLifecycleEventsEnabled", {"enabled": True})
    await client.send("Page.addScriptToEvaluateOnNewDocument", {"source": "globalThis.__frameProbeInit={realm:parent===window?'top':'child',readyState:document.readyState,array:Array}"})
    navigation_error = ""
    try:
        await page.goto(url, {"waitUntil": "load", "timeout": int(timeout * 1000)})
    except Exception as exc:
        navigation_error = str(exc)
    deadline = asyncio.get_running_loop().time() + timeout
    value = None
    partial = None
    while asyncio.get_running_loop().time() < deadline:
        state = await page.evaluate(
            "({result:globalThis.__frameProbeResult||null,timeline:typeof timeline==='undefined'?[]:timeline})",
            force_expr=True,
        )
        value = state.get("result")
        partial = state.get("timeline")
        if value is not None:
            break
        await asyncio.sleep(0.05)
    tree = await client.send("Page.getFrameTree")
    mimic_trace = []
    try:
        events = (await client.send("Mimic.getTrace")).get("events", [])
        mimic_trace = [
            {"sequence": event.get("sequence"), "kind": event.get("kind"), "name": event.get("name"), "data": event.get("data", {})}
            for event in events
            if event.get("name") in ("frameMessagePosted", "messagePortMessagePosted")
            or event.get("kind") in ("error", "exception")
        ]
    except Exception:
        pass
    error = "" if value is not None else (navigation_error or "probe timed out")
    await page.close()
    await browser.disconnect()
    return {"value": value, "partialTimeline": partial if error else [], "error": error, "protocol": protocol, "frameTree": normalize_tree(tree.get("frameTree", {})), "mimicTrace": mimic_trace}


def normalize_tree(tree):
    frame = tree.get("frame", {})
    parsed = urlsplit(frame.get("url", ""))
    result = {"path": parsed.path + (("?" + parsed.query) if parsed.query else "")}
    children = tree.get("childFrames", [])
    if children:
        result["children"] = [normalize_tree(child) for child in children]
    return result


async def run(args):
    with probe_server() as url:
        chrome, mimic = await asyncio.gather(
            capture(args.chrome, url, args.timeout),
            capture(args.mimic, url, args.timeout),
        )
    if args.semantic_only:
        chrome_timeline = semantic_timeline(chrome)
        mimic_timeline = semantic_timeline(mimic)
        first = next((
            index for index in range(max(len(chrome_timeline), len(mimic_timeline)))
            if index >= len(chrome_timeline) or index >= len(mimic_timeline)
            or chrome_timeline[index] != mimic_timeline[index]
        ), None)
        result = {
            "match": chrome_timeline == mimic_timeline,
            "firstDivergence": None if first is None else {
                "index": first,
                "chrome": chrome_timeline[first] if first < len(chrome_timeline) else None,
                "mimic": mimic_timeline[first] if first < len(mimic_timeline) else None,
            },
            "chromeTimeline": chrome_timeline,
            "mimicTimeline": mimic_timeline,
            "chromeError": chrome["error"],
            "mimicError": mimic["error"],
        }
    else:
        result = {"match": chrome == mimic, "chrome": chrome, "mimic": mimic}
    print(json.dumps(result, ensure_ascii=False, indent=2))


def semantic_timeline(capture):
    value = capture.get("value") or {}
    timeline = []
    for item in value.get("timeline", []):
        normalized = {key: value for key, value in item.items() if key not in {"sequence", "origin"}}
        timeline.append(normalized)
    return timeline


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--mimic", default="http://127.0.0.1:19222")
    parser.add_argument("--timeout", type=float, default=15)
    parser.add_argument("--semantic-only", action="store_true")
    args = parser.parse_args()
    asyncio.run(run(args))
