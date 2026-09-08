"""Trace Web-platform prototype access in the pinned differential Chrome.

This development-only probe instruments the effective API surface before page
scripts. It never ships in Mimic and is intentionally origin-agnostic.
"""

import argparse
import asyncio
import json
import urllib.request
from urllib.parse import urlsplit

from pyppeteer import connect


PINNED_PRODUCT = "Chrome/152.0.7977.82"
MARKER = "__MIMIC_API_ACCESS__"


def product(endpoint):
    with urllib.request.urlopen(endpoint.rstrip("/") + "/json/version") as response:
        return json.load(response).get("Browser", "")


INSTRUMENT = r"""() => {
  const seen = new Set();
  const debug = console.debug.bind(console);
  const intrinsic = new Set(['Object','Function','Array','Number','String','Boolean','RegExp','Date','Error','EvalError','RangeError','ReferenceError','SyntaxError','TypeError','URIError','AggregateError','Promise','Proxy','Map','Set','WeakMap','WeakSet','ArrayBuffer','SharedArrayBuffer','DataView','Int8Array','Uint8Array','Uint8ClampedArray','Int16Array','Uint16Array','Int32Array','Uint32Array','Float32Array','Float64Array','BigInt64Array','BigUint64Array','Symbol','BigInt','Atomics','Math','JSON','Reflect','Intl','Console']);
  const emit = name => {
    if (seen.has(name)) return;
    seen.add(name);
    debug('__MIMIC_API_ACCESS__' + name);
  };
  const emitValue = (api, value) => {
    if (value === null || ['string','number','boolean','undefined'].includes(typeof value)) {
      let text;
      try { text = JSON.stringify(value); } catch (_) { text = String(value); }
      emit(api + ' => ' + String(text).slice(0, 200));
    }
  };
  for (const interfaceName of Object.getOwnPropertyNames(globalThis)) {
    if (intrinsic.has(interfaceName)) continue;
    let ctor;
    try { ctor = globalThis[interfaceName]; } catch (_) { continue; }
    if (typeof ctor !== 'function' || !ctor.prototype) continue;
    for (const member of Object.getOwnPropertyNames(ctor.prototype)) {
      if (member === 'constructor') continue;
      let descriptor;
      try { descriptor = Object.getOwnPropertyDescriptor(ctor.prototype, member); } catch (_) { continue; }
      if (!descriptor || !descriptor.configurable) continue;
      const api = interfaceName + '.' + member;
      if (typeof descriptor.value === 'function') {
        descriptor.value = new Proxy(descriptor.value, {
          apply(target, self, args) { emit(api); const result=Reflect.apply(target, self, args); emitValue(api,result); return result; },
          construct(target, args, newTarget) { emit(api); return Reflect.construct(target, args, newTarget); }
        });
      }
      if (typeof descriptor.get === 'function') {
        descriptor.get = new Proxy(descriptor.get, {
          apply(target, self, args) { emit(api); const result=Reflect.apply(target, self, args); emitValue(api,result); return result; }
        });
      }
      if (typeof descriptor.set === 'function') {
        descriptor.set = new Proxy(descriptor.set, {
          apply(target, self, args) { emit(api + ' setter'); return Reflect.apply(target, self, args); }
        });
      }
      try { Object.defineProperty(ctor.prototype, member, descriptor); } catch (_) {}
    }
  }
  // Web-platform namespaces are objects rather than interface constructors,
  // so the prototype pass above cannot see their static operations.
  for (const namespaceName of ['WebAssembly']) {
    const namespace = globalThis[namespaceName];
    if (!namespace || typeof namespace !== 'object') continue;
    try {
      Object.defineProperty(globalThis, namespaceName, {
        configurable: true,
        writable: true,
        value: new Proxy(namespace, {
          get(target, property, receiver) {
            if (typeof property === 'string') emit(namespaceName + '.' + property);
            return Reflect.get(target, property, receiver);
          }
        })
      });
    } catch (_) {}
  }
  // Record reads of Web-platform constructors/functions too. This exposes
  // APIs used through static operations or construction, which prototype-only
  // instrumentation cannot observe.
  for (const name of Object.getOwnPropertyNames(globalThis)) {
    if (intrinsic.has(name) || ['window','self','globalThis'].includes(name)) continue;
    const descriptor = Object.getOwnPropertyDescriptor(globalThis, name);
    if (!descriptor || !descriptor.configurable || !('value' in descriptor)) continue;
    let value = descriptor.value;
    if (typeof value !== 'function') continue;
    try {
      Object.defineProperty(globalThis, name, {
        enumerable: descriptor.enumerable,
        configurable: true,
        get() { emit('Window.' + name); return value; },
        set(next) { value = next; }
      });
    } catch (_) {}
  }
}"""

REFLECTION_INSTRUMENT = r"""() => {
  const marker = '__MIMIC_API_ACCESS__';
  const seen = new Set();
  const debug = console.debug.bind(console);
  const original = {
    names: Object.getOwnPropertyNames,
    keys: Object.keys,
    descriptor: Object.getOwnPropertyDescriptor,
    prototype: Object.getPrototypeOf,
    ownKeys: Reflect.ownKeys,
    tag: Object.prototype.toString,
  };
  const emit = name => { if (!seen.has(name)) { seen.add(name); debug(marker + name); } };
  const label = value => {
    if (value === globalThis) return 'Window';
    if (value === null) return 'null';
    if (value === undefined) return 'undefined';
    let tag = '';
    try { tag = Reflect.apply(original.tag, value, []).slice(8, -1); } catch (_) {}
    return tag || typeof value;
  };
  Object.getOwnPropertyNames = new Proxy(original.names, {apply(target, self, args) { emit('Reflect.getOwnPropertyNames<' + label(args[0]) + '>'); return Reflect.apply(target, self, args); }});
  Object.keys = new Proxy(original.keys, {apply(target, self, args) { emit('Reflect.keys<' + label(args[0]) + '>'); return Reflect.apply(target, self, args); }});
  Object.getOwnPropertyDescriptor = new Proxy(original.descriptor, {apply(target, self, args) { emit('Reflect.getOwnPropertyDescriptor<' + label(args[0]) + '>.' + String(args[1])); return Reflect.apply(target, self, args); }});
  Object.getPrototypeOf = new Proxy(original.prototype, {apply(target, self, args) { emit('Reflect.getPrototypeOf<' + label(args[0]) + '>'); return Reflect.apply(target, self, args); }});
  Reflect.ownKeys = new Proxy(original.ownKeys, {apply(target, self, args) { emit('Reflect.ownKeys<' + label(args[0]) + '>'); return Reflect.apply(target, self, args); }});
}"""


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--url", default="https://voxel.shop/")
    parser.add_argument("--settle-ms", type=int, default=8_000)
    parser.add_argument("--headers", action="store_true")
    parser.add_argument("--expected-product", default=PINNED_PRODUCT)
    parser.add_argument("--reflection-only", action="store_true")
    parser.add_argument("--timeout", type=int, default=45_000)
    parser.add_argument("--wait-until", default="load", choices=["load", "domcontentloaded"])
    args = parser.parse_args()
    observed = product(args.chrome)
    if observed != args.expected_product:
        raise RuntimeError(f"Chrome oracle drift: {observed!r}")

    browser = await connect(browserURL=args.chrome, defaultViewport=None)
    page = await browser.newPage()
    accesses_before_fo, accesses_after_fo, resources, requests, errors, network_responses = [], [], [], [], [], []
    first_fo_requested = False

    def console(message):
        text = message.text
        if text.startswith(MARKER):
            target = accesses_after_fo if first_fo_requested else accesses_before_fo
            target.append(text[len(MARKER):])

    def request_seen(request):
        nonlocal first_fo_requested
        path = urlsplit(request.url).path
        requests.append({
            "url": f"{urlsplit(request.url).scheme}://{urlsplit(request.url).netloc}{path}",
            "method": request.method,
            "type": request.resourceType,
            "postDataLength": len(request.postData or ""),
            **({"headers": request.headers} if args.headers else {}),
        })
        if "/fo/" in path:
            first_fo_requested = True

    page.on("console", console)
    page.on("pageerror", lambda error: errors.append(str(error)))
    page.on("request", request_seen)
    page.on("response", lambda response: resources.append({
        "status": response.status,
        "url": f"{urlsplit(response.url).scheme}://{urlsplit(response.url).netloc}{urlsplit(response.url).path}",
    }))
    page._client.on("Network.responseReceived", lambda event: network_responses.append({
        "url": event.get("response", {}).get("url"),
        "status": event.get("response", {}).get("status"),
        "protocol": event.get("response", {}).get("protocol"),
        "connectionReused": event.get("response", {}).get("connectionReused"),
        "connectionId": event.get("response", {}).get("connectionId"),
        "timing": event.get("response", {}).get("timing"),
        "headers": event.get("response", {}).get("headers") if args.headers else None,
    }))
    await page._client.send("Network.clearBrowserCookies")
    await page._client.send("Network.clearBrowserCache")
    await page.evaluateOnNewDocument(REFLECTION_INSTRUMENT if args.reflection_only else INSTRUMENT)
    try:
        navigation_error = None
        try:
            await page.goto(args.url, {"waitUntil": args.wait_until, "timeout": args.timeout})
        except Exception as exc:
            navigation_error = str(exc)
        await asyncio.sleep(args.settle_ms / 1000)
        state = await page.evaluate("() => ({title:document.title,url:location.href,cookie:document.cookie})")
        print(json.dumps({
            "target": observed,
            "state": state,
            "firstFORequested": first_fo_requested,
            "accessesBeforeFirstFO": accesses_before_fo,
            "accessesAfterFirstFO": accesses_after_fo,
            "requests": requests,
            "resources": resources,
            "networkResponses": network_responses,
            "errors": errors,
            "navigationError": navigation_error,
        }, ensure_ascii=False, indent=2))
    finally:
        await page.close()
        await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
