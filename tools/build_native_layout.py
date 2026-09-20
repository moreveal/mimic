#!/usr/bin/env python3
"""Compatibility entry point for Mimic's content-addressed native build."""

from __future__ import annotations

import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def main() -> None:
    subprocess.run(
        ["go", "run", "./tools/buildnative"],
        check=True,
        cwd=ROOT,
    )


if __name__ == "__main__":
    main()
