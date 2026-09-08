"""Read-only BrowserScan compatibility benchmark for a running CDP endpoint."""

import argparse
import asyncio
import json
from pathlib import Path

from pyppeteer import connect


async def run(endpoint, url, settle_ms):
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    page_exceptions = []
    console_errors = []
    console_messages = []
    requests = []
    responses = []
    response_tasks = []
    page.on("pageerror", lambda error: page_exceptions.append(str(error)))
    page.on("console", lambda message: (console_messages.append({"type": message.type, "text": message.text}), console_errors.append(message.text) if message.type == "error" else None))
    page.on("request", lambda request: requests.append({"url": request.url, "method": request.method, "type": request.resourceType, "headers": request.headers, "postData": request.postData}))
    async def record_response(response):
        item = {"url": response.url, "status": response.status}
        if "/api/" in response.url:
            try:
                item["body"] = (await response.text())[:20_000]
            except Exception as exc:
                item["bodyError"] = str(exc)
        responses.append(item)
    page.on("response", lambda response: response_tasks.append(asyncio.ensure_future(record_response(response))))
    has_trace = True
    try:
        await page._client.send("Mimic.clearTrace")
    except Exception:
        has_trace = False
    navigation = asyncio.ensure_future(page.goto(url, {"waitUntil": "domcontentloaded", "timeout": 60_000}))
    await asyncio.sleep(settle_ms / 1000)
    response = navigation.result() if navigation.done() and not navigation.cancelled() and navigation.exception() is None else None
    # Third-party requests may still be in flight after the benchmark itself
    # has rendered. Record completed response bodies without letting analytics
    # traffic hold the observation open.
    completed_response_tasks = [task for task in response_tasks if task.done()]
    if completed_response_tasks:
        await asyncio.gather(*completed_response_tasks, return_exceptions=True)
    # Mimic deliberately keeps Page.navigate attached while browser tasks spawned
    # during navigation are running. Observe the live target through an independent
    # CDP attachment, as DevTools does, so unrelated third-party traffic cannot
    # serialize this read-only snapshot behind Page.navigate.
    observer_browser = await connect(browserURL=endpoint, defaultViewport=None)
    observer_pages = await observer_browser.pages()
    matching_pages = [candidate for candidate in observer_pages if candidate.url == url]
    observer_page = matching_pages[-1] if matching_pages else page
    state = await observer_page.evaluate(r"""() => {
      const text=document.body?document.body.innerText:'';
      const lines=text.split(/\n+/).map(value=>value.trim()).filter(Boolean);
      // Only accept a classification next to its heading: individual checks
      // also say "Normal" and must not be mistaken for an overall result.
      const resultText=Array.from(document.querySelectorAll('*')).map(element=>String(element.textContent||'').trim()).find(value=>/^Test Results:\s*(Robot|Human|Normal|No bots detected)$/.test(value));
      const classification=resultText?resultText.replace(/^Test Results:\s*/,''):null;
      const resultElement=Array.from(document.querySelectorAll('*')).filter(element=>String(element.textContent||'').includes('Test Results:')).sort((a,b)=>String(a.textContent||'').length-String(b.textContent||'').length)[0]||null;
      const normalChecks=[];
      for(let i=0;i+1<lines.length;i++)if(lines[i+1]==='Normal')normalChecks.push(lines[i]);
      return {title:document.title,url:location.href,readyState:document.readyState,
        bodyHTMLLength:document.body?document.body.innerHTML.length:0,
        bodyText:text.slice(0,4000),
        nuxtData:(document.querySelector('#__NUXT_DATA__')?.textContent||'').slice(0,4000),
        resultRegion:resultElement?{tag:resultElement.tagName,text:String(resultElement.textContent||'').slice(0,1000),html:String(resultElement.innerHTML||'').slice(0,2000),children:Array.from(resultElement.children||[]).map(child=>String(child.textContent||'').slice(0,300)),parentText:String(resultElement.parentElement?.textContent||'').slice(0,1200),parentHTML:String(resultElement.parentElement?.innerHTML||'').slice(0,2500)}:null,
        classification,normalChecks:[...new Set(normalChecks)]};
    }""")
    trace = await observer_page._client.send("Mimic.getTrace") if has_trace else {"events": []}
    events = trace.get("events", [])
    issue_kinds = {"exception", "error", "unsupported", "semantic-missing", "surface-missing"}
    result = {
        "endpoint": endpoint,
        "response": response.status if response else None,
        "state": state,
        "classification": state["classification"],
        "pageExceptions": page_exceptions,
        "consoleErrors": console_errors,
        "consoleMessages": console_messages,
        "requests": requests,
        "responses": responses,
        "apiAccesses": [event for event in events if event.get("kind") == "api"],
        "runtimeIssues": [event for event in events if event.get("kind") in issue_kinds],
    }
    if not navigation.done():
        navigation.cancel()
    for task in response_tasks:
        if not task.done():
            task.cancel()
    await observer_browser.disconnect()
    await browser.disconnect()
    return result


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", default="http://127.0.0.1:9222")
    parser.add_argument("--url", default="https://www.browserscan.net/bot-detection")
    parser.add_argument("--settle-ms", type=int, default=5_000)
    parser.add_argument("--out")
    args = parser.parse_args()
    result = await run(args.endpoint, args.url, args.settle_ms)
    encoded = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.out:
        Path(args.out).write_text(encoded, encoding="utf-8")
    print(encoded, end="")


if __name__ == "__main__":
    asyncio.run(main())
