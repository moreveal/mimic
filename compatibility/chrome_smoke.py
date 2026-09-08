"""Run an end-to-end navigation against an already running exact Chrome oracle."""
import argparse
import asyncio
import json

from pyppeteer import connect


async def run(endpoint: str, url: str, timeout: int, settle_ms: int, clear_cookies: bool, trace_observations: bool) -> None:
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    pages = await browser.pages()
    page = pages[0] if pages else await browser.newPage()
    if trace_observations:
        await page.evaluateOnNewDocument("""() => {
            globalThis.__mimicOracleObservations = {performance: []};
            const record = (method, entries) => {
                globalThis.__mimicOracleObservations.performance.push({method, entries: entries.map(entry => ({
                    name: entry.name, entryType: entry.entryType, initiatorType: entry.initiatorType,
                    startTime: entry.startTime, duration: entry.duration,
                    fetchStart: entry.fetchStart, responseEnd: entry.responseEnd,
                    transferSize: entry.transferSize, encodedBodySize: entry.encodedBodySize,
                    decodedBodySize: entry.decodedBodySize, nextHopProtocol: entry.nextHopProtocol
                }))});
            };
            for (const method of ['getEntries', 'getEntriesByType']) {
                const original = Performance.prototype[method];
                Object.defineProperty(Performance.prototype, method, {configurable: true, enumerable: true, writable: true,
                    value: new Proxy(original, {apply(target, self, args) { const entries = Reflect.apply(target, self, args); record(method + ':' + String(args[0] || ''), entries); return entries; }})});
            }
        }""")
    if clear_cookies:
        await page._client.send("Network.clearBrowserCookies")
    requests, responses, console, errors, response_tasks = [], [], [], [], []
    page.on("request", lambda r: requests.append({
        "url": r.url,
        "method": r.method,
        "type": r.resourceType,
        "headers": r.headers,
        "postDataLength": len(r.postData or ""),
    }))
    async def record_response(response):
        item = {"url": response.url, "status": response.status}
        responses.append(item)
        if response.status >= 400 and "/fo/" in response.url:
            try:
                item["body"] = (await response.text())[:1000]
            except Exception as exc:
                item["bodyError"] = str(exc)

    page.on("response", lambda response: response_tasks.append(asyncio.ensure_future(record_response(response))))
    page.on("console", lambda m: console.append({"type": m.type, "text": m.text}))
    page.on("pageerror", lambda e: errors.append(str(e)))
    navigation_error = ""
    response = None
    try:
        response = await page.goto(url, {"waitUntil": "load", "timeout": timeout})
    except Exception as exc:
        navigation_error = str(exc)
    await asyncio.sleep(settle_ms / 1000)
    if response_tasks:
        await asyncio.gather(*response_tasks, return_exceptions=True)
    state = await page.evaluate("() => ({title:document.title,url:location.href,cookie:document.cookie,turnstileType:typeof globalThis.turnstile,callbackType:typeof globalThis.khCN8,bodyText:document.body?document.body.textContent.slice(0,500):'',oracleObservations:globalThis.__mimicOracleObservations||null})")
    print(json.dumps({"response": response.status if response else None, "navigationError": navigation_error, "state": state, "requests": requests, "responses": responses, "console": console, "errors": errors}, ensure_ascii=False, indent=2))
    await browser.disconnect()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", default="http://127.0.0.1:9223")
    parser.add_argument("--url", default="https://voxel.shop/")
    parser.add_argument("--timeout", type=int, default=30_000)
    parser.add_argument("--settle-ms", type=int, default=10_000)
    parser.add_argument("--clear-cookies", action="store_true")
    parser.add_argument("--trace-observations", action="store_true")
    args = parser.parse_args()
    asyncio.run(run(args.endpoint, args.url, args.timeout, args.settle_ms, args.clear_cookies, args.trace_observations))
