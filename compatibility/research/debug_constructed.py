"""Locate the first constructor/own-property mismatch in a CDP target."""

import argparse
import asyncio
import json

from pyppeteer import connect


EXPRESSIONS = [
    "new Blob(['x'])",
    "new URL('https://example.com')",
    "new URLSearchParams('a=b')",
    "new ReadableStream()",
    "new PerformanceObserver(()=>{})",
    "new XMLHttpRequest()",
    "new Event('x')",
    "new MessageEvent('message',{data:1})",
    "new RTCIceCandidate({candidate:'candidate:x',sdpMid:'0'})",
    "document.createElement('iframe')",
    "new Worker(URL.createObjectURL(new Blob([''])))",
]


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    args = parser.parse_args()
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    await page.goto("https://example.com")
    for expression in EXPRESSIONS:
        try:
            result = await page.evaluate(
                """expression => {
                  try {
                    const value = eval(expression);
                    const result = {ok: true, tag: Object.prototype.toString.call(value), own: Object.getOwnPropertyNames(value)};
                    if (typeof Worker === 'function' && value instanceof Worker) value.terminate();
                    return result;
                  } catch (error) {
                    return {ok: false, error: String(error), stack: error && error.stack};
                  }
                }""",
                expression,
            )
        except Exception as error:
            result = {"protocolError": repr(error)}
        print(expression, json.dumps(result, ensure_ascii=False))
    await page.close()
    await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
