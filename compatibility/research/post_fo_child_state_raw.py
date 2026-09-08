"""Passive post-/fo child-state capture over a direct page CDP socket."""

import argparse
import asyncio
import json
import time
import urllib.request
from pathlib import Path
from urllib.parse import urlsplit

import websockets

from post_fo_child_state import CHILD_SNAPSHOT, INSTRUMENT, MARKER, clean_url


def page_target(endpoint):
    with urllib.request.urlopen(endpoint.rstrip("/") + "/json/list") as response:
        targets = json.load(response)
    candidates = [target for target in targets if target.get("type") == "page" and target.get("url") == "about:blank"]
    if not candidates:
        raise RuntimeError("No about:blank page target is available")
    return candidates[0]


class DirectCDP:
    def __init__(self, socket):
        self.socket = socket
        self.sequence = 0
        self.pending = {}
        self.handlers = []

    async def call(self, method, params=None):
        self.sequence += 1
        command_id = self.sequence
        future = asyncio.get_running_loop().create_future()
        self.pending[command_id] = future
        await self.socket.send(json.dumps({"id":command_id,"method":method,"params":params or {}}))
        return await future

    async def read(self):
        async for encoded in self.socket:
            message = json.loads(encoded)
            command_id = message.get("id")
            if command_id is not None:
                future = self.pending.pop(command_id, None)
                if future and not future.done():
                    if "error" in message:
                        future.set_exception(RuntimeError(message["error"].get("message", str(message["error"]))))
                    else:
                        future.set_result(message.get("result", {}))
                continue
            for handler in tuple(self.handlers):
                handler(message.get("method", ""), message.get("params", {}))


async def capture(args):
    target = page_target(args.endpoint)
    started = time.monotonic()
    network, boundaries, child_samples, errors = [], [], [], []
    contexts = {}
    frames = {}
    top_frame_id = ""
    message_sequence = 0
    sample_lock = asyncio.Lock()

    def elapsed():
        return round((time.monotonic() - started) * 1000, 3)

    async with websockets.connect(target["webSocketDebuggerUrl"], max_size=None,
                                  ping_interval=None, ping_timeout=None) as socket:
        cdp = DirectCDP(socket)
        reader = asyncio.create_task(cdp.read())

        async def sample_child(reason):
            async with sample_lock:
                candidates = [(context_id, info) for context_id, info in contexts.items()
                    if info.get("isDefault") and info.get("frameId") and info.get("frameId") != top_frame_id]
                if not candidates:
                    return
                context_id, info = candidates[-1]
                try:
                    response = await asyncio.wait_for(cdp.call("Runtime.evaluate", {
                        "expression":f"({CHILD_SNAPSHOT})()","contextId":context_id,
                        "returnByValue":True,"awaitPromise":True}), timeout=2)
                    value = response.get("result", {}).get("value")
                    if isinstance(value, dict):
                        child_samples.append({"observedMs":elapsed(),"reason":reason,
                            "messageSequence":int(reason.split("-")[-1]) if reason.startswith("message-") else None,
                            "frameId":info.get("frameId"),"contextId":context_id,**value})
                except Exception as error:
                    errors.append(f"child sample {reason}: {error}")

        def event(method, params):
            nonlocal top_frame_id, message_sequence
            if method == "Runtime.executionContextCreated":
                context = params.get("context", {})
                auxiliary = context.get("auxData", {})
                contexts[context.get("id")] = {"frameId":auxiliary.get("frameId"),
                    "isDefault":auxiliary.get("isDefault", True),"origin":context.get("origin", "")}
            elif method in {"Runtime.executionContextsCleared", "Runtime.executionContextDestroyed"}:
                if method == "Runtime.executionContextsCleared":
                    contexts.clear()
                else:
                    contexts.pop(params.get("executionContextId"), None)
            elif method == "Page.frameNavigated":
                frame = params.get("frame", {})
                frames[frame.get("id", "")] = frame
                if not frame.get("parentId"):
                    top_frame_id = frame.get("id", "")
            elif method == "Runtime.consoleAPICalled":
                arguments = params.get("args", [])
                text = arguments[0].get("value") if arguments else None
                if not isinstance(text, str) or not text.startswith(MARKER):
                    return
                try:
                    value = json.loads(text[len(MARKER):])
                    value["observedMs"] = elapsed()
                    value["executionContextId"] = params.get("executionContextId")
                    if not value.get("child") and value.get("phase") == "message-after-handlers":
                        message_sequence += 1
                        value["messageSequence"] = message_sequence
                        asyncio.create_task(sample_child(f"message-{message_sequence}"))
                    boundaries.append(value)
                except Exception as error:
                    errors.append(f"boundary decode: {error}")
            elif method == "Network.requestWillBeSent":
                request = params.get("request", {})
                network.append({"observedMs":elapsed(),"kind":"request",
                    "method":request.get("method"),"resourceType":params.get("type"),
                    "url":clean_url(request.get("url", "")),
                    "bodyLength":len(request.get("postData", ""))})
            elif method == "Network.responseReceived":
                response = params.get("response", {})
                network.append({"observedMs":elapsed(),"kind":"response",
                    "status":response.get("status"),"url":clean_url(response.get("url", ""))})

        cdp.handlers.append(event)
        try:
            await cdp.call("Page.enable")
            await cdp.call("Runtime.enable")
            await cdp.call("Network.enable")
            if args.clear_state:
                await cdp.call("Network.clearBrowserCookies")
                await cdp.call("Network.clearBrowserCache")
            tree = (await cdp.call("Page.getFrameTree")).get("frameTree", {}).get("frame", {})
            top_frame_id = tree.get("id", "")
            await cdp.call("Page.addScriptToEvaluateOnNewDocument", {"source":f"({INSTRUMENT})()"})
            await cdp.call("Page.navigate", {"url":args.url})
            await asyncio.sleep(args.settle_ms / 1000)
            try:
                final = await asyncio.wait_for(cdp.call("Runtime.evaluate", {
                    "expression":"({url:location.href,title:document.title,cookie:document.cookie})",
                    "returnByValue":True}), timeout=2)
                final_state = final.get("result", {}).get("value", {})
            except Exception as error:
                final_state = {"error":str(error) or type(error).__name__}
            result = {"endpoint":args.endpoint,"targetId":target.get("id"),
                "finalState":final_state,"frames":frames,"contexts":contexts,
                "network":network,"boundaries":boundaries,
                "childSamples":child_samples,"errors":errors}
            encoded = json.dumps(result, ensure_ascii=False, indent=2)
            if args.output:
                Path(args.output).write_text(encoded + "\n", encoding="utf-8")
            print(json.dumps({"finalState":final_state,
                "acceptedFO":any(item.get("kind")=="response" and item.get("status")==200 and "/fo/" in item.get("url","") for item in network),
                "foRequests":sum(item.get("kind")=="request" and "/fo/" in item.get("url","") for item in network),
                "topMessages":message_sequence,"childSamples":len(child_samples),
                "errors":errors,"output":args.output}, ensure_ascii=False, indent=2))
        finally:
            reader.cancel()
            await asyncio.gather(reader, return_exceptions=True)


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", required=True)
    parser.add_argument("--settle-ms", type=int, default=5_000)
    parser.add_argument("--clear-state", action="store_true")
    parser.add_argument("--output")
    await capture(parser.parse_args())


if __name__ == "__main__":
    asyncio.run(main())
