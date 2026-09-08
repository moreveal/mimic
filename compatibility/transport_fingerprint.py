"""Capture an externally observed TLS/HTTP fingerprint through a CDP target."""

import argparse
import asyncio
import json

from pyppeteer import connect


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", default="https://tls.peet.ws/api/all")
    parser.add_argument("--timeout", type=int, default=30_000)
    parser.add_argument("--xhr-probe", action="store_true", help="repeat through XHR with generic author headers")
    args = parser.parse_args()

    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    response = await page.goto(args.url, {"waitUntil": "load", "timeout": args.timeout})
    body = await response.text()
    if args.xhr_probe:
        body = await page.evaluate("""url => new Promise((resolve, reject) => {
          const xhr = new XMLHttpRequest();
          xhr.open('POST', url);
          xhr.setRequestHeader('X-Mimic-First', 'one');
          xhr.setRequestHeader('X-Mimic-Second', 'two');
          xhr.setRequestHeader('X-Mimic-Third', 'three');
          xhr.onload = () => resolve(xhr.responseText);
          xhr.onerror = () => reject(new Error('XHR failed'));
          xhr.send('probe');
        })""", args.url)
    try:
        value = json.loads(body)
    except json.JSONDecodeError:
        value = {"status": response.status, "body": body[:2000]}
    print(json.dumps(value, ensure_ascii=False, indent=2))
    await page.close()
    await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
