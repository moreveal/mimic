"""Real Pyppeteer compatibility smoke test for Mimic's CDP contract."""
import argparse
import asyncio
import json
from collections import Counter

from pyppeteer import connect


def summarize_trace(trace):
    events = trace.get("events", [])
    counts = Counter(f"{e.get('kind')}:{e.get('name')}" for e in events)
    issues = [e for e in events if e.get("kind") in {"unsupported", "exception", "error", "csp"}]
    diagnostics = [
        e for e in events
        if (e.get("kind"), e.get("name")) in {("js", "frameEvalStart"), ("js", "frameEvalResult")}
    ]
    first_fo_sequence = next((
        event.get("sequence", 1 << 60) for event in events
        if event.get("kind") == "network" and event.get("name") == "request"
        and "/fo/" in str(event.get("data", {}).get("url", ""))
    ), 1 << 60)
    api_before_fo = []
    for event in events:
        if event.get("sequence", 0) >= first_fo_sequence or event.get("kind") != "api":
            continue
        data = event.get("data", {})
        api = data.get("api", data.get("property"))
        if not api and event.get("name") != "propertyAccess":
            api = event.get("name")
        if api and api not in api_before_fo:
            api_before_fo.append(api)
    performance = []
    for event in events:
        if event.get("kind") != "api" or event.get("name") != "Performance.getEntries":
            continue
        data = event.get("data", {})
        performance.append({
            "types": data.get("types"),
            "count": data.get("count"),
            "entries": data.get("entries"),
            "values": data.get("values", [])[:4],
        })
        if len(performance) == 8:
            break
    ready_states = [
        e.get("data", {}).get("value") for e in events
        if e.get("kind") == "api" and e.get("name") == "Document.readyStateValue"
        and e.get("sequence", 0) < first_fo_sequence
    ]
    transport = [
        {
            "url": event.get("data", {}).get("url"),
            "sessionCold": event.get("data", {}).get("sessionCold"),
            "connectionState": event.get("data", {}).get("connectionState"),
            "timing": event.get("data", {}).get("timing"),
        }
        for event in events
        if event.get("kind") == "network" and event.get("name") == "transport"
    ]
    load_transitions = [
        {"sequence": event.get("sequence"), "name": event.get("name"), "data": event.get("data")}
        for event in events
        if event.get("kind") == "lifecycle" and event.get("name") in {
            "loadBlockerAdded", "loadBlockerRemoved", "loadBlockerIgnored", "readyStateComplete", "load"
        }
    ]
    return {
        "eventCount": len(events),
        "counts": dict(sorted(counts.items())),
        "apiBeforeFirstFO": api_before_fo,
        "performance": performance,
        "readyStatesBeforeFirstFO": ready_states,
        "transport": transport,
        "loadTransitions": load_transitions,
        "diagnostics": diagnostics[-20:],
        "issues": issues[-30:],
    }


def compact_resources(items, include_headers=False):
    compact = []
    for item in items:
        url = item.get("url", "")
        compact.append({
            "url": url.split("?", 1)[0],
            **({"method": item["method"]} if "method" in item else {}),
            **({"type": item["type"]} if "type" in item else {}),
            **({"status": item["status"]} if "status" in item else {}),
            **({"postDataLength": item["postDataLength"]} if item.get("postDataLength") else {}),
            **({"body": item["body"][:200]} if item.get("body") else {}),
            **({"bodyError": item["bodyError"]} if item.get("bodyError") else {}),
            **({"headers": item["headers"]} if include_headers and item.get("headers") else {}),
        })
    return compact


def compact_outcome(response, evaluated, requests, responses, trace, include_headers=False):
    events = trace.get("events", [])
    trace_summary = summarize_trace(trace)
    first_fo_sequence = next((
        event.get("sequence", 1 << 60) for event in events
        if event.get("kind") == "network" and event.get("name") == "request"
        and "/fo/" in str(event.get("data", {}).get("url", ""))
    ), 1 << 60)
    performance_before_fo = [
        event.get("data", {}).get("values", [])
        for event in events
        if event.get("sequence", 0) < first_fo_sequence
        and event.get("kind") == "api" and event.get("name") == "Performance.getEntries"
    ]
    worker_before_fo = [
        {
            "sequence": event.get("sequence"),
            "kind": event.get("kind"),
            "name": event.get("name"),
            "data": event.get("data"),
        }
        for event in events
        if event.get("sequence", 0) < first_fo_sequence
        and event.get("data", {}).get("worker") is not None
    ]
    api_after_first_fo = []
    seen_api_after_first_fo = set()
    for event in events:
        if event.get("sequence", 0) <= first_fo_sequence or event.get("kind") != "api":
            continue
        data = event.get("data", {})
        api = data.get("api", data.get("property"))
        if not api and event.get("name") != "propertyAccess":
            api = event.get("name")
        identity = (api, data.get("realm"), data.get("worker"), data.get("supported"))
        if not api or identity in seen_api_after_first_fo:
            continue
        seen_api_after_first_fo.add(identity)
        item = {"sequence": event.get("sequence"), "api": api}
        for key in ("realm", "worker", "supported"):
            if key in data:
                item[key] = data[key]
        api_after_first_fo.append(item)
        if len(api_after_first_fo) == 160:
            break
    after_first_fo = []
    for event in events:
        if event.get("sequence", 0) <= first_fo_sequence:
            continue
        kind, name, data = event.get("kind"), event.get("name"), event.get("data", {})
        if kind not in {"network", "resource", "lifecycle", "js", "scheduler", "error", "exception", "semantic-missing", "surface-missing", "unsupported"}:
            continue
        item = {"sequence": event.get("sequence"), "kind": kind, "name": name}
        for key in (
            "status", "method", "source", "direction", "worker", "frameId", "parentFrameId",
            "sourceFrameId", "targetFrameId", "valueType", "valueShape", "encodedSize", "portCount", "realm", "error",
        ):
            if key in data:
                item[key] = data[key]
        if data.get("url"):
            item["url"] = str(data["url"]).split("?", 1)[0]
        after_first_fo.append(item)
        if len(after_first_fo) == 160:
            break
    cookie_lifecycle = []
    for event in events:
        if event.get("kind") != "network" or event.get("name") not in {"request", "response"}:
            continue
        data = event.get("data", {})
        headers = data.get("headers", {})
        header = next((value for key, value in headers.items() if key.lower() == ("set-cookie" if event.get("name") == "response" else "cookie")), "")
        if not header:
            continue
        names = []
        for value in str(header).replace("\r", "").split("\n"):
            first = value.strip().split(";", 1)[0]
            if "=" in first:
                names.append(first.split("=", 1)[0])
        cookie_lifecycle.append({"kind": event.get("name"), "url": data.get("url", "").split("?", 1)[0], "names": names})
    relevant_requests = [
        item for item in requests
        if any(part in item.get("url", "") for part in ("/challenge-platform/", "/turnstile/", "/fo/", "/eb/", "favicon.ico"))
    ]
    first_fo = next((item for item in responses if "/fo/" in item.get("url", "")), None)
    return {
        "response": response.status if response else None,
        "url": evaluated.get("url"),
        "title": evaluated.get("title"),
        "cookie": evaluated.get("cookie"),
        "bodyHTMLLength": evaluated.get("bodyHTMLLength"),
        "turnstileType": evaluated.get("turnstileType"),
        "requests": compact_resources(relevant_requests, include_headers),
        "firstFO": compact_resources([first_fo])[0] if first_fo else None,
        "readyStatesBeforeFirstFO": trace_summary["readyStatesBeforeFirstFO"],
        "apiBeforeFirstFO": trace_summary["apiBeforeFirstFO"],
        "performanceBeforeFirstFO": performance_before_fo[-2:],
        "workerBeforeFirstFO": worker_before_fo[-120:],
        "apiAfterFirstFO": api_after_first_fo,
        "afterFirstFO": after_first_fo,
        "cookieLifecycle": cookie_lifecycle,
        "issues": [
            {"kind": event.get("kind"), "name": event.get("name"), "data": event.get("data")}
            for event in events
            if event.get("kind") in {"unsupported", "exception", "error", "csp"}
        ][-12:],
    }


def compact_post_fo_outcome(response, evaluated, requests, responses, trace):
    outcome = compact_outcome(response, evaluated, requests, responses, trace)
    events = trace.get("events", [])
    first_fo_sequence = next((
        event.get("sequence", 1 << 60) for event in events
        if event.get("kind") == "network" and event.get("name") == "request"
        and "/fo/" in str(event.get("data", {}).get("url", ""))
    ), 1 << 60)
    milestones = []
    milestone_names = {
        "frameAttached", "frameNavigated", "DOMContentLoaded", "load", "iframeOwnerLoad",
        "frameMessagePosted", "workerCreated", "workerTerminate", "workerBootstrapStart",
        "workerBootstrapEnd", "workerStart", "workerMessageQueued", "workerMessageDispatch",
    }
    for event in events:
        if event.get("sequence", 0) <= first_fo_sequence:
            continue
        kind, name, data = event.get("kind"), event.get("name"), event.get("data", {})
        if not (
            (kind == "network" and name in {"request", "response"})
            or name in milestone_names
            or kind in {"unsupported", "exception", "error", "semantic-missing", "surface-missing"}
        ):
            continue
        item = {"sequence": event.get("sequence"), "kind": kind, "name": name}
        for key in (
            "status", "method", "direction", "worker", "frameId", "parentFrameId",
            "sourceFrameId", "targetFrameId", "valueType", "valueShape", "encodedSize", "portCount", "realm", "error",
        ):
            if key in data:
                item[key] = data[key]
        if data.get("url"):
            item["url"] = str(data["url"]).split("?", 1)[0]
        milestones.append(item)
    apis = outcome["apiAfterFirstFO"]
    return {
        "response": outcome["response"],
        "url": outcome["url"],
        "cookie": outcome["cookie"],
        "frames": evaluated.get("frames", []),
        "firstFO": outcome["firstFO"],
        "apiAfterFirstFOCount": len(apis),
        "apiAfterFirstFOHead": apis[:20],
        "apiAfterFirstFOTail": apis[-60:],
        "postFOMilestones": milestones[-160:],
        "cookieLifecycle": outcome["cookieLifecycle"],
        "issues": outcome["issues"],
    }


async def run(endpoint: str, url: str, timeout: int, settle_ms: int, summary: bool, clear_state: bool, headers: bool, outcome_only: bool, post_fo_only: bool) -> None:
    print("stage=connect", flush=True)
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    print("stage=pages", flush=True)
    page = await browser.newPage()
    has_mimic_trace = True
    try:
        await page._client.send("Mimic.clearTrace")
    except Exception:
        has_mimic_trace = False
    if clear_state:
        await page._client.send("Network.clearBrowserCookies")
        await page._client.send("Network.clearBrowserCache")
    print("stage=interception", flush=True)
    requests = []
    responses = []
    response_tasks = []

    async def continue_request(request):
        requests.append({
            "url": request.url,
            "method": request.method,
            "type": request.resourceType,
            "headers": request.headers,
            "postDataLength": len(request.postData or ""),
        })
        await request.continue_()

    page.on("request", lambda request: asyncio.ensure_future(continue_request(request)))
    async def record_response(response):
        item = {"url": response.url, "status": response.status}
        responses.append(item)
        if response.status >= 400 and "/fo/" in response.url:
            try:
                item["body"] = (await response.text())[:1000]
            except Exception as exc:
                item["bodyError"] = str(exc)

    page.on("response", lambda response: response_tasks.append(asyncio.ensure_future(record_response(response))))
    await page.setRequestInterception(True)
    print("stage=navigate", flush=True)
    try:
        response = await page.goto(url, {"waitUntil": "load", "timeout": timeout})
    except Exception:
        trace = await page._client.send("Mimic.getTrace") if has_mimic_trace else {"events": []}
        print(json.dumps({"requests": compact_resources(requests, headers) if summary else requests, "responses": compact_resources(responses, headers) if summary else responses, "trace": summarize_trace(trace) if summary else trace}, ensure_ascii=False, indent=2, default=str), flush=True)
        raise
    if settle_ms:
        print(f"stage=settle:{settle_ms}ms", flush=True)
        await asyncio.sleep(settle_ms / 1000)
    if response_tasks:
        await asyncio.gather(*response_tasks, return_exceptions=True)
    print("stage=evaluate", flush=True)
    evaluated = await page.evaluate(
        "() => ({title: document.title, url: location.href, ua: navigator.userAgent, "
        "bodyText: document.body ? document.body.textContent.slice(0, 300) : '', "
        "bodyHTMLLength: document.body ? document.body.innerHTML.length : 0, "
        "turnstileType: typeof globalThis.turnstile, callbackType: typeof globalThis.khCN8, "
        "cookie: document.cookie})"
    )
    frame_states = []
    for frame in page.frames:
        try:
            state = await frame.evaluate(
                "() => ({url: location.href, title: document.title, readyState: document.readyState, "
                "bodyText: document.body ? document.body.textContent.trim().slice(0, 300) : '', "
                "inputs: document.querySelectorAll('input').length, "
                "buttons: document.querySelectorAll('button').length, "
                "iframes: document.querySelectorAll('iframe').length})"
            )
            frame_states.append(state)
        except Exception as exc:
            frame_states.append({"url": frame.url, "error": str(exc)})
    evaluated["frames"] = frame_states
    trace = await page._client.send("Mimic.getTrace") if has_mimic_trace else {"events": []}
    if post_fo_only:
        result = compact_post_fo_outcome(response, evaluated, requests, responses, trace)
    elif outcome_only:
        result = compact_outcome(response, evaluated, requests, responses, trace, headers)
    else:
        result = {"response": response.status if response else None, "evaluated": evaluated, "requests": compact_resources(requests, headers) if summary else requests, "responses": compact_resources(responses, headers) if summary else responses, "trace": summarize_trace(trace) if summary else trace}
    print(json.dumps(result, ensure_ascii=False, indent=2, default=str))
    await page.close()
    await browser.disconnect()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", default="http://127.0.0.1:19222")
    parser.add_argument("--url", default="https://example.com/")
    parser.add_argument("--timeout", type=int, default=20_000)
    parser.add_argument("--settle-ms", type=int, default=0)
    parser.add_argument("--summary", action="store_true")
    parser.add_argument("--clear-state", action="store_true")
    parser.add_argument("--headers", action="store_true")
    parser.add_argument("--outcome-only", action="store_true")
    parser.add_argument("--post-fo-only", action="store_true")
    args = parser.parse_args()
    asyncio.run(run(args.endpoint, args.url, args.timeout, args.settle_ms, args.summary, args.clear_state, args.headers, args.outcome_only, args.post_fo_only))
