"""Capture pinned Chrome Fetch/legacy interception-stage behavior on local traffic."""
import argparse
import asyncio
from collections import Counter
import contextlib
import json
from pathlib import Path
import threading

import websockets

from cdp_automation import Fixture, ThreadingHTTPServer, capture_oracle_metadata, write_json
from cdp_automation_events import Connection
from oracle import endpoint_version


def comparison_shape(row):
    ids = [pause.get("interceptionId", pause.get("requestId")) for pause in row.get("paused", [])]
    headers = ["entries" if isinstance(pause["responseHeaders"], list) else "object"
               for pause in row.get("paused", []) if "responseHeaders" in pause]
    return {"loaded": row["loaded"], "stages": row["stages"],
            "sameIdAcrossStages": len(set(ids)) <= 1, "responseHeaderShapes": headers}


def compare(reference, actual):
    if reference.get("captureMetadata", {}).get("browserMode") != "headful":
        raise ValueError("A provenanced headful oracle reference is required")
    expected = {row["name"]: row for row in reference["scenarios"]}
    rows = []
    for observed in actual["scenarios"]:
        name = observed["name"]
        oracle = expected.get(name)
        expectation = comparison_shape(oracle) if oracle else None
        observation = comparison_shape(observed)
        rows.append({"name": name, "classification": "match" if expectation == observation else "divergence",
                     "expected": expectation, "observed": observation})
    return {"summary": dict(Counter(row["classification"] for row in rows)), "scenarios": rows}


async def run(args):
    version = endpoint_version(args.endpoint)
    server = ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    origin = "http://127.0.0.1:" + str(server.server_port)
    rows = []
    result = {"schemaVersion": 1, "version": version, "origin": origin}
    try:
        if args.oracle:
            result["captureMetadata"] = await capture_oracle_metadata(args.endpoint, origin)
        async with websockets.connect(version["webSocketDebuggerUrl"], max_size=None) as websocket:
            connection = Connection(websocket)
            target = (await connection.send("Target.createTarget", {"url": "about:blank"}))["targetId"]
            sid = (await connection.send("Target.attachToTarget", {"targetId": target, "flatten": True}))["sessionId"]
            try:
                await connection.send("Page.enable", {}, sid)
                await connection.send("Network.enable", {}, sid)
                cases = [
                    ("fetch_default", "Fetch", {}, None),
                    ("fetch_empty_patterns", "Fetch", {"patterns": []}, None),
                    ("fetch_request", "Fetch", {"patterns": [{"requestStage": "Request"}]}, None),
                    ("fetch_response", "Fetch", {"patterns": [{"requestStage": "Response"}]}, None),
                    ("fetch_wrong_resource", "Fetch", {"patterns": [{"resourceType": "Script"}]}, None),
                    ("fetch_wrong_url", "Fetch", {"patterns": [{"urlPattern": "*never-match*"}]}, None),
                    ("fetch_url_question_wildcard", "Fetch", {"patterns": [{"urlPattern": "*/page/prob?"}]}, None),
                    ("fetch_url_escape_wildcard", "Fetch", {"patterns": [{"urlPattern": "*/page/prob\\*"}]}, None),
                    ("fetch_override_response_true", "Fetch", {"patterns": [{"requestStage": "Request"}]}, True),
                    ("fetch_both_stages_override_false", "Fetch", {"patterns": [{"requestStage": "Request"}, {"requestStage": "Response"}]}, False),
                    ("fetch_disable_pending_request", "Fetch", {}, "disable"),
                    ("fetch_disable_pending_response", "Fetch", {"patterns": [{"requestStage": "Response"}]}, "disable"),
                    ("legacy_request", "Network", {"patterns": [{"urlPattern": "*"}]}, None),
                    ("legacy_response", "Network", {"patterns": [{"urlPattern": "*", "interceptionStage": "HeadersReceived"}]}, None),
                    ("legacy_disable_pending", "Network", {"patterns": [{"urlPattern": "*"}]}, "disable"),
                ]
                for name, domain, params, override in cases:
                    begin = len(connection.events)
                    if domain == "Fetch":
                        await connection.send("Fetch.enable", params, sid)
                    else:
                        await connection.send("Network.setRequestInterception", params, sid)
                    navigation = asyncio.create_task(connection.send("Page.navigate", {"url": origin + "/page/probe"}, sid))
                    seen = set()
                    paused = []
                    deadline = asyncio.get_running_loop().time() + 2
                    loaded = False
                    while asyncio.get_running_loop().time() < deadline:
                        for event in connection.events[begin:]:
                            if event.get("sessionId") != sid:
                                continue
                            if event["method"] == "Page.loadEventFired":
                                loaded = True
                            if event["method"] not in ("Fetch.requestPaused", "Network.requestIntercepted"):
                                continue
                            p = event["params"]
                            key = p.get("requestId", p.get("interceptionId"))
                            response_stage = "responseStatusCode" in p
                            key = (key, response_stage)
                            if key in seen:
                                continue
                            seen.add(key)
                            paused.append(p)
                            if override == "disable":
                                await connection.send("Fetch.disable" if domain == "Fetch" else "Network.setRequestInterception",
                                                      {} if domain == "Fetch" else {"patterns": []}, sid)
                            elif domain == "Fetch":
                                continuation = {"requestId": p["requestId"]}
                                if isinstance(override, bool) and not response_stage:
                                    continuation["interceptResponse"] = override
                                await connection.send("Fetch.continueRequest", continuation, sid)
                            else:
                                await connection.send("Network.continueInterceptedRequest", {"interceptionId": p["interceptionId"]}, sid)
                        if loaded and navigation.done():
                            break
                        await asyncio.sleep(.01)
                    await connection.send("Fetch.disable" if domain == "Fetch" else "Network.setRequestInterception",
                                          {} if domain == "Fetch" else {"patterns": []}, sid)
                    nav = await navigation
                    rows.append({"name": name, "loaded": loaded, "navigation": nav,
                                 "stages": ["Response" if "responseStatusCode" in p else "Request" for p in paused],
                                 "paused": paused})

                begin = len(connection.events)
                await connection.send("Fetch.enable", {}, sid)
                navigation = asyncio.create_task(connection.send("Page.navigate", {"url": origin + "/page/detached"}, sid))
                paused = None
                for _ in range(200):
                    paused = next((event["params"] for event in connection.events[begin:]
                                   if event.get("sessionId") == sid and event["method"] == "Fetch.requestPaused"), None)
                    if paused:
                        break
                    await asyncio.sleep(.01)
                if not paused:
                    raise RuntimeError("detach probe never paused")
                await connection.send("Target.detachFromTarget", {"sessionId": sid})
                sid = (await connection.send("Target.attachToTarget", {"targetId": target, "flatten": True}))["sessionId"]
                await asyncio.sleep(.2)
                observed = await connection.send("Runtime.evaluate", {"expression": "document.title", "returnByValue": True}, sid)
                rows.append({"name": "fetch_detach_pending_request", "loaded": observed.get("result", {}).get("value") == "Automation detached",
                             "stages": ["Request"], "observed": observed, "paused": [paused]})
                await asyncio.gather(navigation, return_exceptions=True)
            finally:
                await connection.send("Target.closeTarget", {"targetId": target})
                connection.reader.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await connection.reader
    finally:
        server.shutdown()
        server.server_close()
    result["scenarios"] = rows
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--oracle", action="store_true")
    parser.add_argument("--reference", type=Path)
    parser.add_argument("--allow-failures", action="store_true")
    arguments = parser.parse_args()
    report = asyncio.run(run(arguments))
    if arguments.reference:
        report["comparison"] = compare(json.loads(arguments.reference.read_text(encoding="utf-8")), report)
    write_json(Path(arguments.output), report)
    print(json.dumps([{key: row[key] for key in ("name", "loaded", "stages")} for row in report["scenarios"]], indent=2))
    failed = any(not row["loaded"] for row in report["scenarios"]) or report.get("comparison", {}).get("summary", {}).get("divergence", 0)
    raise SystemExit(int(bool(failed) and not arguments.allow_failures))
