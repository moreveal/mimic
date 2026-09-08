"""Capture effective DedicatedWorker exposure from the pinned Chrome oracle."""

import argparse
import asyncio
import json
from pathlib import Path

from pyppeteer import connect
from oracle import (MODES, PINNED_PRODUCT, capture_metadata, default_profile_id,
                    prepare_page, product)


async def capture(endpoint: str, url: str, mode: str, profile_id: str,
                  feature_overrides: list[str], window_size: str) -> dict:
    observed = product(endpoint)
    if observed != PINNED_PRODUCT:
        raise RuntimeError(f"Chrome oracle drift: expected {PINNED_PRODUCT}, got {observed!r}")
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    try:
        await prepare_page(browser, page, mode, window_size)
        await page.goto(url, {"waitUntil": "load", "timeout": 30_000})
        if mode == "headful":
            await page.bringToFront()
        value = await page.evaluate("""() => new Promise((resolve, reject) => {
          const source = `
            const describe=(owner,name)=>{const d=Object.getOwnPropertyDescriptor(owner,name);let valueType='accessor',functionName=null,functionLength=null;if('value' in d){valueType=typeof d.value;if(valueType==='function'){functionName=d.value.name;functionLength=d.value.length}}return{name,enumerable:!!d.enumerable,configurable:!!d.configurable,writable:'writable' in d?!!d.writable:null,getter:typeof d.get==='function',setter:typeof d.set==='function',valueType,functionName,functionLength}};
            onmessage=()=>{const prototypes={};for(const name of Object.getOwnPropertyNames(self)){const d=Object.getOwnPropertyDescriptor(self,name),value=d&&d.value;if(typeof value==='function'&&value.prototype)prototypes[name]=Object.getOwnPropertyNames(value.prototype).sort().map(member=>describe(value.prototype,member))}postMessage({context:{url:location.href,origin:location.origin,secureContext:isSecureContext,crossOriginIsolated,userAgent:navigator.userAgent,platform:navigator.platform},properties:Object.getOwnPropertyNames(self).sort().map(name=>describe(self,name)),prototypes})}`;
          const objectURL=URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
          const worker=new Worker(objectURL);
          worker.onmessage=event=>{worker.terminate();URL.revokeObjectURL(objectURL);resolve(event.data)};
          worker.onerror=event=>{worker.terminate();URL.revokeObjectURL(objectURL);reject(new Error(event.message))};
          worker.postMessage(null);
        })""")
        page_context = await page.evaluate("() => ({secureContext:isSecureContext,crossOriginIsolated})")
        return {
            "schemaVersion": 1,
            "target": {"product": observed, "milestone": 152, "version": "152.0.7977.82", "chromiumRevision": 1669021, "chromiumCommit": "d04cdb24d67b081f6cf80200ffc5233f44b61109", "platform": "windows-x64", "channel": "stable"},
            "captureMetadata": await capture_metadata(
                endpoint, browser, page, mode=mode,
                environment_profile_id=profile_id,
                declared_feature_overrides=feature_overrides,
                context_states={"worker": value["context"], "ownerWindow": page_context},
            ),
            "captureContext": value["context"],
            "properties": value["properties"],
            "prototypes": value["prototypes"],
        }
    finally:
        await page.close()
        await browser.disconnect()


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--url", default="https://example.com/")
    parser.add_argument("--output", default="chrome/152/generated/worker-secure.json")
    parser.add_argument("--browser-mode", choices=MODES, default="headful")
    parser.add_argument("--environment-profile-id", default="")
    parser.add_argument("--feature-override", action="append", default=[])
    parser.add_argument("--window-size", default="1280x800")
    args = parser.parse_args()
    profile_id = args.environment_profile_id or default_profile_id(args.browser_mode)
    result = await capture(args.chrome, args.url, args.browser_mode, profile_id,
                           args.feature_override, args.window_size)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"captured {len(result['properties'])} worker properties -> {output}")


if __name__ == "__main__":
    asyncio.run(main())
