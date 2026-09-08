"""Open a URL in an existing Mimic instance and save a static snapshot."""

import argparse
import asyncio
import base64
import json
from pathlib import Path, PurePosixPath

from pyppeteer import connect


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

    try:
        response = await page.goto(
            args.url,
            {"waitUntil": "load", "timeout": args.timeout},
        )

        if args.settle_ms:
            await asyncio.sleep(args.settle_ms / 1000)

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
            "status": response.status if response else None,
            "warnings": snapshot["warnings"],
        }
        (output / "snapshot.json").write_text(
            json.dumps(metadata, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )

        print(output / "index.html")
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
        help="Navigation timeout in milliseconds (default: %(default)s)",
    )
    parser.add_argument(
        "--settle-ms",
        type=int,
        default=0,
        help="Extra delay after load before capture (default: %(default)s)",
    )
    return parser.parse_args()


if __name__ == "__main__":
    asyncio.run(save_snapshot(parse_args()))
