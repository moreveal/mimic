"""Read Mimic's live trace even while a browser operation is in progress."""
import argparse
import asyncio
import json
from urllib.parse import urlsplit


def compact_event(event):
    data = event.get("data", {})
    compact = {}
    for key in ("status", "method", "resourceType", "type", "error", "property", "api", "task", "source", "access", "realm"):
        if key in data:
            compact[key] = data[key]
    url = data.get("url", data.get("resource"))
    if url:
        parsed = urlsplit(str(url))
        compact["url"] = f"{parsed.scheme}://{parsed.netloc}{parsed.path}"
        if "/fo/" in parsed.path and isinstance(data.get("headers"), dict):
            wanted = {"content-type", "cf-chl", "cf-chl-ra", "origin", "referer", "user-agent", "sec-ch-ua", "sec-ch-ua-full-version-list"}
            compact["headers"] = {key: value for key, value in data["headers"].items() if key.lower() in wanted}
        if data.get("postData") is not None:
            compact["postDataLength"] = len(str(data.get("postData", "")))
    if event.get("kind") == "api" and event.get("name") == "Performance.getEntries":
        compact["count"] = data.get("count")
        compact["entries"] = data.get("entries")
    return {"sequence": event.get("sequence"), "kind": event.get("kind"), "name": event.get("name"), "data": compact}

from pyppeteer import connect


async def main(endpoint: str, summary: bool, compact: bool, url_contains: str) -> None:
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    pages = await browser.pages()
    page = next((candidate for candidate in pages if url_contains in candidate.url), pages[0])
    trace = await page._client.send("Mimic.getTrace")
    if compact:
        events = trace.get("events", [])
        first_fo_sequence = next((
            event.get("sequence", 1 << 60) for event in events
            if event.get("kind") == "network"
            and event.get("name") == "request"
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
            if api:
                api_before_fo.append(api)
        selected = [
            compact_event(event) for event in events
            if event.get("kind") in {"network", "resource", "lifecycle", "error", "exception", "semantic-missing", "surface-missing", "unsupported", "csp"}
            or (event.get("kind") == "api" and event.get("name") == "Performance.getEntries")
        ]
        trace = {"eventCount": len(events), "apiBeforeFirstFO": api_before_fo, "selected": selected, "tail": [compact_event(event) for event in events[-20:]]}
    elif summary:
        events = trace.get("events", [])
        timeline = []
        for event in events:
            data = event.get("data", {})
            api_name = str(data.get("api", data.get("property", event.get("name", ""))))
            url = str(data.get("url", data.get("resource", "")))
            if (
                event.get("kind") in {"network", "resource", "js", "lifecycle", "error", "exception"}
                or "RTCPeerConnection" in api_name
                or "/fo/" in url
            ):
                timeline.append(event)
        trace = {
            "network": [event for event in events if event.get("kind") == "network" and event.get("name") in {"request", "response", "failed"}],
            "dynamicResources": [event for event in events if event.get("kind") == "dom" and event.get("name") == "dynamicResourceInsertion"],
            "performance": [event for event in events if event.get("kind") == "api" and event.get("name") == "Performance.getEntries"],
            "issues": [event for event in events if event.get("kind") in {"error", "exception", "semantic-missing", "surface-missing", "unsupported"}],
            "timeline": timeline,
            "tail": events[-20:],
        }
    print(json.dumps(trace, ensure_ascii=False, indent=2, default=str))
    await browser.disconnect()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", default="http://127.0.0.1:19222")
    parser.add_argument("--summary", action="store_true")
    parser.add_argument("--compact", action="store_true")
    parser.add_argument("--url-contains", default="")
    args = parser.parse_args()
    asyncio.run(main(args.endpoint, args.summary, args.compact, args.url_contains))
