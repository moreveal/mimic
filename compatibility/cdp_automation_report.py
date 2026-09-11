"""Retain compact, measured Chrome automation references without rewriting values."""
import argparse
import json
from pathlib import Path


def compact_report(source):
    metadata = source.get("captureMetadata", {})
    if metadata.get("chromeVersion") != "152.0.7977.82" or metadata.get("browserMode") != "headful":
        raise ValueError("A provenanced headful Chrome 152 capture is required")
    result = {key: source[key] for key in (
        "schemaVersion", "suiteVersion", "suiteSources", "target", "version", "fixture", "origin",
        "dependencies", "captureMetadata", "excludedCapabilities") if key in source}
    if "clients" in source:
        result["clients"] = {}
        for name, client in source["clients"].items():
            if client.get("workerFailure") or client.get("exitCode") or any(row["status"] != "pass" for row in client["checks"]):
                raise ValueError("Cannot freeze a failed or incomplete automation-client reference")
            result["clients"][name] = {
                "client": client["client"], "version": client["version"], "summary": client["summary"],
                "checks": [{key: value for key, value in row.items() if key != "durationMs"} for row in client["checks"]],
                "protocol": {"methods": client["protocol"]["methods"], "events": client["protocol"]["events"]},
            }
    else:
        if any(not row.get("loaded") for row in source["scenarios"]):
            raise ValueError("Cannot freeze an incomplete interception reference")
        result["scenarios"] = []
        for row in source["scenarios"]:
            entry = {"name": row["name"], "loaded": row["loaded"], "stages": row["stages"]}
            entry["paused"] = []
            for pause in row["paused"]:
                retained = {key: pause[key] for key in (
                    "requestId", "networkId", "interceptionId", "frameId", "resourceType", "isNavigationRequest",
                    "responseStatusCode", "responseStatusText", "responseHeaders") if key in pause}
                retained["request"] = {key: pause["request"][key] for key in ("url", "method", "postData") if key in pause["request"]}
                entry["paused"].append(retained)
            result["scenarios"].append(entry)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = compact_report(json.loads(args.source.read_text(encoding="utf-8")))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(output, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(str(args.output))
