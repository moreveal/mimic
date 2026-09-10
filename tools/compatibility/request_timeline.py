"""Offline CDP request provenance inventory. No payload decoding or replay.

Output is PRIVATE: URLs, headers and initiator stacks can contain credentials.
Request IDs are scoped to sessions; repeated IDs retain attempt ambiguity.
Temporal adjacency is deliberately not represented as a data-dependency edge.
"""
import argparse
import base64
import collections
import hashlib
import json
from pathlib import Path


def unwrap(event, session=""):
    if event.get("method") == "Target.receivedMessageFromTarget":
        params = event["params"]
        return unwrap(json.loads(params["message"]), params.get("sessionId", session))
    return event, event.get("sessionId", session)


def digest(data):
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}


def inventory(directory):
    directory = Path(directory)
    jsonl = directory / "events.jsonl"
    events = ([json.loads(line) for line in jsonl.read_text(encoding="utf-8").splitlines() if line]
              if jsonl.exists() else json.loads((directory / "events.json").read_text(encoding="utf-8")))
    groups = collections.defaultdict(lambda: {"requests": [], "responses": [], "requestExtra": [], "responseExtra": []})
    rows = []
    names = {"Network.requestWillBeSent": "requests", "Network.responseReceived": "responses",
             "Network.requestWillBeSentExtraInfo": "requestExtra", "Network.responseReceivedExtraInfo": "responseExtra"}
    for index, raw in enumerate(events):
        event, session = unwrap(raw)
        name = names.get(event.get("method"))
        if not name:
            continue
        params = event["params"]
        group = groups[(session, params["requestId"])]
        group[name].append(params)
        if name == "requests":
            request = params["request"]
            rows.append({"eventIndex": index, "session": session, "requestId": params["requestId"],
                         "attempt": len(group[name]), "url": request["url"], "method": request["method"],
                         "timestamp": params.get("timestamp"), "wallTime": params.get("wallTime"),
                         "frameId": params.get("frameId"), "loaderId": params.get("loaderId"),
                         "initiator": params.get("initiator"), "requestHeaders": request.get("headers", {}),
                         "redirectResponse": params.get("redirectResponse"), "_request": request})
    manifest_path = directory / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8")) if manifest_path.exists() else {}
    bodies = {(v["session"], v["requestId"]): v["file"] for v in manifest.get("bodies", [])}
    for row in rows:
        key = (row["session"], row["requestId"])
        group = groups[key]
        request = row.pop("_request")
        unique = len(group["requests"]) == 1
        row["association"] = "unique" if unique else "ambiguous-reused-request-id"
        row["requestExtraCandidates"] = group["requestExtra"]
        row["responseExtraCandidates"] = group["responseExtra"]
        row["responseCandidates"] = group["responses"]
        body = request.get("postData")
        body_source = "requestWillBeSent.postData"
        post_path = directory / ("post-" + hashlib.sha256((key[0] + ":" + key[1]).encode()).hexdigest()[:20] + ".json")
        if body is None and unique and post_path.exists():
            body = json.loads(post_path.read_text(encoding="utf-8")).get("result", {}).get("postData")
            body_source = "getRequestPostData"
        row["requestBody"] = ({**digest(body.encode("utf-8")), "source": body_source,
                               "representation": "CDP text encoded as UTF-8; not independent wire bytes"}
                              if body is not None else {"missing": True})
        body_file = bodies.get(key)
        if unique and body_file:
            result = json.loads((directory / body_file).read_text(encoding="utf-8")).get("result", {})
            if "body" in result:
                decoded = base64.b64decode(result["body"]) if result.get("base64Encoded") else result["body"].encode("utf-8")
                row["responseBody"] = {**digest(decoded), "file": body_file, "representation": "CDP decoded response"}
        if unique and len(group["responses"]) == 1:
            response = group["responses"][0]
            row["response"] = response
            if row["timestamp"] is not None and response.get("timestamp") is not None:
                row["responseDeltaMs"] = (response["timestamp"] - row["timestamp"]) * 1000
        row["browserObservationDependencies"] = {"known": False, "reason": "These captures contain no value-flow provenance"}
        row["continuationDependency"] = {"known": False, "reason": "Initiator stack is evidence of callers, not response-to-code/value dependency"}
    return {"capture": str(directory), "requests": rows,
            "limitations": ["No cross-run equivalence inferred from request order or body hashes.",
                            "Session-local CDP timestamps are retained; cross-target clock equivalence is not assumed.",
                            "Missing fields remain unknown. Reused request IDs are not guessed into attempts.",
                            "Response bytes and random per-attempt tokens naturally differ; hashes alone do not identify a semantic divergence."]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("capture", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    args.output.write_text(json.dumps(inventory(args.capture), indent=2, ensure_ascii=False), encoding="utf-8")


if __name__ == "__main__":
    main()
