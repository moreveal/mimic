"""Run the same JavaScript observation probes against pinned Chrome and Mimic."""

import argparse
import asyncio
import json
import urllib.request

from pyppeteer import connect


def product(endpoint: str) -> str:
    with urllib.request.urlopen(endpoint.rstrip("/") + "/json/version") as response:
        return json.load(response).get("Browser", "")


async def observe(endpoint: str, probes: list[dict], timeout: float, url: str) -> dict:
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    # Never reuse a live benchmark/application target: navigation and timers on
    # that page would make otherwise deterministic differential probes race it.
    page = await browser.newPage()
    if url:
        await page.goto(url, {"waitUntil": "load", "timeout": int(timeout * 1000)})
    results = {}
    for probe in probes:
        try:
            results[probe["name"]] = {
                "value": await asyncio.wait_for(
                    page.evaluate(probe["expression"], force_expr=True), timeout=timeout
                ),
                "error": "",
            }
        except Exception as exc:
            result = {"value": None, "error": str(exc)}
            if product(endpoint).startswith("Mimic/"):
                try:
                    events = (await page._client.send("Mimic.getTrace")).get("events", [])
                    result["workerTrace"] = [
                        event for event in events
                        if event.get("data", {}).get("worker") is not None
                        or event.get("name", "").startswith("worker")
                    ][-40:]
                except Exception:
                    pass
            results[probe["name"]] = result
    try:
        await page.close()
        await browser.disconnect()
    except (Exception, asyncio.CancelledError):
        # Pyppeteer may try to reject an already-cancelled CDP callback after a
        # deliberately timed-out probe. The collected observations remain valid.
        pass
    return results


async def run(chrome: str, mimic: str, probes_path: str, name: str, timeout: float, url: str, summary_only: bool) -> None:
    with open(probes_path, encoding="utf-8") as source:
        probes = json.load(source)
    if name:
        probes = [probe for probe in probes if probe["name"] == name]
        if not probes:
            raise RuntimeError(f"unknown probe: {name}")
    chrome_product = product(chrome)
    mimic_product = product(mimic)
    if chrome_product != "Chrome/152.0.7977.82":
        raise RuntimeError(f"differential oracle drift: {chrome_product!r}")
    chrome_results, mimic_results = await asyncio.gather(
        observe(chrome, probes, timeout, url), observe(mimic, probes, timeout, url)
    )
    rows = []
    for probe in probes:
        name = probe["name"]
        expected, actual = chrome_results[name], mimic_results[name]
        if name == "window surface census" and expected.get("value") and actual.get("value"):
            chrome_names = set(expected["value"].get("names", []))
            mimic_names = set(actual["value"].get("names", []))
            expected = {**expected, "value": {"count": len(chrome_names), "missingInMimic": sorted(chrome_names - mimic_names)}}
            actual = {**actual, "value": {"count": len(mimic_names), "extraInMimic": sorted(mimic_names - chrome_names)}}
        if name == "dedicated worker realm surface" and expected.get("value") and actual.get("value"):
            chrome_value, mimic_value = expected["value"], actual["value"]
            chrome_chain, mimic_chain = chrome_value.get("chain", []), mimic_value.get("chain", [])
            chain_diffs = []
            for index in range(max(len(chrome_chain), len(mimic_chain))):
                chrome_item = chrome_chain[index] if index < len(chrome_chain) else {}
                mimic_item = mimic_chain[index] if index < len(mimic_chain) else {}
                chrome_names, mimic_names = set(chrome_item.get("names", [])), set(mimic_item.get("names", []))
                chain_diffs.append({
                    "index": index,
                    "chromeTag": chrome_item.get("tag"), "mimicTag": mimic_item.get("tag"),
                    "chromeCtor": chrome_item.get("ctor"), "mimicCtor": mimic_item.get("ctor"),
                    "missingInMimic": sorted(chrome_names - mimic_names),
                    "extraInMimic": sorted(mimic_names - chrome_names),
                })
            expected = {**expected, "value": {key: value for key, value in chrome_value.items() if key != "chain"} | {"chainDiffs": chain_diffs}}
            actual = {**actual, "value": {key: value for key, value in mimic_value.items() if key != "chain"}}
        rows.append({
            "name": name,
            "match": chrome_results[name] == mimic_results[name],
            "chrome": expected,
            "mimic": actual,
        })
    result = {
        "target": chrome_product,
        "mimic": mimic_product,
        "matched": sum(row["match"] for row in rows),
        "total": len(rows),
        "results": rows,
    }
    if summary_only:
        result["results"] = [{"name": row["name"], "match": row["match"], "chromeError": row["chrome"]["error"], "mimicError": row["mimic"]["error"]} for row in rows]
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--chrome", default="http://127.0.0.1:9223")
    parser.add_argument("--mimic", default="http://127.0.0.1:19222")
    parser.add_argument("--probes", default="compatibility/probes.json")
    parser.add_argument("--name", default="")
    parser.add_argument("--probe-timeout", type=float, default=10)
    parser.add_argument("--url", default="")
    parser.add_argument("--summary-only", action="store_true")
    args = parser.parse_args()
    asyncio.run(run(args.chrome, args.mimic, args.probes, args.name, args.probe_timeout, args.url, args.summary_only))
