#!/usr/bin/env python3
"""Explicitly refresh the CDP schema and parameter oracle from pinned Chrome.

Generation itself is offline (tools/generate_cdp.py). This separate maintenance
tool connects only to a user-started exact-version Chrome, creates its own page,
and closes that page after observing the protocol. Requires websockets.
"""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
from pathlib import Path
import urllib.request

import websockets


ROOT = Path(__file__).resolve().parents[1]


def read_endpoint(endpoint: str, path: str):
    with urllib.request.urlopen(endpoint.rstrip("/") + path, timeout=15) as response:
        return json.load(response)


def write_json(path: Path, value) -> bytes:
    raw = (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(raw)
    return raw


class Connection:
    def __init__(self, socket):
        self.socket = socket
        self.next_id = 0

    async def send(self, method: str, params: str | dict | None = None):
        self.next_id += 1
        raw = '{"id":' + str(self.next_id) + ',"method":' + json.dumps(method)
        if params is not None:
            raw += ',"params":' + (params if isinstance(params, str) else json.dumps(params))
        await self.socket.send(raw + "}")
        while True:
            response = json.loads(await asyncio.wait_for(self.socket.recv(), timeout=15))
            if response.get("id") == self.next_id:
                response.pop("id")
                return response

    async def result(self, method: str, params: dict | None = None):
        response = await self.send(method, params)
        if "error" in response:
            raise RuntimeError(f"{method}: {response['error']}")
        return response["result"]


# Exercise the generated wire decoder independently of DOM, rendering or state.
# Keep raw JSON spelling: integer 1 and number 1.0 are separate observations.
CASES = [
    ("unknown-command", "Runtime.noSuchCommand", "{}"),
    ("missing-params", "Runtime.evaluate", None),
    ("null-params", "Runtime.evaluate", "null"),
    ("array-params", "Runtime.evaluate", "[]"),
    ("required-missing", "Runtime.evaluate", "{}"),
    ("required-null", "Runtime.evaluate", '{"expression":null}'),
    ("required-wrong-type", "Runtime.evaluate", '{"expression":42}'),
    ("unknown-param", "Runtime.evaluate", '{"expression":"1+1","futureField":{"a":1}}'),
    ("optional-null", "Runtime.evaluate", '{"expression":"1+1","returnByValue":null}'),
    ("optional-wrong-type", "Runtime.evaluate", '{"expression":"1+1","returnByValue":"true"}'),
    ("integer-literal", "DOM.getDocument", '{"depth":1}'),
    ("optional-params-missing", "DOM.getDocument", None),
    ("optional-params-null", "DOM.getDocument", 'null'),
    ("optional-params-array", "DOM.getDocument", '[]'),
    ("integer-float", "DOM.getDocument", '{"depth":1.0}'),
    ("integer-exponent", "DOM.getDocument", '{"depth":1e0}'),
    ("integer-fraction", "DOM.getDocument", '{"depth":1.5}'),
    ("integer-overflow", "DOM.getDocument", '{"depth":2147483648}'),
    ("enum-string", "Page.setLifecycleEventsEnabled", '{"enabled":true}'),
    ("enum-unknown-string", "Runtime.evaluate", '{"expression":"1","serializationOptions":{"serialization":"future"}}'),
    ("enum-wrong-type", "Runtime.evaluate", '{"expression":"1","serializationOptions":{"serialization":42}}'),
    ("nested-required", "Runtime.evaluate", '{"expression":"1","serializationOptions":{}}'),
    ("nested-unknown-field", "Runtime.evaluate", '{"expression":"1","serializationOptions":{"serialization":"json","future":42}}'),
    ("array-wrong-type", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":{}}'),
    ("array-item-wrong-type", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":[1]}'),
    ("array-item-null", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":[null]}'),
    ("any-null", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":[{"value":null}]}'),
    ("any-object", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":[{"value":{"a":1}}]}'),
    ("empty-command-null", "Browser.getVersion", "null"),
    ("empty-command-array", "Browser.getVersion", "[]"),
    ("empty-command-extra", "Browser.getVersion", '{"future":true}'),
    ("required-nested-array", "Network.setCookies", '{"cookies":[{}]}'),
    ("optional-object-null", "Runtime.evaluate", '{"expression":"1","serializationOptions":null}'),
    ("optional-array-null", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":null}'),
    ("optional-any-null", "Runtime.callFunctionOn", '{"functionDeclaration":"function(){}","arguments":[{"value":null}]}'),
    ("binary-valid", "Fetch.fulfillRequest", '{"requestId":"missing","responseCode":200,"body":"aGk="}'),
    ("binary-invalid-base64", "Fetch.fulfillRequest", '{"requestId":"missing","responseCode":200,"body":"!"}'),
    ("binary-wrong-type", "Fetch.fulfillRequest", '{"requestId":"missing","responseCode":200,"body":42}'),
    ("binary-unpadded", "Fetch.fulfillRequest", '{"requestId":"missing","responseCode":200,"body":"aGk"}'),
    ("binary-newline", "Fetch.fulfillRequest", '{"requestId":"missing","responseCode":200,"body":"aGk=\\n"}'),
    ("binary-empty", "Fetch.fulfillRequest", '{"requestId":"missing","responseCode":200,"body":""}'),
]


async def capture(args):
    target = json.loads((ROOT / "chrome/152/target.json").read_text(encoding="utf-8"))
    version = read_endpoint(args.endpoint, "/json/version")
    expected = "Chrome/" + target["chrome_version"]
    if version.get("Browser") != expected:
        raise RuntimeError(f"expected {expected}, observed {version.get('Browser')}")
    if version.get("V8-Version") != target["differential_browser"]["v8_version"]:
        raise RuntimeError("V8 version does not match the frozen oracle")
    if target["chromium_commit"] not in version.get("WebKit-Version", ""):
        raise RuntimeError("Chromium revision does not match the frozen oracle")
    mode = "headless" if "HeadlessChrome/" in version.get("User-Agent", "") else "headful"
    if mode != args.browser_mode:
        raise RuntimeError(f"browser mode mismatch: {mode}")
    protocol = read_endpoint(args.endpoint, "/json/protocol")
    async with websockets.connect(version["webSocketDebuggerUrl"], max_size=None) as socket:
        browser = Connection(socket)
        created = await browser.result("Target.createTarget", {"url": "about:blank"})
        target_id = created["targetId"]
        try:
            page_info = next(row for row in read_endpoint(args.endpoint, "/json/list") if row["id"] == target_id)
            async with websockets.connect(page_info["webSocketDebuggerUrl"], max_size=None) as page_socket:
                page = Connection(page_socket)
                observations = (await page.result("Runtime.evaluate", {
                    "expression": "({viewport:{width:innerWidth,height:innerHeight,deviceScaleFactor:devicePixelRatio},window:{x:screenX,y:screenY,outerWidth,outerHeight},secureContextState:isSecureContext,isolationState:crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus()})",
                    "returnByValue": True,
                }))["result"]["value"]
                metadata = {
                    "schemaVersion": 1,
                    "chromeVersion": target["chrome_version"],
                    "chromiumRevision": target["main_branch_revision"],
                    "chromiumCommit": target["chromium_commit"],
                    "v8Version": version["V8-Version"],
                    "platform": target["platform"],
                    "browserMode": mode,
                    "commandLineFeatureOverrides": [],
                    "environmentProfileId": f"chrome-152-windows-x64-{mode}-controlled-v1",
                    "profileFreshness": "fresh-controlled",
                    **observations,
                }
                rows = []
                for name, method, params in CASES:
                    response = await page.send(method, params)
                    # Success payloads contain node/realm IDs unrelated to wire parsing.
                    rows.append({"name": name, "method": method, "params": params,
                                 "response": {"error": response["error"]} if "error" in response else {"success": True}})
        finally:
            await browser.result("Target.closeTarget", {"targetId": target_id})
    source = ROOT / "internal/cdp/protocol/chrome152.json"
    raw = write_json(source, protocol)
    write_json(source.with_suffix(".source.json"), {
        "schemaVersion": 1,
        "source": "Chrome /json/protocol",
        "sourcePath": source.relative_to(ROOT).as_posix(),
        "sha256": hashlib.sha256(raw).hexdigest(),
        "captureMetadata": metadata,
    })
    write_json(ROOT / "internal/cdp/testdata/protocol_params_chrome152.json", {"captureMetadata": metadata, "cases": rows})
    print(f"Captured {len(protocol['domains'])} domains from {expected}; run python tools/generate_cdp.py")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--browser-mode", choices=("headful", "headless"), default="headful")
    parser.add_argument("--fresh-controlled-profile", action="store_true", required=True,
                        help="assert Chrome was launched with a fresh controlled profile and no feature overrides")
    asyncio.run(capture(parser.parse_args()))
