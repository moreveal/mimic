"""Capture bounded, frame-attributed Web API access and network ordering."""

import argparse
import asyncio
import json
import time

from pyppeteer import connect

from chrome_api_trace import INSTRUMENT, MARKER


CONTEXT_INSTRUMENT = INSTRUMENT.replace(
    "debug('__MIMIC_API_ACCESS__' + name);",
    "debug('__MIMIC_API_ACCESS__'+JSON.stringify({href:location.href,name}));",
)


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", required=True)
    parser.add_argument("--settle-ms", type=int, default=8_000)
    parser.add_argument("--limit", type=int, default=2_000)
    args = parser.parse_args()

    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    started = time.monotonic()
    events = []

    def append(kind, **data):
        if len(events) < args.limit:
            events.append({"timeMs": round((time.monotonic() - started) * 1000, 3), "kind": kind, **data})

    def console(message):
        if not message.text.startswith(MARKER):
            return
        try:
            append("api", **json.loads(message.text[len(MARKER):]))
        except Exception:
            append("api", href="", name=message.text[len(MARKER):])

    page.on("console", console)
    page.on("request", lambda request: append("request", url=request.url.split("?", 1)[0], method=request.method, resourceType=request.resourceType))
    page.on("response", lambda response: append("response", url=response.url.split("?", 1)[0], status=response.status))
    page.on("pageerror", lambda error: append("pageerror", error=str(error)))
    await page._client.send("Network.clearBrowserCookies")
    await page._client.send("Network.clearBrowserCache")
    await page.evaluateOnNewDocument(CONTEXT_INSTRUMENT)
    navigation_error = ""
    try:
        await page.goto(args.url, {"waitUntil": "load", "timeout": 120_000})
    except Exception as exc:
        navigation_error = str(exc)
    await asyncio.sleep(args.settle_ms / 1000)
    state = await page.evaluate("() => ({url:location.href,title:document.title,cookie:document.cookie})")
    frames = []
    for frame in page.frames:
        try:
            value = await frame.evaluate("""() => ({
              url:location.href, origin:location.origin, readyState:document.readyState,
              referrer:document.referrer, visibility:document.visibilityState,
              title:document.title, bodyChildren:document.body&&document.body.children.length,
              bodyHTMLLength:document.body&&document.body.innerHTML.length,
              parentIsSelf:parent===self, topIsSelf:top===self,
              frameElementTag:frameElement&&frameElement.tagName,
              dark:matchMedia('(prefers-color-scheme: dark)').matches,
              reducedMotion:matchMedia('(prefers-reduced-motion: reduce)').matches,
              secure:isSecureContext, isolated:crossOriginIsolated,
              worker:typeof Worker, rtc:typeof RTCPeerConnection,
              entries:performance.getEntries().map(entry=>[entry.entryType,entry.name,entry.duration]).slice(0,8)
            })""")
            frames.append(value)
        except Exception as exc:
            frames.append({"url": frame.url, "error": str(exc)})
    print(json.dumps({"navigationError": navigation_error, "state": state, "frames": frames, "events": events}, ensure_ascii=False, indent=2))
    await page.close()
    await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
