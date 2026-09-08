#!/usr/bin/env python3
"""One-time, idempotent provenance annotation for retained Chrome 152 evidence."""

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CAPTURES = ROOT / "compatibility" / "captures" / "semantic-checkpoints"
METADATA = {
    "schemaVersion": 1,
    "chromeVersion": "152.0.7977.82",
    "chromiumRevision": 1669021,
    "chromiumCommit": "d04cdb24d67b081f6cf80200ffc5233f44b61109",
    "v8Version": "15.2.124.21",
    "platform": "windows-x64",
    "browserMode": "headless",
    "commandLineFeatureOverrides": [],
    "viewport": {"width": 772, "height": 433, "deviceScaleFactor": 1},
    "window": {"width": 780, "height": 580},
    "secureContextState": True,
    "isolationState": False,
    "environmentProfileId": "historical-chrome-152-windows-x64-headless-uncontrolled",
    "profileFreshness": "historical-unverified",
    "provenance": "retained historical differential; mode and geometry recovered from its pinned runner profile",
}


changed = 0
for path in sorted(CAPTURES.glob("browserscan-path-*.json")):
    data = json.loads(path.read_text(encoding="utf-8"))
    if "152.0.7977.82" not in json.dumps(data):
        continue
    if data.get("captureMetadata") == METADATA:
        continue
    data["captureMetadata"] = METADATA
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8", newline="\n")
    changed += 1
print(f"annotated {changed} historical Chrome captures")
