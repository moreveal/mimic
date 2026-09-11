"""Open a URL in an existing Mimic instance and save a static snapshot."""

import argparse
import asyncio
import base64
import contextlib
import json
import time
from pathlib import Path, PurePosixPath

from pyppeteer import connect


class NavigationProgress:
    """Track observable navigation progress without making load a hard gate."""

    def __init__(self) -> None:
        self.started = time.monotonic()
        self.last_activity = self.started
        self.pending: set[str] = set()
        self.document_seen = False
        self.dom_content_loaded = False
        self.load_fired = False
        self.status = None
        self.events = {"requests": 0, "responses": 0, "finished": 0, "failed": 0}

    def touch(self) -> None:
        self.last_activity = time.monotonic()

    def request(self, event: dict) -> None:
        request_id = event.get("requestId")
        if request_id:
            self.pending.add(request_id)
        self.events["requests"] += 1
        if event.get("type") == "Document":
            self.document_seen = True
        self.touch()

    def response(self, event: dict) -> None:
        self.events["responses"] += 1
        if event.get("type") == "Document":
            self.status = event.get("response", {}).get("status")
        self.touch()

    def finished(self, event: dict, failed: bool = False) -> None:
        self.pending.discard(event.get("requestId"))
        self.events["failed" if failed else "finished"] += 1
        self.touch()


async def wait_for_stable_page(page, progress: NavigationProgress, args):
    """Wait for quiescence; return diagnostics even when progress stalls."""
    quiet_seconds = max(args.settle_ms / 1000, 0.5)
    stall_seconds = max(args.timeout / 1000, quiet_seconds)
    max_seconds = args.max_wait / 1000 if args.max_wait else None
    last_dom = None
    sample = {"readyState": "loading", "nodes": 0, "htmlBytes": 0}
    reason = "stable"

    while True:
        await asyncio.sleep(0.5)
        now = time.monotonic()
        try:
            sample = await page.evaluate(
                """()=>({readyState:document.readyState,
                nodes:document.getElementsByTagName('*').length,
                htmlBytes:new TextEncoder().encode(document.documentElement?.outerHTML||'').length})"""
            )
            signature = (sample["readyState"], sample["nodes"], sample["htmlBytes"])
            if signature != last_dom:
                last_dom = signature
                progress.touch()
        except Exception as error:
            sample = {**sample, "sampleError": str(error)}

        elapsed = now - progress.started
        quiet_for = now - progress.last_activity
        navigation_observed = progress.document_seen or sample["readyState"] != "loading"
        if navigation_observed and not progress.pending and quiet_for >= quiet_seconds:
            break
        if navigation_observed and quiet_for >= stall_seconds:
            reason = "stalled"
            break
        if max_seconds is not None and elapsed >= max_seconds:
            reason = "max-wait"
            break

    return {
        "reason": reason,
        "partial": reason != "stable" or not progress.load_fired,
        "elapsedMs": round((time.monotonic() - progress.started) * 1000),
        "quietMs": round((time.monotonic() - progress.last_activity) * 1000),
        "pendingRequests": len(progress.pending),
        "domContentLoaded": progress.dom_content_loaded,
        "loadFired": progress.load_fired,
        **sample,
        **progress.events,
    }


def decode_files(snapshot: dict) -> dict[PurePosixPath, bytes]:
    """Decode and validate snapshot files before writing anything to disk."""
    files = {}
    for name, encoded in snapshot["files"].items():
        relative = PurePosixPath(name)
        if (
            relative.is_absolute()
            or ".." in relative.parts
            or "\\" in name
            or ":" in name
        ):
            raise ValueError(f"Invalid snapshot path: {name!r}")
        files[relative] = base64.b64decode(encoded, validate=True)
    return files


async def save_snapshot(args: argparse.Namespace) -> None:
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    progress = NavigationProgress()

    page._client.on("Network.requestWillBeSent", progress.request)
    page._client.on("Network.responseReceived", progress.response)
    page._client.on("Network.loadingFinished", progress.finished)
    page._client.on(
        "Network.loadingFailed", lambda event: progress.finished(event, failed=True)
    )

    def dom_content_loaded(_event: dict) -> None:
        progress.dom_content_loaded = True
        progress.touch()

    def load_fired(_event: dict) -> None:
        progress.load_fired = True
        progress.touch()

    page._client.on("Page.domContentEventFired", dom_content_loaded)
    page._client.on("Page.loadEventFired", load_fired)

    try:
        navigation = asyncio.create_task(page.goto(
            args.url,
            {"waitUntil": "load", "timeout": 0},
        ))
        diagnostics = await wait_for_stable_page(page, progress, args)
        navigation_error = None
        response = None
        if navigation.done():
            try:
                response = navigation.result()
            except Exception as error:
                navigation_error = str(error)
        else:
            navigation.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await navigation

        snapshot = await page._client.send("Mimic.captureSnapshot")
        files = decode_files(snapshot)

        output = Path(args.output).resolve()
        output.mkdir(parents=True, exist_ok=False)

        for relative, data in files.items():
            destination = output.joinpath(*relative.parts)
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(data)

        metadata = {
            "url": snapshot["url"],
            "status": response.status if response else progress.status,
            "warnings": snapshot["warnings"],
            "navigation": diagnostics,
        }
        if navigation_error:
            metadata["navigation"]["error"] = navigation_error
        (output / "snapshot.json").write_text(
            json.dumps(metadata, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )

        print(output / "index.html")
        print(
            f"Navigation: {diagnostics['reason']}; "
            f"readyState={diagnostics['readyState']}; "
            f"pending={diagnostics['pendingRequests']}; "
            f"elapsed={diagnostics['elapsedMs']} ms"
        )
        if diagnostics["partial"]:
            print("Warning: Navigation did not fully settle; saved the current DOM")
        for warning in snapshot["warnings"]:
            print(f"Warning: {warning}")
    finally:
        await page.close()
        await browser.disconnect()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("url", help="URL to open through Mimic")
    parser.add_argument("output", help="New directory for the snapshot")
    parser.add_argument(
        "--endpoint",
        default="http://127.0.0.1:9222",
        help="Mimic HTTP discovery endpoint (default: %(default)s)",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=30_000,
        help="No-progress timeout before a partial capture (default: %(default)s)",
    )
    parser.add_argument(
        "--max-wait",
        type=int,
        default=120_000,
        help="Absolute wait cap before a partial capture; 0 disables (default: %(default)s)",
    )
    parser.add_argument(
        "--settle-ms",
        type=int,
        default=0,
        help="Required quiet period before capture (default: %(default)s)",
    )
    return parser.parse_args()


if __name__ == "__main__":
    asyncio.run(save_snapshot(parse_args()))
