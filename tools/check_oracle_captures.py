#!/usr/bin/env python3
"""Verify that retained Chrome oracle observations have mandatory provenance."""

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
REQUIRED = {
    "chromeVersion", "chromiumRevision", "v8Version", "platform", "browserMode",
    "commandLineFeatureOverrides", "viewport", "window", "secureContextState",
    "isolationState", "environmentProfileId",
    "profileFreshness",
}
paths = list((ROOT / "chrome" / "152" / "generated").glob("window-*.json"))
paths += [ROOT / "chrome" / "152" / "generated" / "worker-secure.json"]
paths += list((ROOT / "compatibility" / "captures").glob("navigator-chrome152*.json"))
paths += list((ROOT / "compatibility" / "captures" / "semantic-checkpoints").glob("browserscan-path-*.json"))
issues = []
for path in paths:
    data = json.loads(path.read_text(encoding="utf-8"))
    if "152.0.7977.82" not in json.dumps(data):
        continue
    metadata = data.get("captureMetadata", {})
    missing = sorted(REQUIRED - metadata.keys())
    if missing:
        issues.append(f"{path.relative_to(ROOT)}: missing {', '.join(missing)}")
    if metadata.get("browserMode") not in {"headful", "headless"}:
        issues.append(f"{path.relative_to(ROOT)}: invalid browserMode")
if issues:
    print("\n".join(f"FAIL {issue}" for issue in issues))
    raise SystemExit(1)
print(f"PASS: {len(paths)} retained oracle captures carry explicit provenance")
