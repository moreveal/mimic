"""Summarize `go test -json` logs, including parallel subtest wall time.

Example: go test -json ./... -timeout=25m > .build/tests.jsonl
         python tools/testing/timings.py .build/tests.jsonl --top 20
"""

import argparse
import datetime
import json
import sys


def summarize(lines, top, leaves):
    starts, completed, packages, errors = {}, {}, [], []
    for line in lines:
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue  # Preserve usefulness with mixed compiler/test output.
        action, name = event.get("Action"), event.get("Test", "")
        package = event.get("Package", "")
        key = (package, name)
        if action == "run":
            starts[key] = datetime.datetime.fromisoformat(event["Time"])
        elif action in ("pass", "fail", "skip"):
            if action == "fail":
                errors.append(f"{package}/{name}".rstrip("/"))
            if not name:
                packages.append((package, action, event.get("Elapsed", 0)))
            elif key in starts:
                elapsed = (datetime.datetime.fromisoformat(event["Time"]) - starts.pop(key)).total_seconds()
                # Go's Elapsed on a parallel parent excludes its children.
                # Measure run-to-completion instead; never sum overlapping rows.
                completed[key] = (elapsed, action)
    parents = {(pkg, name.rsplit("/", 1)[0]) for pkg, name in completed if "/" in name}
    rows = [(seconds, pkg, name, status) for (pkg, name), (seconds, status) in completed.items()
            if ((pkg, name) not in parents if leaves else "/" not in name)]
    print("Wall seconds  Status  Test (parallel rows overlap; do not sum)")
    for seconds, pkg, name, status in sorted(rows, reverse=True)[:top]:
        print(f"{seconds:12.3f}  {status:6}  {pkg}/{name}")
    for pkg, status, elapsed in packages:
        print(f"Package: {pkg}: {status}, {elapsed:.3f}s")
    if starts:
        print(f"Incomplete: {len(starts)} tests started without a terminal event")
    if errors:
        print(f"Failures: {len(errors)} test/package events")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("log", help="Go JSON log, or - for stdin")
    parser.add_argument("--top", type=int, default=20)
    parser.add_argument("--leaves", action="store_true", help="show individual cases instead of top-level groups")
    args = parser.parse_args()
    if args.log == "-":
        summarize(sys.stdin, args.top, args.leaves)
    else:
        with open(args.log, encoding="utf-8-sig") as stream:
            summarize(stream, args.top, args.leaves)


if __name__ == "__main__":
    main()
