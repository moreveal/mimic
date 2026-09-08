"""Capture child-frame state at the post-/fo message boundary.

The page hook only observes lifecycle and MessageEvent delivery. Child state is
read through the frame's CDP context, without replacing browser APIs.
"""

import argparse
import asyncio
import json
import time
from pathlib import Path
from urllib.parse import urlsplit

from pyppeteer import connect


MARKER = "__MIMIC_CHILD_BOUNDARY__"

INSTRUMENT = r"""() => {
  const marker = '__MIMIC_CHILD_BOUNDARY__';
  const nativeDebug = console.debug.bind(console);
  const scalar = value => {
    if (value === undefined) return {type:'undefined'};
    if (value === null || ['string','number','boolean'].includes(typeof value))
      return {type:value === null ? 'null' : typeof value,
        value:typeof value === 'string' ? value.slice(0,240) : value};
    if (Array.isArray(value)) return {type:'array',length:value.length};
    let tag = '';
    try { tag = Object.prototype.toString.call(value); } catch (_) {}
    return {type:typeof value,tag};
  };
  const shape = value => {
    if (!value || typeof value !== 'object') return scalar(value);
    const result = {};
    let keys = [];
    try { keys = Object.keys(value).sort(); } catch (_) {}
    for (const key of keys) {
      try { result[key] = scalar(value[key]); }
      catch (error) { result[key] = {error:String(error)}; }
    }
    return result;
  };
  const emit = (phase, extra) => {
    let value;
    try { value = {phase,child:parent !== self,href:location.href,
      readyState:document.readyState,extra}; }
    catch (error) { value = {phase,error:String(error)}; }
    nativeDebug(marker + JSON.stringify(value));
  };
  addEventListener('DOMContentLoaded', () => queueMicrotask(() => emit('DOMContentLoaded')));
  addEventListener('load', () => queueMicrotask(() => emit('load')));
  addEventListener('message', event => queueMicrotask(() => emit('message-after-handlers', {
    origin:event.origin,sourceIsParent:event.source === parent,
    sourceIsSelf:event.source === self,ports:event.ports.length,data:shape(event.data)
  })));
  queueMicrotask(() => emit('installed'));
}"""

CHILD_SNAPSHOT = r"""() => {
  const safe = callback => { try { return callback(); } catch (error) { return {error:String(error)}; } };
  const node = value => value ? {tag:value.tagName||'',id:value.id||'',className:typeof value.className==='string'?value.className.slice(0,120):''} : null;
  const rect = value => value ? safe(() => { const r=value.getBoundingClientRect(); return {x:r.x,y:r.y,width:r.width,height:r.height,top:r.top,right:r.right,bottom:r.bottom,left:r.left}; }) : null;
  return {
    url:location.href,origin:location.origin,readyState:document.readyState,
    referrer:document.referrer,visibilityState:document.visibilityState,
    hidden:document.hidden,hasFocus:document.hasFocus(),activeElement:node(document.activeElement),
    parentIsSelf:parent===self,topIsSelf:top===self,parentIsTop:parent===top,
    frameElement:node(frameElement),secureContext:isSecureContext,crossOriginIsolated,
    viewport:{innerWidth,innerHeight,outerWidth,outerHeight,devicePixelRatio,scrollX,scrollY},
    screen:{width:screen.width,height:screen.height,availWidth:screen.availWidth,availHeight:screen.availHeight,colorDepth:screen.colorDepth,pixelDepth:screen.pixelDepth},
    navigator:{userAgent:navigator.userAgent,platform:navigator.platform,vendor:navigator.vendor,
      languages:Array.from(navigator.languages),language:navigator.language,
      hardwareConcurrency:navigator.hardwareConcurrency,deviceMemory:navigator.deviceMemory,
      maxTouchPoints:navigator.maxTouchPoints,webdriver:navigator.webdriver,onLine:navigator.onLine,
      cookieEnabled:navigator.cookieEnabled,pdfViewerEnabled:navigator.pdfViewerEnabled},
    storage:safe(() => ({cookie:document.cookie,localLength:localStorage.length,sessionLength:sessionStorage.length})),
    documentElement:safe(() => ({rect:rect(document.documentElement),clientWidth:document.documentElement.clientWidth,clientHeight:document.documentElement.clientHeight})),
    body:safe(() => document.body ? ({rect:rect(document.body),children:document.body.children.length,
      clientWidth:document.body.clientWidth,clientHeight:document.body.clientHeight,
      offsetWidth:document.body.offsetWidth,offsetHeight:document.body.offsetHeight}) : null),
    media:{dark:matchMedia('(prefers-color-scheme: dark)').matches,
      reducedMotion:matchMedia('(prefers-reduced-motion: reduce)').matches},
    entries:safe(() => performance.getEntries().map(entry => ({entryType:entry.entryType,
      name:entry.name,startTime:entry.startTime,duration:entry.duration,
      requestStart:entry.requestStart||0,responseStart:entry.responseStart||0,
      responseEnd:entry.responseEnd||0,transferSize:entry.transferSize||0,
      encodedBodySize:entry.encodedBodySize||0,nextHopProtocol:entry.nextHopProtocol||''})).slice(0,24))
  };
}"""


def clean_url(url):
    parts = urlsplit(url)
    return f"{parts.scheme}://{parts.netloc}{parts.path}"


async def capture(args):
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    started = time.monotonic()
    boundaries, child_samples, network, errors = [], [], [], []
    sample_lock = asyncio.Lock()
    target_sessions = {}
    top_message_sequence = 0

    if args.clear_state:
        await page._client.send("Network.clearBrowserCookies")
        await page._client.send("Network.clearBrowserCache")

    def elapsed():
        return round((time.monotonic() - started) * 1000, 3)

    async def sample_children(reason):
        if sample_lock.locked():
            return
        async with sample_lock:
            sampled_urls = set()
            for frame in list(page.frames):
                if frame.parentFrame is None:
                    continue
                try:
                    value = await frame.evaluate(CHILD_SNAPSHOT)
                    child_samples.append({"observedMs":elapsed(),"reason":reason,**value})
                    sampled_urls.add(value.get("url", ""))
                except Exception as error:
                    text = str(error)
                    if "detached" not in text.lower() and "context" not in text.lower():
                        errors.append(f"child sample: {text}")
            try:
                target_infos = (await browser._connection.send("Target.getTargets")).get("targetInfos", [])
            except Exception as error:
                errors.append(f"target inventory: {error}")
                return
            for info in target_infos:
                if info.get("type") != "iframe" or info.get("url", "") in sampled_urls:
                    continue
                target_id = info.get("targetId", "")
                try:
                    session = target_sessions.get(target_id)
                    if session is None:
                        session = await browser._connection.createSession(info)
                        target_sessions[target_id] = session
                        await session.send("Runtime.enable")
                    response = await session.send("Runtime.evaluate", {
                        "expression": f"({CHILD_SNAPSHOT})()",
                        "returnByValue": True,
                    })
                    value = response.get("result", {}).get("value")
                    if isinstance(value, dict):
                        child_samples.append({"observedMs":elapsed(),"reason":reason,
                            "targetId":target_id,**value})
                except Exception as error:
                    text = str(error)
                    if "closed" not in text.lower() and "target" not in text.lower():
                        errors.append(f"iframe target sample: {text}")

    def schedule_sample(reason):
        asyncio.ensure_future(sample_children(reason))

    def console(message):
        nonlocal top_message_sequence
        if not message.text.startswith(MARKER):
            return
        try:
            value = json.loads(message.text[len(MARKER):])
            value["observedMs"] = elapsed()
            if not value.get("child") and value.get("phase") == "message-after-handlers":
                top_message_sequence += 1
                value["messageSequence"] = top_message_sequence
            boundaries.append(value)
            if args.sample_boundaries and value.get("messageSequence") is not None:
                schedule_sample(f"message-{value['messageSequence']}")
        except Exception as error:
            errors.append(f"boundary decode: {error}")

    def request_seen(request):
        network.append({"observedMs":elapsed(),"kind":"request","method":request.method,
            "resourceType":request.resourceType,"url":clean_url(request.url),
            "bodyLength":len(request.postData or "")})
        if args.sample_boundaries and "/fo/" in urlsplit(request.url).path:
            schedule_sample("fo-request")

    page.on("console", console)
    page.on("pageerror", lambda error: errors.append(str(error)))
    page.on("request", request_seen)
    page.on("response", lambda response: network.append({"observedMs":elapsed(),
        "kind":"response","status":response.status,"url":clean_url(response.url)}))
    await page.evaluateOnNewDocument(INSTRUMENT)

    navigation_error = ""
    try:
        try:
            await page.goto(args.url, {"waitUntil":"load","timeout":args.timeout})
        except Exception as error:
            navigation_error = str(error)
        await asyncio.sleep(args.settle_ms / 1000)
        final_state = await page.evaluate("() => ({url:location.href,title:document.title,cookie:document.cookie})")
        frames = [frame.url for frame in page.frames]
        result = {"endpoint":args.endpoint,"navigationError":navigation_error,
            "finalState":final_state,"frames":frames,"network":network,
            "boundaries":boundaries,"childSamples":child_samples,"errors":errors}
        encoded = json.dumps(result, ensure_ascii=False, indent=2)
        if args.output:
            Path(args.output).write_text(encoded + "\n", encoding="utf-8")
        print(json.dumps({"finalState":final_state,
            "acceptedFO":any(item.get("kind")=="response" and item.get("status")==200 and "/fo/" in item.get("url","") for item in network),
            "foRequests":sum(item.get("kind")=="request" and "/fo/" in item.get("url","") for item in network),
            "topMessages":sum(not item.get("child") and item.get("phase")=="message-after-handlers" for item in boundaries),
            "childSamples":len(child_samples),"errors":errors,"output":args.output},
            ensure_ascii=False,indent=2))
    finally:
        await page.close()
        await browser.disconnect()


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", required=True)
    parser.add_argument("--timeout", type=int, default=60_000)
    parser.add_argument("--settle-ms", type=int, default=8_000)
    parser.add_argument("--clear-state", action="store_true")
    parser.add_argument("--sample-boundaries", action="store_true")
    parser.add_argument("--output")
    await capture(parser.parse_args())


if __name__ == "__main__":
    asyncio.run(main())
