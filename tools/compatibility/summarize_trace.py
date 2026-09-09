"""Summarize Mimic trace JSON or native CDP events JSONL without inferring success.

Output may include application console text and URLs; keep it private with the
input capture. Caught exceptions logged to console are diagnostics too.
"""
import argparse
import collections
import json
from pathlib import Path


def read_events(path):
    text = Path(path).read_text(encoding="utf-8")
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        return [json.loads(line) for line in text.splitlines() if line.strip()]
    if isinstance(value, list):
        return value
    if "result" in value and "events" in value["result"]:
        return value["result"]["events"]
    if "events" in value:
        return value["events"]
    return [value]


def summarize(events):
    counts = collections.Counter()
    diagnostics, documents, redirects = [], [], []
    unsupported = collections.Counter()
    requests = {}
    for index, event in enumerate(events):
        if event.get("kind") == "cdp" and event.get("name") == "event":
            # Mimic includes CDP projections alongside its authoritative trace.
            # Use projections for native network fields, but not duplicate errors.
            projection = event.get("data", {})
            if not projection.get("method", "").startswith("Network."):
                counts["cdp.event"] += 1
                continue
            event = {**event, **projection}
        method = event.get("method")
        kind, name = event.get("kind"), event.get("name")
        counts[method or f"{kind}.{name}"] += 1
        data = event.get("params", {}) if method else event.get("data", {})
        if kind == "semantic-missing":
            unsupported[name] += 1
        position = {"event": index + 1}
        if "sequence" in event:
            position["sequence"] = event["sequence"]
        if method == "Runtime.consoleAPICalled":
            if data.get("type") in ("error", "warning", "assert"):
                args = [arg.get("description", arg.get("value", arg.get("type")))
                        for arg in data.get("args", [])]
                diagnostics.append({**position, "source": method, "level": data["type"], "args": args})
        elif method == "Runtime.exceptionThrown":
            diagnostics.append({**position, "source": method, "details": data.get("exceptionDetails", {})})
        elif kind == "error" or kind == "console" and name in ("error", "warning", "warn", "assert"):
            diagnostics.append({**position, "source": f"{kind}.{name}", "details": data})
        if method == "Network.requestWillBeSent":
            key = (event.get("sessionId"), data.get("requestId"))
            requests[key] = data.get("type")
            previous = data.get("redirectResponse")
            if previous:
                redirects.append({**position, "url": previous.get("url"), "status": previous.get("status"),
                                  "target": data.get("request", {}).get("url")})
        elif method == "Network.responseReceived":
            key = (event.get("sessionId"), data.get("requestId"))
            if data.get("type", requests.get(key)) == "Document":
                response = data.get("response", {})
                headers = {k.lower(): v for k, v in response.get("headers", {}).items()}
                documents.append({**position, "url": response.get("url"), "status": response.get("status"),
                                  "cfMitigated": headers.get("cf-mitigated")})
        elif kind == "network" and name == "redirect":
            redirects.append({**position, "details": data})
        elif method == "Network.loadingFailed" or kind == "network" and name == "error":
            diagnostics.append({**position, "source": method or f"{kind}.{name}", "details": data})
    return {"eventCount": sum(counts.values()), "counts": dict(sorted(counts.items())),
            "diagnostics": diagnostics, "unsupported": dict(sorted(unsupported.items())),
            "documentResponses": documents, "redirects": redirects,
            "limitations": "No passage verdict. Caught exceptions not logged by the page require an inspector capture. "
                           "Document responses include frames; identify the top-level target before interpreting them."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("capture", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = json.dumps(summarize(read_events(args.capture)), ensure_ascii=False, indent=2)
    if args.output:
        args.output.write_text(result + "\n", encoding="utf-8")
    else:
        print(result)


if __name__ == "__main__":
    main()
