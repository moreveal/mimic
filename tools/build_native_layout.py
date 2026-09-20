#!/usr/bin/env python3
"""Build the shared Blitz and legacy Taffy migration archive."""

from __future__ import annotations

import platform
import shutil
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "internal" / "layoutblitz" / "native" / "Cargo.toml"


def main() -> None:
    machine = platform.machine().lower()
    if machine not in {"amd64", "x86_64"}:
        raise SystemExit(f"native Taffy layout is not validated on {machine}")
    target = (
        "x86_64-pc-windows-gnu"
        if platform.system() == "Windows"
        else "x86_64-unknown-linux-gnu"
    )
    if platform.system() not in {"Windows", "Linux"}:
        raise SystemExit(f"native Taffy layout is not supported on {platform.system()}")
    rustup = shutil.which("rustup")
    if rustup:
        subprocess.run([rustup, "target", "add", target], check=True, cwd=ROOT)
    subprocess.run(
        [
            "cargo",
            "build",
            "--manifest-path",
            str(MANIFEST),
            "--target",
            target,
            "--release",
            "--locked",
        ],
        check=True,
        cwd=ROOT,
    )


if __name__ == "__main__":
    main()
