"""
Use Pyppeteer to click blast.hk's Other games section and save a snapshot.

Run against a freshly built Mimic server:

    python compatibility/research/blast_other_games.py --output snapshots/blast

Use --chrome with a frozen Chrome endpoint to save an MHTML reference instead.
"""

import argparse
import asyncio
import json
from pathlib import Path
import sys
import time

from pyppeteer import connect


ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools"))

from mimic_snapshot import decode_files, resolve_loopback_endpoint


class Timer:
    def __init__(self):
        self.started = time.perf_counter()

    def elapsed(self) -> float:
        return time.perf_counter() - self.started


def log(message: str):
    now = time.strftime("%H:%M:%S")
    print(f"[{now}] {message}", flush=True)


async def run(args):
    total_timer = Timer()

    output = Path(args.output).resolve()
    if output.exists():
        raise FileExistsError(
            f"Snapshot directory already exists: {output}"
        )

    diagnostics = {
        "exceptions": [],
        "consoleErrors": [],
        "failedRequests": [],
        "timings": {},
    }

    browser = None
    page = None

    try:
        # ------------------------------------------------------------
        # Connect
        # ------------------------------------------------------------

        timer = Timer()

        log(f"Connecting to CDP endpoint: {args.endpoint}")

        browser = await connect(
            browserURL=await resolve_loopback_endpoint(args.endpoint),
            defaultViewport=None,
        )

        diagnostics["timings"]["connect"] = timer.elapsed()

        log(
            f"Connected in "
            f"{diagnostics['timings']['connect']:.3f}s"
        )

        # ------------------------------------------------------------
        # New page
        # ------------------------------------------------------------

        timer = Timer()

        log("Creating new page...")

        page = await browser.newPage()
        page.setDefaultNavigationTimeout(args.timeout)

        diagnostics["timings"]["newPage"] = timer.elapsed()

        log(
            f"Page created in "
            f"{diagnostics['timings']['newPage']:.3f}s"
        )

        # ------------------------------------------------------------
        # Diagnostics
        # ------------------------------------------------------------

        page._client.on(
            "Runtime.exceptionThrown",
            diagnostics["exceptions"].append,
        )

        page.on(
            "console",
            lambda message: (
                diagnostics["consoleErrors"].append(message.text)
                if message.type == "error"
                else None
            ),
        )

        page.on(
            "requestfailed",
            lambda request: diagnostics["failedRequests"].append(
                {
                    "url": request.url,
                    "failure": request.failure(),
                }
            ),
        )

        # Optional request logging
        if args.log_requests:
            page.on(
                "request",
                lambda request: log(
                    f"REQUEST  {request.method} {request.url}"
                ),
            )

            page.on(
                "response",
                lambda response: log(
                    f"RESPONSE {response.status} {response.url}"
                ),
            )

        # ------------------------------------------------------------
        # Initial navigation
        # ------------------------------------------------------------

        timer = Timer()

        log(f"Opening: {args.url}")

        response = await page.goto(
            args.url,
            waitUntil="domcontentloaded",
        )

        diagnostics["timings"]["initialGoto"] = timer.elapsed()
        diagnostics["initialStatus"] = (
            response.status if response else None
        )

        log(
            f"Initial navigation finished in "
            f"{diagnostics['timings']['initialGoto']:.3f}s "
            f"(HTTP {diagnostics['initialStatus']})"
        )

        # ------------------------------------------------------------
        # Initial page state
        # ------------------------------------------------------------

        timer = Timer()

        initial_state = await page.evaluate(
            """() => ({
                url: location.href,
                title: document.title,
                readyState: document.readyState,
                nodeCount: document.getElementsByTagName('*').length
            })"""
        )

        diagnostics["timings"]["initialStateEval"] = timer.elapsed()
        diagnostics["initialState"] = initial_state

        log(
            "Initial page state: "
            f"url={initial_state['url']!r}, "
            f"title={initial_state['title']!r}, "
            f"readyState={initial_state['readyState']!r}, "
            f"nodes={initial_state['nodeCount']}"
        )

        log(
            f"Initial state evaluate took "
            f"{diagnostics['timings']['initialStateEval']:.3f}s"
        )

        # ------------------------------------------------------------
        # Find "Другие игры"
        # ------------------------------------------------------------

        find_link = """
            Array.from(
                document.querySelectorAll('.node-title a')
            ).find(
                a => a.textContent.trim() === 'Другие игры'
            )
        """

        async def wait_for_link():
            attempts = 0
            timer = Timer()

            while True:
                attempts += 1

                eval_timer = Timer()

                href = await page.evaluate(
                    f"() => ({find_link})?.href"
                )

                eval_elapsed = eval_timer.elapsed()

                if args.verbose_poll:
                    log(
                        f"Link poll #{attempts}: "
                        f"{eval_elapsed:.4f}s, "
                        f"found={bool(href)}"
                    )

                if href:
                    return href, attempts, timer.elapsed()

                await asyncio.sleep(0.1)

        log("Looking for 'Другие игры' link...")

        href, attempts, elapsed = await asyncio.wait_for(
            wait_for_link(),
            args.timeout / 1000,
        )

        diagnostics["timings"]["findLink"] = elapsed
        diagnostics["findLinkAttempts"] = attempts
        diagnostics["targetHref"] = href

        log(
            f"Found link in {elapsed:.3f}s "
            f"after {attempts} attempt(s)"
        )

        log(f"Target: {href}")

        # ------------------------------------------------------------
        # Prepare navigation wait
        # ------------------------------------------------------------

        log("Preparing navigation waiter...")

        nav_task = asyncio.ensure_future(
            page.waitForNavigation(
                waitUntil="domcontentloaded",
                timeout=args.timeout,
            )
        )

        # Give waitForNavigation a chance to subscribe before click.
        await asyncio.sleep(0)

        # ------------------------------------------------------------
        # Click only
        # ------------------------------------------------------------

        timer = Timer()

        log("Clicking link...")

        click_result = await page.evaluate(
            f"""() => {{
                const el = ({find_link});

                if (!el) {{
                    throw new Error(
                        "'Другие игры' link disappeared before click"
                    );
                }}

                el.click();

                return true;
            }}"""
        )

        diagnostics["timings"]["clickEvaluate"] = timer.elapsed()
        diagnostics["clickResult"] = click_result

        log(
            f"Click evaluate returned in "
            f"{diagnostics['timings']['clickEvaluate']:.3f}s"
        )

        # ------------------------------------------------------------
        # Navigation only
        # ------------------------------------------------------------

        timer = Timer()

        log("Waiting for navigation / DOMContentLoaded...")

        response = await nav_task

        diagnostics["timings"]["navigationAfterClick"] = (
            timer.elapsed()
        )

        diagnostics["status"] = (
            response.status if response else None
        )

        log(
            f"Navigation after click finished in "
            f"{diagnostics['timings']['navigationAfterClick']:.3f}s "
            f"(HTTP {diagnostics['status']})"
        )

        if response and response.status >= 400:
            raise RuntimeError(
                f"Section returned HTTP {response.status}"
            )

        # ------------------------------------------------------------
        # Optional settle
        # ------------------------------------------------------------

        if args.settle_ms > 0:
            timer = Timer()

            log(
                f"Settling for {args.settle_ms} ms..."
            )

            await asyncio.sleep(
                args.settle_ms / 1000
            )

            diagnostics["timings"]["settle"] = timer.elapsed()

            log(
                f"Settle completed in "
                f"{diagnostics['timings']['settle']:.3f}s"
            )
        else:
            diagnostics["timings"]["settle"] = 0.0

        # ------------------------------------------------------------
        # Final state
        # ------------------------------------------------------------

        timer = Timer()

        log("Reading final page state...")

        state = await page.evaluate(
            """() => ({
                url: location.href,
                title: document.title,
                heading:
                    document.querySelector('h1')
                        ?.textContent
                        .trim() || '',
                readyState: document.readyState,
                nodeCount:
                    document.getElementsByTagName('*').length
            })"""
        )

        diagnostics["timings"]["finalStateEval"] = timer.elapsed()

        diagnostics.update(state)

        log(
            "Final state: "
            f"url={state['url']!r}, "
            f"title={state['title']!r}, "
            f"heading={state['heading']!r}, "
            f"readyState={state['readyState']!r}, "
            f"nodes={state['nodeCount']}"
        )

        log(
            f"Final state evaluate took "
            f"{diagnostics['timings']['finalStateEval']:.3f}s"
        )

        if state["url"] != href:
            raise RuntimeError(
                "Section navigation URL mismatch: "
                f"expected={href!r}, actual={state['url']!r}"
            )

        if "Другие игры" not in state["heading"]:
            raise RuntimeError(
                f"Section heading was not confirmed: {state!r}"
            )

        # ------------------------------------------------------------
        # Snapshot
        # ------------------------------------------------------------

        timer = Timer()

        if args.chrome:
            log("Capturing Chrome MHTML snapshot...")

            snapshot = await page._client.send(
                "Page.captureSnapshot",
                {
                    "format": "mhtml",
                },
            )

            files = {
                Path("index.mhtml"):
                    snapshot["data"].encode("utf-8")
            }

        else:
            log("Capturing Mimic snapshot...")

            snapshot = await page._client.send(
                "Mimic.captureSnapshot"
            )

            files = decode_files(snapshot)

            diagnostics["warnings"] = snapshot.get(
                "warnings",
                [],
            )

        diagnostics["timings"]["captureSnapshot"] = (
            timer.elapsed()
        )

        log(
            f"Snapshot captured in "
            f"{diagnostics['timings']['captureSnapshot']:.3f}s "
            f"({len(files)} file(s))"
        )

        # ------------------------------------------------------------
        # Save files
        # ------------------------------------------------------------

        timer = Timer()

        log(f"Writing snapshot to: {output}")

        output.mkdir(
            parents=True,
            exist_ok=False,
        )

        total_bytes = 0

        for relative, data in files.items():
            destination = output.joinpath(
                *relative.parts
            )

            destination.parent.mkdir(
                parents=True,
                exist_ok=True,
            )

            destination.write_bytes(data)
            total_bytes += len(data)

        diagnostics["timings"]["writeSnapshot"] = (
            timer.elapsed()
        )

        diagnostics["snapshotFiles"] = len(files)
        diagnostics["snapshotBytes"] = total_bytes

        diagnostics["totalTime"] = total_timer.elapsed()

        # ------------------------------------------------------------
        # Save diagnostics
        # ------------------------------------------------------------

        (output / "snapshot.json").write_text(
            json.dumps(
                diagnostics,
                ensure_ascii=False,
                indent=2,
            ),
            encoding="utf-8",
        )

        log(
            f"Snapshot files written in "
            f"{diagnostics['timings']['writeSnapshot']:.3f}s"
        )

        log(
            f"Saved {len(files)} file(s), "
            f"{total_bytes / 1024:.1f} KiB"
        )

        # ------------------------------------------------------------
        # Summary
        # ------------------------------------------------------------

        log("")
        log("=== Timing summary ===")

        for name, value in diagnostics["timings"].items():
            log(f"{name:24s} {value:8.3f}s")

        log(
            f"{'TOTAL':24s} "
            f"{diagnostics['totalTime']:8.3f}s"
        )

        log("")
        log(
            f"JavaScript exceptions: "
            f"{len(diagnostics['exceptions'])}"
        )

        log(
            f"Console errors: "
            f"{len(diagnostics['consoleErrors'])}"
        )

        log(
            f"Failed requests: "
            f"{len(diagnostics['failedRequests'])}"
        )

        if diagnostics.get("warnings"):
            log(
                f"Snapshot warnings: "
                f"{len(diagnostics['warnings'])}"
            )

    finally:
        if page is not None:
            close_timer = Timer()

            try:
                log("Closing page...")
                await page.close()

                log(
                    f"Page closed in "
                    f"{close_timer.elapsed():.3f}s"
                )
            except Exception as exc:
                log(
                    f"Page close failed: {exc!r}"
                )

        if browser is not None:
            try:
                log("Disconnecting...")
                await browser.disconnect()
            except Exception as exc:
                log(
                    f"Browser disconnect failed: {exc!r}"
                )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description=__doc__,
    )

    parser.add_argument(
        "--endpoint",
        default="http://127.0.0.1:9222",
    )

    parser.add_argument(
        "--url",
        default="https://www.blast.hk/",
    )

    parser.add_argument(
        "--output",
        required=True,
        help="New snapshot directory",
    )

    parser.add_argument(
        "--timeout",
        type=int,
        default=60000,
        help="Navigation timeout, ms",
    )

    parser.add_argument(
        "--settle-ms",
        type=int,
        default=0,
        help="Extra wait after navigation, ms",
    )

    parser.add_argument(
        "--chrome",
        action="store_true",
        help="Save Chrome MHTML reference",
    )

    parser.add_argument(
        "--log-requests",
        action="store_true",
        help="Log all requests and responses",
    )

    parser.add_argument(
        "--verbose-poll",
        action="store_true",
        help="Log every link polling attempt",
    )

    asyncio.run(
        run(parser.parse_args())
    )

