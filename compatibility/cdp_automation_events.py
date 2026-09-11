"""Measure CDP event enable/disable isolation and lifecycle-idle timing.

Uses two raw flattened sessions attached to one owned page. Event receipt times
are recorded separately from Chrome's event timestamps; the latter denote the
underlying activity boundary and need not be the event-delivery time.
"""
from __future__ import annotations

import argparse
import asyncio
from collections import Counter
import contextlib
import json
from pathlib import Path
import threading
import time

import websockets

from cdp_automation import Fixture, ThreadingHTTPServer, capture_oracle_metadata, write_json
from oracle import endpoint_version


class Connection:
    def __init__(self, websocket):
        self.websocket = websocket
        self.pending = {}
        self.events = []
        self.commands = []
        self.next_id = 0
        self.start = time.perf_counter()
        self.reader = asyncio.create_task(self.read())

    async def read(self):
        async for message in self.websocket:
            payload = json.loads(message)
            if "id" in payload:
                future = self.pending.pop(payload["id"], None)
                if future and not future.done():
                    future.set_result(payload)
            else:
                self.events.append({"receivedMs": round((time.perf_counter() - self.start) * 1000, 2),
                                    **payload})

    async def send(self, method, params=None, session=None):
        self.next_id += 1
        payload = {"id": self.next_id, "method": method}
        if params is not None:
            payload["params"] = params
        if session:
            payload["sessionId"] = session
        future = asyncio.get_running_loop().create_future()
        self.pending[self.next_id] = future
        await self.websocket.send(json.dumps(payload))
        response = await asyncio.wait_for(future, 5)
        self.commands.append({"request": payload, "response": response})
        if "error" in response:
            raise RuntimeError(method + ": " + json.dumps(response["error"]))
        return response.get("result", {})


async def run(args):
    version = endpoint_version(args.endpoint)
    server = ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    origin = "http://127.0.0.1:" + str(server.server_port)
    result = {"schemaVersion": 1, "target": {"endpoint": args.endpoint, "version": version},
              "fixture": {"origin": origin, "controlledLoopback": True}, "scenarios": []}
    try:
        if args.oracle:
            result["captureMetadata"] = await capture_oracle_metadata(args.endpoint, origin)
        async with websockets.connect(version["webSocketDebuggerUrl"], max_size=None) as websocket:
            connection = Connection(websocket)
            target = (await connection.send("Target.createTarget", {"url": "about:blank"}))["targetId"]
            try:
                a = (await connection.send("Target.attachToTarget", {"targetId": target, "flatten": True}))["sessionId"]
                b = (await connection.send("Target.attachToTarget", {"targetId": target, "flatten": True}))["sessionId"]
                labels = {a: "a", b: "b"}

                async def sample(name, operation, delay=1.4):
                    await asyncio.sleep(.05)
                    begin = len(connection.events)
                    await operation()
                    await asyncio.sleep(delay)
                    events = connection.events[begin:]
                    counts = {}
                    for session in (a, b):
                        counts[labels[session]] = dict(sorted(Counter(
                            event["method"] for event in events if event.get("sessionId") == session).items()))
                    result["scenarios"].append({"name": name, "eventsBySession": counts, "events": events})

                async def navigate(name):
                    return await connection.send("Page.navigate", {"url": origin + "/page/" + name}, a)

                await sample("all_domains_disabled", lambda: navigate("disabled"))
                for method in ("Page.enable", "Runtime.enable", "Network.enable"):
                    await connection.send(method, {}, a)
                await connection.send("Page.setLifecycleEventsEnabled", {"enabled": True}, a)
                await sample("a_enabled_b_disabled", lambda: navigate("enabled"))
                await sample("b_runtime_enable_snapshot", lambda: connection.send("Runtime.enable", {}, b), .1)
                for method in ("Page.disable", "Runtime.disable", "Network.disable"):
                    await connection.send(method, {}, a)
                await sample("a_disabled_b_runtime_enabled", lambda: navigate("disabled-again"))
                for method in ("Page.enable", "Runtime.enable", "Network.enable"):
                    await connection.send(method, {}, a)
                await connection.send("Page.setLifecycleEventsEnabled", {"enabled": True}, a)

                async def fetch_after_load():
                    await navigate("delayed-request")
                    await asyncio.sleep(.1)
                    return await connection.send("Runtime.evaluate", {
                        "expression": "fetch('/api/delay?ms=650').then(r => r.text())",
                        "awaitPromise": False, "returnByValue": True}, a)
                await sample("idle_with_request_after_navigation", fetch_after_load, 2)
            finally:
                await connection.send("Target.closeTarget", {"targetId": target})
                result["commands"] = connection.commands
                connection.reader.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await connection.reader
    finally:
        server.shutdown()
        server.server_close()
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--oracle", action="store_true")
    arguments = parser.parse_args()
    capture = asyncio.run(run(arguments))
    write_json(Path(arguments.output), capture)
    print(json.dumps([{key: value for key, value in row.items() if key != "events"}
                      for row in capture["scenarios"]], indent=2))
