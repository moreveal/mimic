"""Real automation-client compatibility gate and transparent CDP wire recorder.

The fixture serves only controlled loopback content. See cdp_automation.md for
setup, pinned client versions, oracle capture and comparison commands.
"""

from __future__ import annotations

import argparse
import asyncio
from collections import Counter
import contextlib
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import logging
from pathlib import Path
import platform
import sys
import threading
import time
import urllib.parse
import urllib.request

import pyppeteer
import websockets

from oracle import capture_metadata, default_profile_id, endpoint_version, prepare_page

ROOT = Path(__file__).resolve().parents[1]
PUPPETEER_VERSION = "25.10.0"
PYPPETEER_VERSION = "2.0.0"
TIMEOUT = 8
CHECK_NAMES = (
    "browser.connect", "browser.version", "page.create", "navigation.goto",
    "runtime.by_value", "runtime.promise", "runtime.handles_properties",
    "runtime.handle_identity", "runtime.handle_dispose", "runtime.exception",
    "runtime.unserializable_arguments", "dom.selectors_element_evaluation",
    "dom.form_input_select_dom_click", "input.pointer_click", "runtime.console_arguments",
    "runtime.pageerror", "page.init_script", "navigation.reload", "navigation.history",
    "page.child_frame_events_evaluation", "network.cookies",
    "network.response_body_extra_headers", "network.interception_fulfill_body",
    "navigation.networkidle0", "page.set_content", "dom.wait_for_selector_mutation",
    "runtime.exposed_function_navigation", "page.init_script_removal", "emulation.viewport",
    "network.user_agent_override", "network.interception_continue_post", "network.interception_abort",
    "target.parallel_pages_isolation",
    "target.browser_context_storage_isolation",
)


class Fixture(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_GET(self):
        self.respond()

    def do_POST(self):
        self.respond()

    def respond(self):
        url = urllib.parse.urlsplit(self.path)
        if url.path.startswith("/api/"):
            if url.path == "/api/delay":
                delay = int(urllib.parse.parse_qs(url.query).get("ms", ["0"])[0])
                time.sleep(min(max(delay, 0), 2000) / 1000)
            body = self.rfile.read(int(self.headers.get("content-length", "0")))
            payload = json.dumps({"path": url.path, "method": self.command,
                                  "body": body.decode("utf-8"),
                                  "header": self.headers.get("x-automation", ""),
                                  "userAgent": self.headers.get("user-agent", ""),
                                  "cookie": self.headers.get("cookie", "")}).encode()
            content_type = "application/json"
        else:
            name = url.path.rstrip("/").rsplit("/", 1)[-1] or "a"
            payload = ("""<!doctype html><html><head><meta charset="utf-8">
<title>Automation NAME</title><link rel="icon" href="data:,"><script>
window.fixtureInit = window.automationInit || null;
// This fixture deliberately exercises network history reloads. Pyppeteer 2.0
// cannot observe BFCache restoration (it updates loaderId only on lifecycle init).
window.addEventListener('unload', function () {});
</script></head><body><h1 id="heading">Fixture NAME</h1>
<ul><li class="item">first</li><li class="item">second</li></ul>
<form id="form"><label>Name<input id="name" name="name"></label>
<select id="choice" name="choice"><option value="a">A</option><option value="b">B</option></select>
<button id="button" type="button" onclick="window.clicked=(window.clicked||0)+1">Go</button></form>
<div id="result"></div></body></html>""".replace("NAME", name)).encode()
            content_type = "text/html; charset=utf-8"
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("Permissions-Policy", "unload=(self)")
        self.end_headers()
        with contextlib.suppress(BrokenPipeError, ConnectionResetError):
            self.wfile.write(payload)


def write_json(path: Path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


async def capture_oracle_metadata(endpoint, origin):
    browser = await pyppeteer.connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    try:
        await page.goto(origin + "/page/metadata")
        await prepare_page(browser, page, "headful", "1280x800")
        return await capture_metadata(endpoint, browser, page, mode="headful",
                                      environment_profile_id=default_profile_id("headful"))
    finally:
        await page.close()
        await browser.disconnect()


class WireRecorder:
    """Record actual transport messages, including legacy nested Target traffic.

    A browser socket and each flattened/nested session have separate ID spaces.
    Parameters/results remain in the trace; the summary counts logical requests
    and explicitly reports requests still unanswered at disconnect.
    """

    def __init__(self, destination: str, path: Path):
        self.destination = destination
        self.path = path
        self.records = []
        self.pending = {}
        self.methods = {}
        self.events = Counter()
        self.connection_id = 0

    def observe(self, direction: str, message, connection: int, session=""):
        if isinstance(message, (bytes, str)):
            message = json.loads(message)
        session = message.get("sessionId", session)
        self.records.append({"direction": direction, "connection": connection,
                             "session": session, "message": message})
        method = message.get("method")
        key = (connection, session, message.get("id"))
        if direction == "send" and method:
            entry = self.methods.setdefault(method, {"calls": 0, "responses": 0,
                                                      "errors": [], "unanswered": 0})
            entry["calls"] += 1
            self.pending[key] = method
        elif direction == "receive" and "id" in message:
            pending = self.pending.pop(key, None)
            if pending:
                entry = self.methods[pending]
                entry["responses"] += 1
                if "error" in message:
                    entry["errors"].append(message["error"])
        elif direction == "receive" and method:
            self.events[method] += 1
        if method in ("Target.sendMessageToTarget", "Target.receivedMessageFromTarget"):
            params = message.get("params", {})
            self.observe(direction, params["message"], connection,
                         params.get("sessionId", session))

    async def proxy(self, client, _path):
        self.connection_id += 1
        connection = self.connection_id
        async with websockets.connect(self.destination, max_size=None,
                                      ping_interval=None) as upstream:
            async def pump(source, destination, direction):
                async for message in source:
                    self.observe(direction, message, connection)
                    await destination.send(message)
            tasks = [asyncio.create_task(pump(client, upstream, "send")),
                     asyncio.create_task(pump(upstream, client, "receive"))]
            done, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
            for task in pending:
                task.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)

    def finish(self):
        for method in self.pending.values():
            self.methods[method]["unanswered"] += 1
        self.path.write_text("".join(json.dumps(row, ensure_ascii=False) + "\n"
                                     for row in self.records), encoding="utf-8")
        return {"trace": str(self.path), "methods": dict(sorted(self.methods.items())),
                "events": dict(sorted(self.events.items()))}


class Checks:
    def __init__(self, output):
        self.results = []
        self.output = Path(output)

    async def run(self, name, callback, expected=None):
        start = time.perf_counter()
        try:
            observed = await asyncio.wait_for(callback(), TIMEOUT)
            if expected is not None and observed != expected:
                row = {"status": "fail", "observed": observed, "expected": expected}
            else:
                row = {"status": "pass", "observed": observed}
        except Exception as exc:
            row = {"status": "fail", "error": type(exc).__name__ + ": " + str(exc)}
        row.update(name=name, durationMs=round((time.perf_counter() - start) * 1000))
        self.results.append(row)
        write_json(self.output, {"client": "pyppeteer", "version": pyppeteer.__version__,
                                 "checks": self.results})
        return row["status"] == "pass"


async def pyppeteer_worker(args):
    if pyppeteer.__version__ != PYPPETEER_VERSION:
        raise RuntimeError(f"Expected Pyppeteer {PYPPETEER_VERSION}, got {pyppeteer.__version__}")
    checks = Checks(args.output)
    browser = None
    pages, contexts = [], []
    base = args.base_url
    try:
        async def connect():
            nonlocal browser
            browser = await pyppeteer.connect(browserWSEndpoint=args.endpoint,
                                               defaultViewport=None)
            return True
        connected = await checks.run("browser.connect", connect, True)
        if not connected:
            return {"client": "pyppeteer", "version": pyppeteer.__version__,
                    "checks": checks.results, "blocked": "browser.connect"}
        async def browser_version():
            return (await browser.version()).endswith("/152.0.7977.82")
        await checks.run("browser.version", browser_version, True)

        async def new_page():
            page = await browser.newPage()
            page.setDefaultNavigationTimeout(TIMEOUT * 800)
            pages.append(page)
            return page

        page = None
        async def create():
            nonlocal page
            page = await new_page()
            return page.url
        created = await checks.run("page.create", create, "about:blank")
        if not created:
            return {"client": "pyppeteer", "version": pyppeteer.__version__,
                    "checks": checks.results, "blocked": "page.create"}

        async def navigate(path="a", wait_until="load"):
            response = await page.goto(base + "/page/" + path,
                                       {"waitUntil": wait_until})
            return {"status": response.status, "title": await page.title()}
        await checks.run("navigation.goto", navigate, {"status": 200, "title": "Automation a"})
        await checks.run("runtime.by_value", lambda: page.evaluate(
            "() => ({number: 42, nested: [true, null, {text: 'value'}]})"),
            {"number": 42, "nested": [True, None, {"text": "value"}]})
        await checks.run("runtime.promise", lambda: page.evaluate(
            "() => new Promise(resolve => setTimeout(() => resolve(42), 20))"), 42)

        async def handles():
            handle = await page.evaluateHandle("() => ({a: 7, nested: {label: 'nested'}})")
            props = await handle.getProperties()
            nested = await handle.getProperty("nested")
            try:
                return {"keys": sorted(props), "a": await props["a"].jsonValue(),
                        "nested": await nested.jsonValue()}
            finally:
                await asyncio.gather(*(item.dispose() for item in props.values()),
                                     nested.dispose(), handle.dispose())
        await checks.run("runtime.handles_properties", handles,
                         {"keys": ["a", "nested"], "a": 7, "nested": {"label": "nested"}})

        async def identity():
            first = await page.evaluateHandle("() => globalThis.identityObject = {x: 1}")
            second = await page.evaluateHandle("() => globalThis.identityObject")
            try:
                return await page.evaluate("(a, b) => a === b", first, second)
            finally:
                await first.dispose()
                await second.dispose()
        await checks.run("runtime.handle_identity", identity, True)

        async def disposed():
            handle = await page.evaluateHandle("() => ({x: 1})")
            await handle.dispose()
            try:
                await page.evaluate("value => value.x", handle)
                return False
            except Exception as exc:
                return "disposed" in str(exc).lower()
        await checks.run("runtime.handle_dispose", disposed, True)

        async def exception():
            try:
                await page.evaluate("() => { throw new TypeError('automation failure'); }")
                return False
            except Exception as exc:
                return "automation failure" in str(exc) and "TypeError" in str(exc)
        await checks.run("runtime.exception", exception, True)
        async def special_arguments():
            # Pyppeteer 2.0 serializes Python float('nan') as invalid JSON NaN.
            # Remote special-value handles exercise supported client behavior.
            handles = [await page.evaluateHandle(value) for value in ("NaN", "Infinity", "-0")]
            try:
                return await page.evaluate(
                    "(a, b, c) => [Number.isNaN(a), b === Infinity, Object.is(c, -0)]", *handles)
            finally:
                await asyncio.gather(*(handle.dispose() for handle in handles))
        await checks.run("runtime.unserializable_arguments", special_arguments, [True, True, True])

        async def selectors():
            element = await page.querySelector("#heading")
            try:
                return {"heading": await page.evaluate("el => el.textContent", element),
                        "items": await page.querySelectorAllEval(".item", "els => els.map(el => el.textContent)")}
            finally:
                await element.dispose()
        await checks.run("dom.selectors_element_evaluation", selectors,
                         {"heading": "Fixture a", "items": ["first", "second"]})

        async def form():
            await page.type("#name", "Alpha 42")
            selected = await page.select("#choice", "b")
            await page.querySelectorEval("#button", "el => el.click()")
            return {"value": await page.querySelectorEval("#name", "el => el.value"),
                    "selected": selected, "clicked": await page.evaluate("window.clicked")}
        await checks.run("dom.form_input_select_dom_click", form,
                         {"value": "Alpha 42", "selected": ["b"], "clicked": 1})

        async def mouse():
            await page.click("#button")
            return await page.evaluate("window.clicked")
        await checks.run("input.pointer_click", mouse, 2)

        async def console():
            messages = []
            page.on("console", messages.append)
            try:
                await page.evaluate("() => console.log('automation-console', {answer: 42})")
                for _ in range(20):
                    if messages:
                        break
                    await asyncio.sleep(.01)
                message = next(m for m in messages if "automation-console" in m.text)
                return {"type": message.type, "arguments": [await a.jsonValue() for a in message.args]}
            finally:
                page.remove_listener("console", messages.append)
        await checks.run("runtime.console_arguments", console,
                         {"type": "log", "arguments": ["automation-console", {"answer": 42}]})

        async def pageerror():
            errors = []
            page.on("pageerror", errors.append)
            try:
                await page.evaluate("() => { setTimeout(() => { throw new TypeError('automation-pageerror'); }, 30); }")
                for _ in range(100):
                    if errors:
                        break
                    await asyncio.sleep(.02)
                if not errors:
                    raise AssertionError("No pageerror event for uncaught timer TypeError")
                return str(errors[0]).splitlines()[0]
            finally:
                page.remove_listener("pageerror", errors.append)
        await checks.run("runtime.pageerror", pageerror, "Uncaught TypeError: automation-pageerror")

        async def init_script():
            await page.evaluateOnNewDocument("() => { window.automationInit = 'installed'; }")
            await navigate("b")
            return await page.evaluate("window.fixtureInit")
        await checks.run("page.init_script", init_script, "installed")

        async def reload():
            await page.evaluate("window.transientValue = 1")
            response = await page.reload({"waitUntil": "load"})
            return {"status": response.status, "value": await page.evaluate(
                "() => [typeof window.transientValue, window.fixtureInit]")}
        await checks.run("navigation.reload", reload,
                         {"status": 200, "value": ["undefined", "installed"]})

        async def history():
            await navigate("a")
            await navigate("b")
            await page.goBack({"waitUntil": "load"})
            back = await page.title()
            await page.goForward({"waitUntil": "load"})
            return [back, await page.title()]
        await checks.run("navigation.history", history, ["Automation a", "Automation b"])

        async def frames():
            events = []
            page.on("frameattached", lambda f: events.append("attached"))
            page.on("framenavigated", lambda f: events.append("navigated"))
            await page.evaluate("url => { const f = document.createElement('iframe'); f.name = 'child'; f.src = url; document.body.append(f); }", base + "/page/child")
            for _ in range(100):
                child = next((f for f in page.frames if f.name == "child" and f.url.endswith("/child")), None)
                if child:
                    break
                await asyncio.sleep(.02)
            if child is None:
                raise AssertionError("child frame not discovered")
            return {"title": await child.evaluate("document.title"),
                    "parent": child.parentFrame == page.mainFrame,
                    "attached": "attached" in events, "navigated": "navigated" in events}
        await checks.run("page.child_frame_events_evaluation", frames,
                         {"title": "Automation child", "parent": True, "attached": True, "navigated": True})

        async def cookies():
            await page.setCookie({"name": "automation", "value": "cookie", "url": base})
            values = await page.cookies(base)
            visible = await page.evaluate("document.cookie")
            await page.deleteCookie({"name": "automation", "url": base})
            return {"cookie": next(c["value"] for c in values if c["name"] == "automation"),
                    "visible": "automation=cookie" in visible,
                    "removed": "automation=cookie" not in await page.evaluate("document.cookie")}
        await checks.run("network.cookies", cookies,
                         {"cookie": "cookie", "visible": True, "removed": True})

        async def response_body():
            await page.setExtraHTTPHeaders({"x-automation": "header"})
            response = await page.goto(base + "/api/body")
            value = await response.json()
            return {"status": response.status, "header": value["header"], "path": value["path"]}
        await checks.run("network.response_body_extra_headers", response_body,
                         {"status": 200, "header": "header", "path": "/api/body"})

        async def interception():
            tasks = []
            async def intercept(request):
                if request.url.endswith("/api/intercept"):
                    await request.respond({"status": 201, "contentType": "application/json",
                                           "body": '{"intercepted":true}'})
                else:
                    await request.continue_()
            listener = lambda request: tasks.append(asyncio.create_task(intercept(request)))
            await page.setRequestInterception(True)
            page.on("request", listener)
            try:
                response = await page.goto(base + "/api/intercept")
                result = {"status": response.status, "body": await response.json()}
                await asyncio.gather(*tasks)
                return result
            finally:
                page.remove_listener("request", listener)
                await page.setRequestInterception(False)
                await asyncio.gather(*tasks, return_exceptions=True)
        await checks.run("network.interception_fulfill_body", interception,
                         {"status": 201, "body": {"intercepted": True}})
        await checks.run("navigation.networkidle0", lambda: navigate("a", "networkidle0"),
                         {"status": 200, "title": "Automation a"})

        async def set_content():
            await page.setContent("<!doctype html><title>Automation content</title><main id='content'>inserted</main>")
            return {"title": await page.title(), "text": await page.querySelectorEval("#content", "el => el.textContent")}
        await checks.run("page.set_content", set_content, {"title": "Automation content", "text": "inserted"})

        async def wait_selector():
            await page.evaluate("() => setTimeout(() => { const el = document.createElement('p'); el.id = 'delayed'; el.textContent = 'arrived'; document.body.append(el); }, 30)")
            element = await page.waitForSelector("#delayed", {"timeout": 3000})
            try:
                return await page.evaluate("el => el.textContent", element)
            finally:
                await element.dispose()
        await checks.run("dom.wait_for_selector_mutation", wait_selector, "arrived")

        async def exposed_function():
            await page.exposeFunction("automationAdd", lambda a, b: a + b)
            first = await page.evaluate("async () => await window.automationAdd(8, 13)")
            await navigate("exposed")
            return [first, await page.evaluate("async () => await window.automationAdd(4, 5)")]
        await checks.run("runtime.exposed_function_navigation", exposed_function, [21, 9])

        async def remove_init_script():
            # Pyppeteer has no public removal method; its real CDPSession remains
            # the supported escape hatch. Puppeteer exercises its public method.
            script = await page._client.send("Page.addScriptToEvaluateOnNewDocument", {"source": "window.removableInit = true"})
            await navigate("init-before")
            before = await page.evaluate("window.removableInit")
            await page._client.send("Page.removeScriptToEvaluateOnNewDocument", {"identifier": script["identifier"]})
            await navigate("init-after")
            return [before, await page.evaluate("typeof window.removableInit")]
        await checks.run("page.init_script_removal", remove_init_script, [True, "undefined"])

        async def viewport():
            await page.setViewport({"width": 900, "height": 620, "deviceScaleFactor": 1.5})
            return await page.evaluate("() => [innerWidth, innerHeight, devicePixelRatio]")
        await checks.run("emulation.viewport", viewport, [900, 620, 1.5])

        async def user_agent():
            await page.setUserAgent("AutomationClient/1.0")
            response = await page.goto(base + "/api/user-agent")
            return [await page.evaluate("navigator.userAgent"), (await response.json())["userAgent"]]
        await checks.run("network.user_agent_override", user_agent, ["AutomationClient/1.0", "AutomationClient/1.0"])

        async def interception_continue():
            tasks = []
            async def intercept(request):
                if request.url.endswith("/api/continue"):
                    await request.continue_({"method": "POST", "postData": "posted=42"})
                else:
                    await request.continue_()
            listener = lambda request: tasks.append(asyncio.create_task(intercept(request)))
            await page.setRequestInterception(True)
            page.on("request", listener)
            try:
                response = await page.goto(base + "/api/continue")
                value = await response.json()
                await asyncio.gather(*tasks)
                return {"method": value["method"], "body": value["body"]}
            finally:
                page.remove_listener("request", listener)
                await page.setRequestInterception(False)
                await asyncio.gather(*tasks, return_exceptions=True)
        await checks.run("network.interception_continue_post", interception_continue,
                         {"method": "POST", "body": "posted=42"})

        async def interception_abort():
            tasks, failed = [], []
            async def intercept(request):
                if request.url.endswith("/api/abort"):
                    await request.abort("failed")
                else:
                    await request.continue_()
            listener = lambda request: tasks.append(asyncio.create_task(intercept(request)))
            await page.setRequestInterception(True)
            page.on("request", listener)
            page.on("requestfailed", failed.append)
            rejected = False
            try:
                try:
                    await page.goto(base + "/api/abort")
                except Exception:
                    rejected = True
                await asyncio.gather(*tasks)
                return {"rejected": rejected, "requestFailed": any(request.url.endswith("/api/abort") for request in failed)}
            finally:
                page.remove_listener("request", listener)
                page.remove_listener("requestfailed", failed.append)
                await page.setRequestInterception(False)
                await asyncio.gather(*tasks, return_exceptions=True)
        await checks.run("network.interception_abort", interception_abort,
                         {"rejected": True, "requestFailed": True})

        async def parallel_pages():
            first, second = await asyncio.gather(new_page(), new_page())
            await asyncio.gather(first.goto(base + "/page/first"), second.goto(base + "/page/second"))
            await asyncio.gather(first.evaluate("window.pageIdentity = 'first'"),
                                 second.evaluate("window.pageIdentity = 'second'"))
            return await asyncio.gather(first.evaluate("() => [document.title, window.pageIdentity]"),
                                        second.evaluate("() => [document.title, window.pageIdentity]"))
        await checks.run("target.parallel_pages_isolation", parallel_pages,
                         [["Automation first", "first"], ["Automation second", "second"]])

        async def context_isolation():
            first, second = await asyncio.gather(browser.createIncognitoBrowserContext(),
                                                 browser.createIncognitoBrowserContext())
            contexts.extend((first, second))
            a, b = await asyncio.gather(first.newPage(), second.newPage())
            await asyncio.gather(a.goto(base + "/page/a"), b.goto(base + "/page/b"))
            await a.evaluate("() => { document.cookie = 'private=first; path=/'; localStorage.setItem('private', 'first'); }")
            return [await a.evaluate("() => [document.cookie, localStorage.getItem('private')]"),
                    await b.evaluate("() => [document.cookie, localStorage.getItem('private')]")]
        await checks.run("target.browser_context_storage_isolation", context_isolation,
                         [["private=first", "first"], ["", None]])
    finally:
        for context in contexts:
            with contextlib.suppress(Exception):
                await asyncio.wait_for(context.close(), 2)
        for page in pages:
            with contextlib.suppress(Exception):
                await asyncio.wait_for(page.close(), 2)
        if browser:
            with contextlib.suppress(Exception):
                await asyncio.wait_for(browser.disconnect(), 2)
    return {"client": "pyppeteer", "version": pyppeteer.__version__, "checks": checks.results}


async def run_client(args, name, base_url, version):
    directory = Path(args.output).resolve().parent
    path = directory / (name + ".json")
    path.unlink(missing_ok=True)
    trace_path = directory / (name + ".protocol.jsonl")
    recorder = WireRecorder(version["webSocketDebuggerUrl"], trace_path)
    async with websockets.serve(recorder.proxy, "127.0.0.1", 0, max_size=None,
                                ping_interval=None) as server:
        endpoint = "ws://127.0.0.1:" + str(server.sockets[0].getsockname()[1])
        common = ["--endpoint", endpoint, "--base-url", base_url, "--output", str(path)]
        if name == "pyppeteer":
            command = [sys.executable, str(Path(__file__).resolve()), "--worker", *common]
        else:
            command = [args.node, str(Path(__file__).with_suffix(".mjs")),
                       "--module", str(Path(args.puppeteer_module).resolve()), *common]
        process = await asyncio.create_subprocess_exec(*command, stdout=asyncio.subprocess.PIPE,
                                                       stderr=asyncio.subprocess.PIPE)
        try:
            stdout, stderr = await asyncio.wait_for(process.communicate(), 240)
        except asyncio.TimeoutError:
            process.kill()
            stdout, stderr = await process.communicate()
        if path.exists():
            result = json.loads(path.read_text(encoding="utf-8"))
        else:
            result = {"client": name, "checks": [], "workerFailure": "No result written"}
        result["exitCode"] = process.returncode
        if process.returncode:
            result["workerFailure"] = f"Client worker exited with code {process.returncode}"
        observed_checks = {row["name"] for row in result["checks"]}
        for name in CHECK_NAMES:
            if name not in observed_checks:
                result["checks"].append({"name": name, "status": "blocked",
                                          "error": result.get("blocked", result.get("workerFailure", "Incomplete client run"))})
        if stderr:
            (directory / (name + ".stderr.log")).write_bytes(stderr)
            result["stderr"] = str(directory / (name + ".stderr.log"))
        result["protocol"] = recorder.finish()
        result["summary"] = dict(Counter(row["status"] for row in result["checks"]))
        return result


def compare(reference, actual):
    rows = []
    for name, result in actual["clients"].items():
        expected = {r["name"]: r for r in reference.get("clients", {}).get(name, {}).get("checks", [])}
        for row in result["checks"]:
            oracle = expected.get(row["name"])
            classification = "unverified"
            if oracle:
                if oracle["status"] != "pass":
                    classification = "reference-failure"
                elif row["status"] == "pass" and row.get("observed") == oracle.get("observed"):
                    classification = "match"
                else:
                    classification = "divergence"
            rows.append({"client": name, "check": row["name"], "classification": classification})
    return {"summary": dict(Counter(row["classification"] for row in rows)), "checks": rows}


async def run(args):
    version = endpoint_version(args.endpoint)
    directory = Path(args.output).resolve().parent
    directory.mkdir(parents=True, exist_ok=True)
    fixture = ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=fixture.serve_forever, daemon=True).start()
    base_url = "http://127.0.0.1:" + str(fixture.server_port)
    result = {"schemaVersion": 1, "suiteVersion": 1,
              "target": {"endpoint": args.endpoint, "version": version},
              "fixture": {"origin": base_url, "controlledLoopback": True},
              "dependencies": {"python": platform.python_version(), "pyppeteer": PYPPETEER_VERSION,
                               "puppeteer-core": PUPPETEER_VERSION},
              "suiteSources": {path.name: hashlib.sha256(path.read_bytes()).hexdigest()
                               for path in (Path(__file__), Path(__file__).with_suffix(".mjs"))},
              "excludedCapabilities": [
                  {"methods": ["Page.captureScreenshot", "Page.printToPDF", "Page.startScreencast"],
                   "reason": "Mimic has no image renderer; output image/media compatibility is not claimed."},
                  {"methods": ["WebAudio.*", "Media.*"],
                   "reason": "Media devices and playback are outside this automation suite."}],
              "clients": {}}
    if args.binary:
        binary = Path(args.binary).resolve()
        result["target"]["binary"] = {"path": str(binary),
                                          "sha256": hashlib.sha256(binary.read_bytes()).hexdigest()}
    try:
        if args.oracle:
            result["captureMetadata"] = await capture_oracle_metadata(args.endpoint, base_url)
        for name in args.clients.split(","):
            print("Running " + name + " at " + args.endpoint, flush=True)
            result["clients"][name] = await run_client(args, name, base_url, version)
            write_json(Path(args.output), result)
        if args.reference:
            reference = json.loads(Path(args.reference).read_text(encoding="utf-8"))
            if "captureMetadata" not in reference:
                raise RuntimeError("Reference must be a --oracle headful Chrome capture")
            result["comparison"] = compare(reference, result)
        write_json(Path(args.output), result)
        print(json.dumps({name: value["summary"] for name, value in result["clients"].items()}))
        return result
    finally:
        fixture.shutdown()
        fixture.server_close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--clients", default="pyppeteer,puppeteer", choices=["pyppeteer", "puppeteer", "pyppeteer,puppeteer"])
    parser.add_argument("--node", default="node")
    parser.add_argument("--puppeteer-module", default=str(ROOT / ".build/cdp-clients/node_modules/puppeteer-core/lib/puppeteer/puppeteer-core.js"))
    parser.add_argument("--oracle", action="store_true", help="Require and capture exact controlled headful Chrome 152 provenance")
    parser.add_argument("--reference", help="Compare against a prior --oracle report")
    parser.add_argument("--binary", help="Record tested Mimic executable hash")
    parser.add_argument("--worker", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--base-url", help=argparse.SUPPRESS)
    parser.add_argument("--allow-failures", action="store_true", help="Exploratory captures still report failures but exit zero")
    args = parser.parse_args()
    logging.basicConfig(level=logging.ERROR)
    if args.worker:
        result = asyncio.run(pyppeteer_worker(args))
        write_json(Path(args.output), result)
        return 0
    result = asyncio.run(run(args))
    failed = any(c.get("workerFailure") or c.get("blocked") or c["summary"].get("fail") or c["summary"].get("blocked")
                 for c in result["clients"].values())
    failed = failed or any(count for status, count in result.get("comparison", {}).get("summary", {}).items()
                           if status != "match")
    return int(bool(failed) and not args.allow_failures)


if __name__ == "__main__":
    raise SystemExit(main())
