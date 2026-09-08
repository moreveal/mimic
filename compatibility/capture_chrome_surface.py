"""Capture reproducible Window exposure metadata from the pinned Chrome oracle.

This is intentionally a generator input, not runtime code.  It records the
effective Blink/runtime-feature/platform exposure that cannot be recovered
from WebIDL's [Exposed] attribute alone.
"""

import argparse
import asyncio
import json
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import threading
import urllib.request

from pyppeteer import connect
from oracle import (
    MODES,
    PINNED_PRODUCT,
    capture_metadata,
    default_profile_id,
    prepare_page,
    product,
)


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
        context = await page.evaluate(
            """() => ({
              url: location.href,
              origin: location.origin,
              secureContext: isSecureContext,
              crossOriginIsolated,
              userAgent: navigator.userAgent,
              platform: navigator.platform
            })"""
        )
        properties = await page.evaluate(
            """() => Object.getOwnPropertyNames(window).sort().map(name => {
              const descriptor = Object.getOwnPropertyDescriptor(window, name);
              let valueType = 'accessor';
              let functionName = null;
              let functionLength = null;
              if ('value' in descriptor) {
                valueType = typeof descriptor.value;
                if (valueType === 'function') {
                  functionName = descriptor.value.name;
                  functionLength = descriptor.value.length;
                }
              }
              return {
                name,
                enumerable: !!descriptor.enumerable,
                configurable: !!descriptor.configurable,
                writable: 'writable' in descriptor ? !!descriptor.writable : null,
                getter: typeof descriptor.get === 'function',
                setter: typeof descriptor.set === 'function',
                valueType,
                functionName,
                functionLength
              };
            })"""
        )
        prototypes = await page.evaluate(
            """() => {
              const describe = (owner, name) => {
                const descriptor = Object.getOwnPropertyDescriptor(owner, name);
                let valueType = 'accessor', functionName = null, functionLength = null;
                if ('value' in descriptor) {
                  valueType = typeof descriptor.value;
                  if (valueType === 'function') {
                    functionName = descriptor.value.name;
                    functionLength = descriptor.value.length;
                  }
                }
                return {
                  name, enumerable: !!descriptor.enumerable,
                  configurable: !!descriptor.configurable,
                  writable: 'writable' in descriptor ? !!descriptor.writable : null,
                  getter: typeof descriptor.get === 'function',
                  setter: typeof descriptor.set === 'function',
                  valueType, functionName, functionLength
                };
              };
              const result = {};
              for (const name of Object.getOwnPropertyNames(window)) {
                const descriptor = Object.getOwnPropertyDescriptor(window, name);
                const value = descriptor && descriptor.value;
                if (typeof value !== 'function' || !value.prototype) continue;
                result[name] = Object.getOwnPropertyNames(value.prototype).sort().map(
                  member => describe(value.prototype, member));
              }
              return result;
            }"""
        )
        return {
            "schemaVersion": 1,
            "target": {
                "product": observed,
                "milestone": 152,
                "version": "152.0.7977.82",
                "chromiumRevision": 1669021,
                "chromiumCommit": "d04cdb24d67b081f6cf80200ffc5233f44b61109",
                "platform": "windows-x64",
                "channel": "stable",
            },
            "captureMetadata": await capture_metadata(
                endpoint, browser, page, mode=mode,
                environment_profile_id=profile_id,
                declared_feature_overrides=feature_overrides,
            ),
            "captureContext": context,
            "properties": properties,
            "prototypes": prototypes,
        }
    finally:
        await page.close()
        await browser.disconnect()


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--url", default="https://example.com/")
    parser.add_argument("--output", default="chrome/152/generated/window-secure.json")
    parser.add_argument("--isolated-fixture", action="store_true")
    parser.add_argument("--browser-mode", choices=MODES, default="headful")
    parser.add_argument("--environment-profile-id", default="")
    parser.add_argument("--feature-override", action="append", default=[])
    parser.add_argument("--window-size", default="1280x800")
    args = parser.parse_args()
    server = None
    if args.isolated_fixture:
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                body = b"<!doctype html><title>Mimic isolated exposure fixture</title>"
                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Cross-Origin-Opener-Policy", "same-origin")
                self.send_header("Cross-Origin-Embedder-Policy", "require-corp")
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *_args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        args.url = f"http://127.0.0.1:{server.server_port}/"
    try:
        profile_id = args.environment_profile_id or default_profile_id(args.browser_mode)
        result = await capture(args.chrome, args.url, args.browser_mode, profile_id,
                               args.feature_override, args.window_size)
    finally:
        if server:
            server.shutdown()
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"captured {len(result['properties'])} properties -> {output}")


if __name__ == "__main__":
    asyncio.run(main())
