"""Capture a small, comparable semantic snapshot immediately before Voxel's first /fo request.

This is benchmark tooling only. It is never loaded by Mimic's runtime and does
not alter runtime behavior based on an origin or challenge implementation.
"""

import argparse
import asyncio
import json

from pyppeteer import connect


MARKER = "__MIMIC_PREFO_SNAPSHOT__"

INSTRUMENT = r"""() => {
  const state = {readyStates: [], readyStateSamples: [], candidates: [], performanceCalls: [], peerConfigurations: [], xhrHeaders: [], workerOps: [], gpu: {called: false, settled: false, fulfilled: false, valueTag: ''}};
  Object.defineProperty(globalThis, '__mimicSemanticProbe', {value: state, configurable: true});
  const debug = console.debug.bind(console);
  const ready = Object.getOwnPropertyDescriptor(Document.prototype, 'readyState');
  if (ready && ready.get) Object.defineProperty(Document.prototype, 'readyState', {
    ...ready,
    get() {
      const value = ready.get.call(this);
      state.readyStates.push(value);
      state.readyStateSamples.push({value, now: performance.now(), currentScript: document.currentScript && document.currentScript.src || ''});
      return value;
    }
  });
  const getEntries = Performance.prototype.getEntries;
  Performance.prototype.getEntries = function(...args) {
    const entries = getEntries.apply(this, args);
    state.performanceCalls.push({now: performance.now(), entries: entries.map(entry => ({
      name: entry.name, entryType: entry.entryType, startTime: entry.startTime,
      duration: entry.duration, requestStart: entry.requestStart || 0,
      responseStart: entry.responseStart || 0, responseEnd: entry.responseEnd || 0,
      transferSize: entry.transferSize || 0, encodedBodySize: entry.encodedBodySize || 0,
      nextHopProtocol: entry.nextHopProtocol || ''
    }))});
    return entries;
  };
  const add = RTCPeerConnection.prototype.addEventListener;
  RTCPeerConnection.prototype.addEventListener = function(type, listener, options) {
    if (type !== 'icecandidate' || typeof listener !== 'function') return add.call(this, type, listener, options);
    return add.call(this, type, function(event) {
      state.candidates.push({now: performance.now(), candidate: event.candidate ? event.candidate.candidate : null});
      return listener.call(this, event);
    }, options);
  };
  const NativePeerConnection = RTCPeerConnection;
  globalThis.RTCPeerConnection = new Proxy(NativePeerConnection, {
    construct(target, args, newTarget) {
      state.peerConfigurations.push(args.length ? args[0] : null);
      return Reflect.construct(target, args, newTarget);
    }
  });
  if (globalThis.webkitRTCPeerConnection === NativePeerConnection) globalThis.webkitRTCPeerConnection = globalThis.RTCPeerConnection;
  if (globalThis.GPU && GPU.prototype && typeof GPU.prototype.requestAdapter === 'function') {
    const requestAdapter = GPU.prototype.requestAdapter;
    GPU.prototype.requestAdapter = function(...args) {
      state.gpu.called = true;
      const result = requestAdapter.apply(this, args);
      Promise.resolve(result).then(value => {
        state.gpu.settled = true; state.gpu.fulfilled = true;
        state.gpu.valueTag = Object.prototype.toString.call(value);
      }, error => {
        state.gpu.settled = true; state.gpu.fulfilled = false;
        state.gpu.valueTag = Object.prototype.toString.call(error);
      });
      return result;
    };
  }
  if (globalThis.Worker && Worker.prototype) {
    const workerPostMessage = Worker.prototype.postMessage;
    const workerTerminate = Worker.prototype.terminate;
    Worker.prototype.postMessage = function(message, ...rest) {
      let tag = '', length = null;
      try {
        tag = Object.prototype.toString.call(message);
        if (typeof message === 'string' || ArrayBuffer.isView(message) || message instanceof ArrayBuffer) length = message.length ?? message.byteLength;
      } catch (_) {}
      state.workerOps.push({op: 'postMessage', now: performance.now(), tag, length});
      return workerPostMessage.call(this, message, ...rest);
    };
    Worker.prototype.terminate = function(...args) {
      state.workerOps.push({op: 'terminate', now: performance.now()});
      return workerTerminate.apply(this, args);
    };
    const workerAddEventListener = Worker.prototype.addEventListener;
    Worker.prototype.addEventListener = function(type, listener, options) {
      if (type === 'message' || type === 'messageerror' || type === 'error')
        state.workerOps.push({op: 'addEventListener', type, now: performance.now()});
      return workerAddEventListener.call(this, type, listener, options);
    };
  }
  const urls = new WeakMap(), open = XMLHttpRequest.prototype.open, setRequestHeader = XMLHttpRequest.prototype.setRequestHeader, send = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function(method, url, ...rest) {
    urls.set(this, String(url));
    return open.call(this, method, url, ...rest);
  };
  XMLHttpRequest.prototype.setRequestHeader = function(name, value) {
    state.xhrHeaders.push({url: urls.get(this) || '', name: String(name), value: String(value)});
    return setRequestHeader.call(this, name, value);
  };
  XMLHttpRequest.prototype.send = function(body) {
    const url = urls.get(this) || '';
    if (url.includes('/fo/')) {
      const summarizeOption = value => {
        const type = typeof value;
        if (value === null || type === 'string' || type === 'number' || type === 'boolean' || type === 'undefined')
          return {type, value: value === undefined ? '__undefined__' : value};
        if (Array.isArray(value)) return {type: 'array', length: value.length, value: value.length <= 16 ? value : undefined};
        return {type, tag: Object.prototype.toString.call(value), constructor: value && value.constructor && value.constructor.name || ''};
      };
      const optionState = {};
      if (globalThis._cf_chl_opt) for (const key of Object.keys(globalThis._cf_chl_opt).sort()) {
        try { optionState[key] = summarizeOption(globalThis._cf_chl_opt[key]); }
        catch (error) { optionState[key] = {error: String(error)}; }
      }
      const snapshot = {
        ...state,
        optionState,
        performanceData: globalThis._cf_chl_opt && Array.isArray(globalThis._cf_chl_opt.rPXg2) ? globalThis._cf_chl_opt.rPXg2 : null,
        url,
        bodyLength: body == null ? 0 : String(body).length,
        readyState: document.readyState,
        cookie: document.cookie,
        ua: navigator.userAgent,
        languages: Array.from(navigator.languages),
        hardwareConcurrency: navigator.hardwareConcurrency,
        deviceMemory: navigator.deviceMemory,
        screen: {width: screen.width, height: screen.height, availWidth: screen.availWidth, availHeight: screen.availHeight, colorDepth: screen.colorDepth},
        viewport: {innerWidth, innerHeight, outerWidth, outerHeight, devicePixelRatio},
        now: performance.now(), timeOrigin: performance.timeOrigin
      };
      debug('""" + MARKER + r"""' + JSON.stringify(snapshot));
    }
    return send.call(this, body);
  };
}"""


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", default="https://voxel.shop/")
    parser.add_argument("--timeout", type=int, default=60_000)
    parser.add_argument("--settle-ms", type=int, default=3_000)
    parser.add_argument("--clear-state", action="store_true")
    parser.add_argument("--compact", action="store_true")
    parser.add_argument("--session-only", action="store_true")
    args = parser.parse_args()

    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    snapshots = []
    fo_responses = []
    client_requests = []
    client_responses = []
    worker_response_ids = []

    if args.clear_state:
        await page._client.send("Network.clearBrowserCookies")
        await page._client.send("Network.clearBrowserCache")

    def console(message):
        text = message.text
        if text.startswith(MARKER):
            snapshots.append(json.loads(text[len(MARKER):]))

    page.on("console", console)
    def request_seen(request):
        if request.url == args.url or "/fo/" in request.url or "/eb/" in request.url:
            client_requests.append({"url": request.url, "method": request.method, "headers": request.headers})

    def response_seen(response):
        if "/fo/" in response.url:
            fo_responses.append({"url": response.url, "status": response.status})
        if response.url == args.url or "/fo/" in response.url or "/eb/" in response.url:
            client_responses.append({"url": response.url, "status": response.status, "headers": response.headers})

    page.on("request", request_seen)
    page.on("response", response_seen)
    page._client.on("Network.responseReceived", lambda event: worker_response_ids.append((event["requestId"], event["response"]["url"])) if event.get("response", {}).get("url", "").startswith("blob:") else None)
    await page.evaluateOnNewDocument(INSTRUMENT)
    try:
        await page.goto(args.url, {"waitUntil": "load", "timeout": args.timeout})
        await asyncio.sleep(args.settle_ms / 1000)
    finally:
        worker_trace, network_trace, execution_trace = [], [], []
        browser_cookies = []
        worker_sources = []
        try:
            final_state = await page.evaluate("() => ({url: location.href, title: document.title, bodyLength: document.body ? document.body.innerHTML.length : 0, cookie: document.cookie, workerOps: globalThis.__mimicSemanticProbe ? globalThis.__mimicSemanticProbe.workerOps : []})")
        except Exception as error:
            final_state = {"error": str(error)}
        try:
            events = (await page._client.send("Mimic.getTrace")).get("events", [])
            worker_trace = [event for event in events if event.get("data", {}).get("worker") is not None or event.get("kind") in {"error", "exception", "semantic-missing", "surface-missing", "unsupported"}][-80:]
            network_trace = [event for event in events if event.get("kind") == "network" and event.get("name") in {"request", "response", "transport", "criticalClientHintsRestart"}]
            execution_trace = [event for event in events if event.get("kind") in {"js", "scheduler", "error", "exception", "semantic-missing", "surface-missing", "unsupported"}][-160:]
        except Exception:
            pass
        try:
            browser_cookies = (await page._client.send("Network.getAllCookies")).get("cookies", [])
        except Exception:
            pass
        for request_id, resource_url in worker_response_ids:
            try:
                body = (await page._client.send("Network.getResponseBody", {"requestId": request_id})).get("body", "")
                worker_sources.append({"url": resource_url, "length": len(body), "source": body[:4096]})
            except Exception as error:
                worker_sources.append({"url": resource_url, "error": str(error)})
        result = {"endpoint": args.endpoint, "finalState": final_state, "cookies": browser_cookies, "snapshots": snapshots, "foResponses": fo_responses, "clientRequests": client_requests, "clientResponses": client_responses, "workerSources": worker_sources, "workerTrace": worker_trace, "networkTrace": network_trace}
        if args.compact:
            result["snapshots"] = [{
                key: snapshot.get(key) for key in (
                    "bodyLength", "readyState", "ua", "languages", "hardwareConcurrency",
                    "deviceMemory", "screen", "viewport", "gpu", "performanceData",
                    "optionState",
                )
            } | {
                "candidateCount": len(snapshot.get("candidates", [])),
                "peerConfigurations": snapshot.get("peerConfigurations", []),
                "xhrHeaders": snapshot.get("xhrHeaders", []),
            } for snapshot in snapshots]
            result["workerTrace"] = [event for event in worker_trace if event.get("kind") in {
                "error", "exception", "semantic-missing", "surface-missing", "unsupported"
            }]
            result["workerTimeline"] = [{
                "sequence": event.get("sequence"),
                "kind": event.get("kind"),
                "name": event.get("name"),
                "worker": event.get("data", {}).get("worker"),
                "direction": event.get("data", {}).get("direction"),
                "source": event.get("data", {}).get("source"),
                "error": event.get("data", {}).get("error"),
            } for event in worker_trace[-120:]]
            result["executionTimeline"] = [{
                "sequence": event.get("sequence"),
                "kind": event.get("kind"),
                "name": event.get("name"),
                "source": event.get("data", {}).get("source"),
                "worker": event.get("data", {}).get("worker"),
                "direction": event.get("data", {}).get("direction"),
                "error": event.get("data", {}).get("error"),
            } for event in execution_trace]
            result["networkTrace"] = [{
                "name": event.get("name"),
                "status": event.get("data", {}).get("status"),
                "url": event.get("data", {}).get("url"),
                "protocol": event.get("data", {}).get("protocol"),
                "fromCache": event.get("data", {}).get("fromCache"),
                "cookie": next((value for name, value in event.get("data", {}).get("headers", {}).items() if name.lower() == "cookie"), None),
                "setCookie": next((value for name, value in event.get("data", {}).get("headers", {}).items() if name.lower() == "set-cookie"), None),
                "sessionCold": event.get("data", {}).get("sessionCold"),
                "connectionState": event.get("data", {}).get("connectionState"),
            } for event in network_trace]
        if args.session_only:
            result = {key: result[key] for key in ("endpoint", "finalState", "cookies", "foResponses", "clientRequests", "clientResponses", "workerSources", "workerTimeline", "executionTimeline", "networkTrace")}
        print(json.dumps(result, ensure_ascii=False, indent=2))
        await page.close()
        await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
