"""Compare the engine-only corpus in pinned Chrome, QuickJS, and V8."""

import argparse
import asyncio
import json
import subprocess
import urllib.request

from pyppeteer import connect


def product(endpoint: str) -> str:
    with urllib.request.urlopen(endpoint.rstrip("/") + "/json/version") as response:
        return json.load(response).get("Browser", "")


def local(engine: str, probes: str) -> dict:
    completed = subprocess.run(
        ["go", "run", "./cmd/engine-spike", "--engine", engine, "--probes", probes],
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(completed.stdout)


async def chrome(endpoint: str, probes: list[dict]) -> dict:
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    results = {}
    try:
        for probe in probes:
            try:
                value = await page.evaluate(probe["expression"], force_expr=True)
                results[probe["name"]] = {"value": value, "error": ""}
            except Exception as exc:
                results[probe["name"]] = {"value": None, "error": str(exc)}
    finally:
        await page.close()
        await browser.disconnect()
    return results


async def run(endpoint: str, probes_path: str) -> None:
    if product(endpoint) != "Chrome/152.0.7977.82":
        raise RuntimeError(f"differential oracle drift: {product(endpoint)!r}")
    with open(probes_path, encoding="utf-8") as source:
        probes = json.load(source)
    chrome_results, quickjs_results, v8_results = await asyncio.gather(
        chrome(endpoint, probes),
        asyncio.to_thread(local, "quickjs", probes_path),
        asyncio.to_thread(local, "v8", probes_path),
    )
    rows = []
    for probe in probes:
        name = probe["name"]
        expected = chrome_results[name]
        rows.append({
            "capability": name,
            "chrome": "pass" if not expected["error"] else "error",
            "quickjs": "match" if quickjs_results[name] == expected else "mismatch",
            "v8": "match" if v8_results[name] == expected else "mismatch",
            "chromeObservation": expected,
            "quickjsObservation": quickjs_results[name],
            "v8Observation": v8_results[name],
        })
    print(json.dumps({
        "target": product(endpoint),
        "quickjsMatched": sum(row["quickjs"] == "match" for row in rows),
        "v8Matched": sum(row["v8"] == "match" for row in rows),
        "total": len(rows),
        "results": rows,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--probes", default="compatibility/probes-engine.json")
    args = parser.parse_args()
    asyncio.run(run(args.chrome, args.probes))

