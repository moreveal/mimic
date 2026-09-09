"""Capture a generic site's navigation, responses, exceptions and settled DOM.

Run against a dedicated, freshly launched endpoint. This is a compatibility
investigation, not part of the frozen performance benchmark.
"""
import argparse
import asyncio
import hashlib
import json
import time
from pathlib import Path

from pyppeteer import connect
from oracle import capture_metadata, default_profile_id, prepare_page


async def capture(args):
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=False)
    binary = Path(args.binary).resolve()
    actual = hashlib.sha256(binary.read_bytes()).hexdigest()
    if actual.lower() != args.sha256.lower():
        raise RuntimeError('Executable differs from just-built/pinned SHA-256')
    receipt = {'binary': str(binary), 'sha256': actual, 'url': args.url,
               'endpoint': args.endpoint, 'oracle': args.chrome}
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    events, pending = [], []
    started = time.monotonic()

    def record(kind, data):
        events.append({'elapsedSeconds': time.monotonic()-started, 'kind': kind, 'data': data})

    async def body(data):
        try:
            value = await page._client.send('Network.getResponseBody', {'requestId': data['requestId']})
            record('responseBody', {'requestId': data['requestId'], **value})
        except Exception as error:
            record('responseBodyUnavailable', {'requestId': data['requestId'], 'error': str(error)})

    try:
        if args.chrome:
            await prepare_page(browser, page, 'headful', '1280x800')
        for kind in ('Runtime.exceptionThrown', 'Runtime.consoleAPICalled',
                     'Network.requestWillBeSent', 'Network.responseReceived',
                     'Network.loadingFailed', 'Page.domContentEventFired', 'Page.loadEventFired'):
            page._client.on(kind, lambda data, kind=kind: record(kind, data))
        page._client.on('Network.loadingFinished', lambda data: pending.append(asyncio.create_task(body(data))))
        await page._client.send('Network.setCacheDisabled', {'cacheDisabled': True})
        try:
            await page.goto(args.url, {'waitUntil': 'load', 'timeout': 60000})
        except Exception as error:
            record('navigationError', str(error))
        if args.chrome:
            receipt['metadata'] = await capture_metadata(args.endpoint, browser, page,
                mode='headful', environment_profile_id=default_profile_id('headful'))
        for elapsed, delay in ((0, 0), (2, 2), (10, 8)):
            await asyncio.sleep(delay)
            root = await page._client.send('DOM.getDocument')
            value = await page._client.send('DOM.getOuterHTML', {'nodeId': root['root']['nodeId']})
            (output/f'dom-load-plus-{elapsed}.html').write_text(value['outerHTML'], encoding='utf8')
            record('domSample', {'secondsAfterLoad': elapsed,
                                'readyState': await page.evaluate('document.readyState'),
                                'sha256': hashlib.sha256(value['outerHTML'].encode()).hexdigest()})
        if not args.chrome:
            trace = await page._client.send('Mimic.getTrace')
            (output/'trace.json').write_text(json.dumps(trace, ensure_ascii=False), encoding='utf8')
        await asyncio.gather(*pending, return_exceptions=True)
    finally:
        (output/'receipt.json').write_text(json.dumps(receipt, indent=2), encoding='utf8')
        (output/'events.json').write_text(json.dumps(events, ensure_ascii=False), encoding='utf8')
        await page.close()
        await browser.disconnect()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('url')
    parser.add_argument('--endpoint', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--binary', required=True, help='Executable used to start the dedicated endpoint')
    parser.add_argument('--sha256', required=True, help='Hash recorded at build/pin time')
    parser.add_argument('--chrome', action='store_true', help='Require pinned headful Chrome and capture provenance')
    asyncio.run(capture(parser.parse_args()))
