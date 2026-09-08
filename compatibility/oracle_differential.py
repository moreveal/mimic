"""Three-way Chrome headful/headless/Mimic compatibility differential."""

from __future__ import annotations

import argparse
import asyncio
import json

from pyppeteer import connect

from differential import observe
from oracle import (PINNED_PRODUCT, capture_metadata, default_profile_id,
                    prepare_page, product)

ENVIRONMENT_SCOPED = frozenset({
    "navigator identity", "display coherence", "webgpu adapter profile",
    "webgpu adapter existence", "webrtc initial state", "intl environment",
    "navigator profile", "screen profile", "webrtc offer lifecycle",
    "webrtc sdp shape", "ua client hints exact",
})


async def metadata(endpoint: str, mode: str, profile_id: str, window_size: str) -> dict:
    browser = await connect(browserURL=endpoint, defaultViewport=None)
    page = await browser.newPage()
    try:
        await prepare_page(browser, page, mode, window_size)
        return await capture_metadata(
            endpoint, browser, page, mode=mode,
            environment_profile_id=profile_id,
        )
    finally:
        await page.close()
        await browser.disconnect()


def classify(probe: dict, headful: dict, headless: dict, mimic: dict) -> str:
    if headful == headless == mimic:
        return "invariant"
    if probe.get("oracleScope") == "environment" or probe.get("name") in ENVIRONMENT_SCOPED:
        return "environment-specific"
    if headful == headless:
        return "Mimic divergence"
    return "headless-specific" if mimic == headful else "Mimic divergence"


async def run(args) -> dict:
    with open(args.probes, encoding="utf-8") as source:
        probes = json.load(source)
    products = {
        "headful": product(args.headful),
        "headless": product(args.headless),
        "mimic": product(args.mimic),
    }
    for mode in ("headful", "headless"):
        if products[mode] != PINNED_PRODUCT:
            raise RuntimeError(f"{mode} oracle drift: {products[mode]!r}")
    headful, headless, mimic, headful_meta, headless_meta = await asyncio.gather(
        observe(args.headful, probes, args.probe_timeout, args.url, "headful", args.window_size),
        observe(args.headless, probes, args.probe_timeout, args.url, "headless", args.window_size),
        observe(args.mimic, probes, args.probe_timeout, args.url),
        metadata(args.headful, "headful", args.headful_profile_id, args.window_size),
        metadata(args.headless, "headless", args.headless_profile_id, args.window_size),
    )
    rows = []
    for probe in probes:
        name = probe["name"]
        rows.append({
            "name": name,
            "classification": classify(probe, headful[name], headless[name], mimic[name]),
            "headfulChrome": headful[name],
            "headlessChrome": headless[name],
            "mimic": mimic[name],
        })
    return {
        "schemaVersion": 1,
        "primaryOracle": "headfulChrome",
        "captures": {
            "headfulChrome": {"product": products["headful"], "captureMetadata": headful_meta},
            "headlessChrome": {"product": products["headless"], "captureMetadata": headless_meta},
            "mimic": {"product": products["mimic"]},
        },
        "summary": {label: sum(row["classification"] == label for row in rows) for label in
                    ("invariant", "headless-specific", "environment-specific", "Mimic divergence")},
        "results": rows,
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--headful", default="http://127.0.0.1:9333")
    parser.add_argument("--headless", default="http://127.0.0.1:9444")
    parser.add_argument("--mimic", default="http://127.0.0.1:19222")
    parser.add_argument("--probes", default="compatibility/probes.json")
    parser.add_argument("--url", default="")
    parser.add_argument("--probe-timeout", type=float, default=10)
    parser.add_argument("--window-size", default="1280x800")
    parser.add_argument("--headful-profile-id", default=default_profile_id("headful"))
    parser.add_argument("--headless-profile-id", default=default_profile_id("headless"))
    parser.add_argument("--output", default="")
    parsed = parser.parse_args()
    result = asyncio.run(run(parsed))
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if parsed.output:
        with open(parsed.output, "w", encoding="utf-8", newline="\n") as destination:
            destination.write(rendered)
    else:
        print(rendered, end="")
