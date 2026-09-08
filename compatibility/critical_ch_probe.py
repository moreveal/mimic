"""Compare CDP request identity across a navigation Critical-CH restart."""

import argparse
import asyncio
import json

from pyppeteer import connect


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", required=True)
    parser.add_argument("--timeout", type=int, default=90_000)
    args = parser.parse_args()
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    events = []

    def request(event):
        if event.get("type") == "Document" and event.get("documentURL") == args.url:
            events.append({
                "event": "request",
                "requestId": event.get("requestId"),
                "loaderId": event.get("loaderId"),
                "url": event.get("request", {}).get("url"),
                "hasExtraInfo": event.get("hasExtraInfo"),
            })

    def response(event):
        response_value = event.get("response", {})
        if event.get("type") == "Document" and response_value.get("url") == args.url:
            events.append({
                "event": "response",
                "requestId": event.get("requestId"),
                "loaderId": event.get("loaderId"),
                "status": response_value.get("status"),
            })

    page._client.on("Network.requestWillBeSent", request)
    page._client.on("Network.responseReceived", response)
    await page._client.send("Network.clearBrowserCookies")
    result, error = None, None
    try:
        result = await page.goto(args.url, {"waitUntil": "load", "timeout": args.timeout})
    except Exception as exc:
        error = str(exc)
    print(json.dumps({"gotoStatus": result.status if result else None, "error": error, "events": events}, indent=2))
    await page.close()
    await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
