"""Compare descriptor-only pre-packer snapshots without exporting payload values.

Inputs are private payload-probes.json files produced by the local VM capture
analyzer. Output contains field identifiers and equality flags only. A numbered
stage is expanded one level; nested result arrays/objects are compared as values,
not claimed to have been individually reverse engineered.
"""
import argparse
import csv
import json
from pathlib import Path


def unpack(value):
    if isinstance(value, dict) and "properties" in value:
        return {key: unpack(item) for key, item in value["properties"].items()}
    return value


def inputs(path):
    records = json.loads(path.read_text(encoding="utf-8-sig"))
    return [unpack(row["value"]) for row in records if row["phase"] == "input"]


def registry(runs, notes):
    sizes = {len(packets) for packets in runs.values()}
    if len(sizes) != 1:
        raise ValueError("Runs have different packet counts; align them first")
    missing = object()
    for pair, (chrome, mimic) in enumerate(zip(runs["chrome"], runs["mimic"])):
        for top in sorted(chrome.keys() | mimic.keys()):
            numbered = top.isdigit() and isinstance(chrome.get(top, mimic.get(top)), dict)
            fields = sorted(chrome.get(top, {}).keys() | mimic.get(top, {}).keys()) if numbered else [top]
            for field in fields:
                def get(name):
                    packet = runs[name][pair]
                    return packet.get(top, {}).get(field, missing) if numbered else packet.get(field, missing)

                different = get("chrome") != get("mimic")
                row = {"pair": pair, "block": top if numbered else "", "field": field,
                       "different": different,
                       "review": notes.get(field, {}).get("status", "unmapped" if different else "equal")}
                for runtime in ("chrome", "mimic"):
                    for suffix in ("Repeat", "Scan"):
                        name = runtime + suffix
                        if name in runs:
                            row[name + "Equal"] = get(runtime) == get(name)
                yield row


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("chrome", "mimic", "chrome-repeat", "mimic-repeat", "chrome-scan", "mimic-scan"):
        parser.add_argument("--" + name, type=Path, required=name in ("chrome", "mimic"))
    parser.add_argument("--notes", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    runs = {}
    for name in ("chrome", "mimic", "chromeRepeat", "mimicRepeat", "chromeScan", "mimicScan"):
        key = name.replace("Repeat", "_repeat").replace("Scan", "_scan")
        if path := getattr(args, key):
            runs[name] = inputs(path)
    notes = json.loads(args.notes.read_text(encoding="utf-8-sig"))
    rows = list(registry(runs, notes))
    if not rows:
        raise ValueError("No input packets")
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w", newline="", encoding="utf-8") as output:
        writer = csv.DictWriter(output, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    print(json.dumps({"rows": len(rows), "differentRows": sum(r["different"] for r in rows),
                      "differentFields": len({r["field"] for r in rows if r["different"]}),
                      "unmappedFields": sorted({r["field"] for r in rows if r["review"] == "unmapped"})}))


if __name__ == "__main__":
    main()
