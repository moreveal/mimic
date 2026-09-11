"""Compatibility entry point for the original snapshot command."""
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from mimic_snapshot import main

if __name__ == "__main__":
    raise SystemExit(main())
