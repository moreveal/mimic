"""Save an existing Mimic page as index.html and local assets (requires pyppeteer)."""
import argparse
import asyncio
import base64
import json
from pathlib import Path, PurePosixPath
from pyppeteer import connect


async def wait_for_selector(page, selector, timeout_ms):
    """Wait for application readiness without exporting a remote DOM handle."""
    async def poll():
        while not await page.evaluate(
            "selector => Boolean(document.querySelector(selector))", selector
        ):
            await asyncio.sleep(0.1)
    await asyncio.wait_for(poll(), timeout=timeout_ms / 1000)


async def save(args):
    browser = await connect(browserURL=args.endpoint)
    try:
        pages = await browser.pages()
        if args.page < 0 or args.page >= len(pages):
            raise ValueError(f"Page index {args.page} unavailable; {len(pages)} page(s) open")
        if args.wait_selector:
            await wait_for_selector(pages[args.page], args.wait_selector, args.timeout)
        session = await pages[args.page].target.createCDPSession()
        snapshot = await session.send("Mimic.captureSnapshot")
        # Validate before touching disk; never overwrite an existing snapshot.
        files = {}
        for name, encoded in snapshot["files"].items():
            relative = PurePosixPath(name)
            if relative.is_absolute() or ".." in relative.parts or "\\" in name or ":" in name:
                raise ValueError(f"Invalid snapshot path: {name!r}")
            # Older servers serialize a nil Go []byte as null for empty assets.
            files[relative] = b"" if encoded is None else base64.b64decode(encoded, validate=True)
        output = Path(args.output).resolve()
        output.mkdir(parents=True, exist_ok=False)
        for relative, data in files.items():
            dest = output.joinpath(*relative.parts)
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(data)
        (output / "snapshot.json").write_text(json.dumps({
            "url": snapshot["url"], "warnings": snapshot["warnings"]
        }, ensure_ascii=False, indent=2), encoding="utf-8")
        print(output / "index.html")
        for warning in snapshot["warnings"]:
            print(f"Warning: {warning}")
    finally:
        await browser.disconnect()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--endpoint", default="http://127.0.0.1:9222")
    parser.add_argument("--page", type=int, default=0, help="Zero-based open page index")
    parser.add_argument("--output", required=True, help="New snapshot directory")
    parser.add_argument("--wait-selector", help="CSS selector indicating application readiness")
    parser.add_argument("--timeout", type=int, default=30_000, help="Readiness timeout in milliseconds")
    asyncio.run(save(parser.parse_args()))
