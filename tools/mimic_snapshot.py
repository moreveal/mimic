"""Open a URL in an existing Mimic instance and save a static snapshot."""

import argparse
import asyncio
import base64
import contextlib
import json
import os
import sys
import time
from collections import Counter
from pathlib import Path, PurePosixPath
from urllib.parse import urlsplit

from pyppeteer import connect


SPINNER = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
ASCII_SPINNER = "|/-\\"


async def resolve_loopback_endpoint(endpoint: str) -> str:
    """Race localhost's address families before Pyppeteer's blocking discovery.

    Windows can spend seconds rejecting IPv6 when Mimic listens on IPv4 only.
    Keep HTTPS hostnames intact for certificate verification and remote URLs
    untouched. Use the address that actually accepted a connection, including
    IPv6-only local servers, rather than assuming localhost means IPv4.
    """
    parsed = urlsplit(endpoint)
    if parsed.scheme != "http" or parsed.hostname != "localhost" or parsed.username is not None:
        return endpoint
    _, writer = await asyncio.wait_for(asyncio.open_connection(
        parsed.hostname, parsed.port or 80, happy_eyeballs_delay=0.05, interleave=1,
    ), timeout=10)
    try:
        address = writer.get_extra_info("peername")[0]
    finally:
        writer.close()
        await writer.wait_closed()
    host = f"[{address}]" if ":" in address else address
    return parsed._replace(netloc=f"{host}:{parsed.port or 80}").geturl()


def format_duration(seconds: float) -> str:
    if seconds < 0.001:
        return f"{seconds * 1_000_000:.0f} us"
    if seconds < 1:
        return f"{seconds * 1000:.0f} ms"
    if seconds < 60:
        return f"{seconds:.1f} s"
    minutes, remainder = divmod(seconds, 60)
    return f"{int(minutes)}m {remainder:04.1f}s"


def format_bytes(size: float) -> str:
    value = float(size)
    for unit in ("B", "KiB", "MiB", "GiB"):
        if abs(value) < 1024 or unit == "GiB":
            return f"{value:.0f} {unit}" if unit == "B" else f"{value:.1f} {unit}"
        value /= 1024
    return f"{value:.1f} GiB"


def short_url(raw: object, limit: int = 54) -> str:
    raw_value = str(raw or "").strip()
    if raw_value in {",", "<nil>"}:
        return ""
    try:
        parsed = urlsplit(raw_value)
        value = parsed.netloc + parsed.path
    except (TypeError, ValueError):
        value = raw_value
    if len(value) <= limit:
        return value
    return value[: limit - 3] + "..."


class Reporter:
    """TTY progress renderer with readable log output when stdout is redirected."""

    def __init__(self, *, max_wait: float | None, log_interval: float, live: bool) -> None:
        self.started = time.monotonic()
        self.max_wait = max_wait
        self.log_interval = max(log_interval, 0.1)
        self.interactive = live and sys.stdout.isatty()
        self.color = self.interactive and "NO_COLOR" not in os.environ
        encoding = sys.stdout.encoding or "ascii"
        try:
            "✓⠋…█░".encode(encoding)
            self.unicode = True
        except UnicodeEncodeError:
            self.unicode = False
        self.last_log = self.started
        self.live_visible = False
        self.frame = 0
        self.timings: dict[str, float] = {}

    def paint(self, code: str, value: str) -> str:
        return f"\033[{code}m{value}\033[0m" if self.color else value

    def clear_live(self) -> None:
        if self.live_visible:
            print("\r\033[2K", end="", flush=True)
            self.live_visible = False

    def line(self, message: str) -> None:
        self.clear_live()
        elapsed = time.monotonic() - self.started
        print(f"[{format_duration(elapsed):>8}] {message}", flush=True)

    def blank(self) -> None:
        self.clear_live()
        print(flush=True)

    def done(self, name: str, started: float, detail: str = "") -> None:
        duration = time.monotonic() - started
        self.timings[name] = duration
        suffix = f"  {self.paint('2', detail)}" if detail else ""
        mark = "✓" if self.unicode else "OK"
        self.line(f"{self.paint('32', mark)} {name:<16} {format_duration(duration):>8}{suffix}")

    def warning(self, message: str) -> None:
        self.line(f"{self.paint('33', '!')} {message}")

    def progress_bar(self, elapsed: float, width: int = 18) -> str:
        if self.max_wait:
            ratio = min(max(elapsed / self.max_wait, 0), 1)
            filled = min(width, int(ratio * width))
            full, empty = ("█", "░") if self.unicode else ("#", "-")
            return "[" + full * filled + empty * (width - filled) + "]"
        position = int(elapsed * 5) % (width * 2 - 2)
        if position >= width:
            position = width * 2 - 2 - position
        full, empty = ("█", "░") if self.unicode else ("#", "-")
        cells = [empty] * width
        cells[position] = full
        return "[" + "".join(cells) + "]"

    def navigation(self, progress, ready_state: str, *, force: bool = False) -> None:
        now = time.monotonic()
        elapsed = now - progress.started
        quiet = now - progress.last_activity
        status = f"HTTP {progress.status}" if progress.status is not None else "HTTP ..."
        detail = (
            f"{status} | {ready_state:<11} | "
            f"{progress.events['requests']} req, {len(progress.pending)} active | "
            f"{format_bytes(progress.transferred)} | quiet {format_duration(quiet)}"
        )
        execution = progress.execution
        if execution.get("running"):
            source = execution.get("source") or "task"
            phase = execution.get("phase") or "callback"
            detail += f" | {source}/{phase} busy {format_duration(float(execution.get('elapsedMs') or 0) / 1000)}"
        if progress.last_event:
            detail += f" | {progress.last_event}"
        if self.interactive:
            frames = SPINNER if self.unicode else ASCII_SPINNER
            spinner = frames[self.frame % len(frames)]
            self.frame += 1
            cap = f"/{format_duration(self.max_wait)}" if self.max_wait else ""
            text = f"{spinner} Loading {self.progress_bar(elapsed)} {format_duration(elapsed)}{cap}  {detail}"
            try:
                width = max(40, os.get_terminal_size().columns - 1)
            except OSError:
                width = 120
            print("\r\033[2K" + text[:width], end="", flush=True)
            self.live_visible = True
        elif now - self.last_log >= self.log_interval or (
            force and now - self.last_log >= 0.05
        ):
            self.line(f"... Loading {format_duration(elapsed)}  {detail}")
            self.last_log = now

    def finish(self) -> None:
        self.clear_live()


class NavigationProgress:
    """Track observable navigation progress without making load a hard gate."""

    def __init__(self, main_frame_id: str) -> None:
        self.started = time.monotonic()
        self.main_frame_id = main_frame_id
        self.last_activity = self.started
        self.pending: set[str] = set()
        self.document_seen = False
        self.dom_content_loaded = False
        self.load_fired = False
        self.status = None
        self.transferred = 0.0
        self.last_event = ""
        self.resource_types: Counter[str] = Counter()
        self.responded: set[str] = set()
        self.completed: set[str] = set()
        self.events = {"requests": 0, "responses": 0, "finished": 0, "failed": 0}
        self.execution = {"running": False, "source": "", "taskId": 0, "elapsedMs": 0}
        self.status_supported = True

    def touch(self) -> None:
        self.last_activity = time.monotonic()

    def request(self, event: dict) -> None:
        request_id = event.get("requestId")
        if request_id:
            self.pending.add(request_id)
            self.completed.discard(request_id)
            self.responded.discard(request_id)
        resource_type = str(event.get("type") or "Other")
        self.resource_types[resource_type] += 1
        self.events["requests"] += 1
        if resource_type == "Document" and event.get("frameId") == self.main_frame_id:
            self.document_seen = True
            self.dom_content_loaded = False
            self.load_fired = False
            self.status = None
        request = event.get("request", {})
        method, raw_url = request.get("method", "GET"), request.get("url", "")
        self.last_event = f"{method} {short_url(raw_url)}" if raw_url else resource_type
        self.touch()

    def response(self, event: dict) -> None:
        request_id = event.get("requestId")
        response = event.get("response", {})
        status = response.get("status")
        # Redirect responses can share their request ID. Subframe failures
        # must not replace the final main-document status.
        if event.get("type") == "Document" and event.get("frameId") == self.main_frame_id:
            self.status = status
        if request_id and request_id in self.responded:
            return
        if request_id:
            self.responded.add(request_id)
        self.events["responses"] += 1
        display_url = short_url(response.get("url"))
        self.last_event = (
            f"{status or '...'} {display_url}"
            if display_url
            else f"HTTP {status or '...'}"
        )
        self.touch()

    def finished(self, event: dict, failed: bool = False) -> None:
        request_id = event.get("requestId")
        if request_id and request_id in self.completed:
            return
        if request_id:
            self.completed.add(request_id)
        self.pending.discard(request_id)
        self.events["failed" if failed else "finished"] += 1
        self.transferred += max(float(event.get("encodedDataLength") or 0), 0)
        if failed:
            self.last_event = "request failed"
        self.touch()


async def wait_for_stable_page(progress: NavigationProgress, client, args, reporter: Reporter):
    """Wait for quiescence; return diagnostics even when progress stalls."""
    quiet_seconds = max(args.settle_ms / 1000, 0.5)
    stall_seconds = max(args.timeout / 1000, quiet_seconds)
    max_seconds = args.max_wait / 1000 if args.max_wait else None
    ready_state = "loading"
    reason = "stable"

    while True:
        await asyncio.sleep(0.2)
        now = time.monotonic()
        if progress.status_supported:
            try:
                status = await client.send("Mimic.getStatus")
                progress.execution = status.get("execution", progress.execution)
            except Exception:
                progress.status_supported = False
        ready_state = (
            "complete"
            if progress.load_fired
            else "interactive"
            if progress.dom_content_loaded
            else "loading"
        )
        reporter.navigation(progress, ready_state)

        elapsed = now - progress.started
        # Periodic animation/telemetry turns are normal on live pages. Wait for
        # a complete turn, not a second network-quiet interval after each timer.
        quiet_for = now - progress.last_activity
        runtime_busy = bool(progress.execution.get("running"))
        long_turn = runtime_busy and float(progress.execution.get("elapsedMs") or 0) >= quiet_seconds * 1000
        navigation_observed = progress.document_seen or ready_state != "loading"
        if (
            navigation_observed
            and ready_state != "loading"
            and not long_turn
            and len(progress.pending) <= args.idle_connections
            and quiet_for >= quiet_seconds
        ):
            reason = "stable" if not progress.pending and not runtime_busy else "network-idle"
            break
        if navigation_observed and not runtime_busy and quiet_for >= stall_seconds:
            reason = "stalled"
            break
        if max_seconds is not None and elapsed >= max_seconds:
            reason = "max-wait"
            break

    reporter.navigation(progress, ready_state, force=True)
    reporter.finish()
    return {
        "reason": reason,
        "partial": reason in {"stalled", "max-wait"},
        "elapsedMs": round((time.monotonic() - progress.started) * 1000),
        "quietMs": round((time.monotonic() - progress.last_activity) * 1000),
        "execution": progress.execution,
        "pendingRequests": len(progress.pending),
        "domContentLoaded": progress.dom_content_loaded,
        "loadFired": progress.load_fired,
        "readyState": ready_state,
        "nodes": None,
        "htmlBytes": None,
        "transferredBytes": round(progress.transferred),
        "resourceTypes": dict(progress.resource_types.most_common()),
        **progress.events,
    }


async def await_with_progress(awaitable, reporter: Reporter, label: str):
    """Keep a slow CDP operation visibly alive without imposing a new timeout."""
    task = asyncio.ensure_future(awaitable)
    started = time.monotonic()
    while not task.done():
        try:
            await asyncio.wait_for(asyncio.shield(task), timeout=0.2)
        except asyncio.TimeoutError:
            elapsed = time.monotonic() - started
            if reporter.interactive:
                frames = SPINNER if reporter.unicode else ASCII_SPINNER
                spinner = frames[reporter.frame % len(frames)]
                reporter.frame += 1
                print(f"\r\033[2K{spinner} {label}... {format_duration(elapsed)}", end="", flush=True)
                reporter.live_visible = True
            elif time.monotonic() - reporter.last_log >= reporter.log_interval:
                reporter.line(f"... {label} still running ({format_duration(elapsed)})")
                reporter.last_log = time.monotonic()
    reporter.finish()
    return await task


def decode_files(snapshot: dict) -> dict[PurePosixPath, bytes]:
    """Decode and validate snapshot files before writing anything to disk."""
    files = {}
    for name, encoded in snapshot["files"].items():
        relative = PurePosixPath(name)
        if relative.is_absolute() or ".." in relative.parts or "\\" in name or ":" in name:
            raise ValueError(f"Invalid snapshot path: {name!r}")
        # Older servers serialize a nil Go []byte as null for empty assets.
        files[relative] = b"" if encoded is None else base64.b64decode(encoded, validate=True)
    return files


async def save_snapshot(args: argparse.Namespace) -> None:
    total_started = time.monotonic()
    output = Path(args.output).resolve()
    if output.exists():
        raise FileExistsError(f"Output directory already exists: {output}")

    max_wait = args.max_wait / 1000 if args.max_wait else None
    reporter = Reporter(
        max_wait=max_wait,
        log_interval=args.log_interval,
        live=not args.no_progress,
    )
    reporter.line(f"Mimic snapshot  {short_url(args.url, 90)}")
    reporter.line(
        f"Endpoint {args.endpoint} | quiet {format_duration(max(args.settle_ms / 1000, 0.5))} "
        f"| idle <= {args.idle_connections} | "
        f"stall {format_duration(max(args.timeout / 1000, 0.5))}"
    )

    browser = None
    page = None
    try:
        stage = time.monotonic()
        browser = await connect(browserURL=await resolve_loopback_endpoint(args.endpoint), defaultViewport=None)
        reporter.done("Connect", stage, args.endpoint)

        stage = time.monotonic()
        page = await browser.newPage()
        reporter.done("New page", stage)
        progress = NavigationProgress(page.mainFrame._id)

        page._client.on("Network.requestWillBeSent", progress.request)
        page._client.on("Network.responseReceived", progress.response)
        page._client.on("Network.loadingFinished", progress.finished)
        page._client.on("Network.loadingFailed", lambda event: progress.finished(event, failed=True))

        def dom_content_loaded(_event: dict) -> None:
            progress.dom_content_loaded = True
            progress.last_event = "DOMContentLoaded"
            progress.touch()

        def load_fired(_event: dict) -> None:
            progress.load_fired = True
            progress.last_event = "load"
            progress.touch()

        page._client.on("Page.domContentEventFired", dom_content_loaded)
        page._client.on("Page.loadEventFired", load_fired)

        navigation = asyncio.create_task(page.goto(args.url, {"waitUntil": "load", "timeout": 0}))
        stage = time.monotonic()
        diagnostics = await wait_for_stable_page(progress, page._client, args, reporter)
        reporter.done(
            "Navigation",
            stage,
            f"{diagnostics['reason']}, HTTP {progress.status or '...'}, {progress.events['requests']} requests",
        )

        navigation_error = None
        response = None
        interrupt = diagnostics["reason"] == "max-wait"
        if interrupt:
            diagnostics["execution"]["interruptRequested"] = True
            reporter.warning("Maximum wait reached; capturing partial DOM at an interrupted task boundary")
        if navigation.done():
            try:
                response = navigation.result()
            except Exception as error:
                navigation_error = str(error)
        else:
            stage = time.monotonic()
            await page._client.send("Page.stopLoading")
            navigation.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await navigation
            reporter.done("Stop loading", stage, "committed DOM preserved")

        stage = time.monotonic()
        snapshot = await await_with_progress(
            page._client.send("Mimic.captureSnapshot", {"interrupt": interrupt}), reporter, "Capturing DOM and assets"
        )
        reporter.done("Capture", stage, f"{len(snapshot['files'])} files")
        capture_timings = snapshot.get("timingsMs", {})
        if capture_timings:
            reporter.line(
                "Capture internals  "
                + " | ".join(f"{name} {value} ms" for name, value in capture_timings.items())
            )

        stage = time.monotonic()
        files = decode_files(snapshot)
        diagnostics["htmlBytes"] = len(files.get(PurePosixPath("index.html"), b""))
        decoded_bytes = sum(map(len, files.values()))
        reporter.done("Decode", stage, format_bytes(decoded_bytes))

        stage = time.monotonic()
        output.mkdir(parents=True, exist_ok=False)
        for relative, data in files.items():
            destination = output.joinpath(*relative.parts)
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(data)

        status = progress.status
        if status is None and response:
            status = response.status
        reporter.done("Write", stage, str(output))
        stage = time.monotonic()
        await await_with_progress(page.close(), reporter, "Closing page")
        page = None
        await browser.disconnect()
        browser = None
        reporter.done("Close", stage)
        reporter.timings["Total"] = time.monotonic() - total_started
        metadata = {
            "url": snapshot["url"],
            "status": status,
            "warnings": snapshot["warnings"],
            "navigation": diagnostics,
            "captureTimingsMs": capture_timings,
            "timingsMs": {name: round(duration * 1000) for name, duration in reporter.timings.items()},
        }
        if navigation_error:
            metadata["navigation"]["error"] = navigation_error
        (output / "snapshot.json").write_text(
            json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8"
        )

        reporter.blank()
        reporter.line(f"{reporter.paint('1;32', 'Snapshot saved')}  {output / 'index.html'}")
        reporter.line(
            f"HTTP {status or '...'} | {diagnostics['reason']} | HTML {format_bytes(diagnostics['htmlBytes'])} "
            f"| assets {max(len(files) - 1, 0)} | total {format_bytes(decoded_bytes)}"
        )
        reporter.line(
            "Timings  "
            + " | ".join(
                f"{name} {format_duration(duration)}" for name, duration in reporter.timings.items()
            )
        )
        if diagnostics["partial"]:
            reporter.warning("Navigation did not fully settle; saved the committed DOM")
        for warning in snapshot["warnings"]:
            reporter.warning(warning)
    finally:
        reporter.finish()
        if page is not None:
            with contextlib.suppress(Exception):
                await page.close()
        if browser is not None:
            with contextlib.suppress(Exception):
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
    parser.add_argument(
        "--idle-connections",
        type=int,
        default=2,
        help="Active requests still considered network-idle (default: %(default)s)",
    )
    parser.add_argument(
        "--log-interval",
        type=float,
        default=10,
        help="Seconds between progress logs outside an interactive terminal (default: %(default)s)",
    )
    parser.add_argument(
        "--no-progress",
        action="store_true",
        help="Disable the animated progress line",
    )
    return parser.parse_args()


def main() -> int:
    try:
        asyncio.run(save_snapshot(parse_args()))
        return 0
    except KeyboardInterrupt:
        print("\nCancelled by user", file=sys.stderr)
        return 130
    except Exception as error:
        print(f"\nSnapshot failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
