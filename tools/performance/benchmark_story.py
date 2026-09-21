"""Render the README benchmark story from a completed benchmark checkpoint.

The measured values come only from raw.json and summary.json. The checked-in
background is decorative and never participates in metric selection or scaling.
"""

import argparse
import hashlib
import json
from pathlib import Path
from statistics import median

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont
from matplotlib import font_manager


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_BACKGROUND = ROOT / "docs/assets/benchmark-mimic-adapts-background-20260921.png"
MIMIC = "#55a8ff"
MINT = "#62e6ce"
CHROME = "#8b97a8"
TEXT = "#f5f7fa"
MUTED = "#a7b1c2"
QUIET = "#737e8e"
LINE = "#27313b"
PANEL = "#090d11"


def load_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def validate_checkpoint(directory):
    raw_path = directory / "raw.json"
    summary_path = directory / "summary.json"
    manifest_path = directory / "manifest.json"
    raw, summary, manifest = map(load_json, (raw_path, summary_path, manifest_path))
    if not raw.get("finished"):
        raise ValueError("benchmark checkpoint is incomplete")
    if raw["metadata"]["arguments"].get("smoke"):
        raise ValueError("smoke runs cannot produce the README benchmark story")
    for path in (raw_path, summary_path):
        expected = manifest["sha256"].get(path.name)
        actual = hashlib.sha256(path.read_bytes()).hexdigest()
        if expected != actual:
            raise ValueError(f"benchmark artifact hash mismatch: {path.name}")
    calibration = raw.get("startup_calibration", {})
    if not calibration.get("complete"):
        raise ValueError("startup calibration is incomplete")
    return raw, summary


def startup(raw, system, field):
    rows = [r for r in raw["startup_calibration"]["rows"] if r["system"] == system and not r["excluded"]]
    if len(rows) != 10:
        raise ValueError(f"expected 10 startup samples for {system}")
    if field == "rss_mib":
        return median(r["ready_memory"]["rss"] / 2**20 for r in rows)
    return median(r[field] for r in rows)


def concurrency(summary, system, workload="static"):
    rows = [r for r in summary["concurrency"] if r["system"] == system and r["workload"] == workload]
    valid = [r for r in rows if not r["stop"] and r["success_rate"] == 1 and r.get("waves", 0) > 0]
    return {r["n"]: r for r in valid}


def fonts():
    regular = font_manager.findfont("DejaVu Sans")
    bold = font_manager.findfont(font_manager.FontProperties(family="DejaVu Sans", weight="bold"))
    mono = font_manager.findfont("DejaVu Sans Mono")
    return {
        "eyebrow": ImageFont.truetype(mono, 22),
        "title": ImageFont.truetype(bold, 66),
        "subtitle": ImageFont.truetype(regular, 27),
        "metric": ImageFont.truetype(bold, 54),
        "label": ImageFont.truetype(bold, 23),
        "small": ImageFont.truetype(regular, 19),
        "tiny": ImageFont.truetype(regular, 16),
        "tiny_bold": ImageFont.truetype(bold, 16),
    }


def rounded(draw, box, fill, outline=LINE, radius=22, width=2):
    draw.rounded_rectangle(box, radius=radius, fill=fill, outline=outline, width=width)


def text(draw, xy, value, font, fill=TEXT, anchor=None):
    draw.text(xy, value, font=font, fill=fill, anchor=anchor)


def comparison_bar(draw, x, y, width, mimic_value, chrome_value, lower, font_set, unit):
    maximum = max(mimic_value, chrome_value)
    for offset, label, value, color in ((0, "Mimic", mimic_value, MINT), (42, "Chrome", chrome_value, CHROME)):
        text(draw, (x, y + offset), label, font_set["tiny"], MUTED)
        bar_x = x + 92
        bar_width = max(8, int(width * value / maximum))
        draw.rounded_rectangle((bar_x, y + offset + 4, bar_x + width, y + offset + 21), 8, fill="#172029")
        draw.rounded_rectangle((bar_x, y + offset + 4, bar_x + bar_width, y + offset + 21), 8, fill=color)
        text(draw, (bar_x + width + 16, y + offset + 1), f"{value:.1f} {unit}", font_set["tiny_bold"], TEXT)
    direction = chrome_value / mimic_value if lower else mimic_value / chrome_value
    return direction


def render(checkpoint, output, background):
    raw, summary = validate_checkpoint(checkpoint)
    fs = fonts()
    bg = Image.open(background).convert("RGB")
    canvas = Image.new("RGB", (2000, 1125), "#050707")
    hero = bg.resize((2000, 667), Image.Resampling.LANCZOS)
    hero = ImageEnhance.Brightness(hero).enhance(0.62)
    hero = hero.filter(ImageFilter.GaussianBlur(0.35))
    canvas.paste(hero, (0, 0))
    overlay = Image.new("RGBA", canvas.size, (0, 0, 0, 0))
    od = ImageDraw.Draw(overlay)
    od.rectangle((0, 0, 2000, 1125), fill=(3, 7, 10, 28))
    od.rectangle((0, 520, 2000, 1125), fill=(5, 7, 8, 244))
    canvas = Image.alpha_composite(canvas.convert("RGBA"), overlay).convert("RGB")
    draw = ImageDraw.Draw(canvas)

    date = raw["metadata"]["date"][:10]
    text(draw, (90, 62), f"MIMIC  /  VERIFIED CHECKPOINT  /  {date}", fs["eyebrow"], MINT)
    text(draw, (90, 112), "Same browser surface.", fs["title"])
    text(draw, (90, 184), "Far less machinery.", fs["title"], MIMIC)
    text(draw, (92, 270), "Fresh local benchmark · Chrome 152 · identical fixtures and correctness gates", fs["subtitle"], MUTED)

    mimic_ready = startup(raw, "mimic", "cdp_ready_ms")
    chrome_ready = startup(raw, "chrome", "cdp_ready_ms")
    mimic_rss = startup(raw, "mimic", "rss_mib")
    chrome_rss = startup(raw, "chrome", "rss_mib")
    mc, cc = concurrency(summary, "mimic"), concurrency(summary, "chrome")
    levels = sorted(set(mc) & set(cc))
    if not levels:
        raise ValueError("no common successful static concurrency level")
    level = max(n for n in levels if n <= 50) if any(n <= 50 for n in levels) else max(levels)
    mrow, crow = mc[level], cc[level]

    cards = [(90, 570, 650, 945), (675, 570, 1235, 945), (1260, 570, 1910, 945)]
    for box in cards:
        rounded(draw, box, PANEL)

    text(draw, (125, 608), "01  START LIGHT", fs["eyebrow"], MIMIC)
    text(draw, (125, 657), f"{chrome_rss / mimic_rss:.1f}×", fs["metric"], MINT)
    text(draw, (125, 719), "less ready RSS", fs["label"])
    comparison_bar(draw, 125, 775, 260, mimic_rss, chrome_rss, True, fs, "MiB")
    text(draw, (125, 885), f"CDP ready: {mimic_ready:.0f} vs {chrome_ready:.0f} ms", fs["small"], QUIET)

    text(draw, (710, 608), f"02  SCALE TO {level} PAGES", fs["eyebrow"], MIMIC)
    text(draw, (710, 657), f"{mrow['throughput'] / crow['throughput']:.1f}×", fs["metric"], MINT)
    text(draw, (710, 719), "static throughput", fs["label"])
    comparison_bar(draw, 710, 775, 230, mrow["throughput"], crow["throughput"], False, fs, "pages/s")
    text(draw, (710, 885), f"Active RSS: {mrow['rss_mib'] / 1024:.2f} vs {crow['rss_mib'] / 1024:.2f} GiB", fs["small"], QUIET)

    text(draw, (1295, 608), "03  KEEP IT LIGHT", fs["eyebrow"], MIMIC)
    text(draw, (1295, 657), f"{crow['rss_mib'] / mrow['rss_mib']:.1f}×", fs["metric"], MINT)
    text(draw, (1295, 719), "less active RSS", fs["label"])
    comparison_bar(draw, 1295, 775, 270, mrow["rss_mib"] / 1024, crow["rss_mib"] / 1024, True, fs, "GiB")
    text(draw, (1295, 885), f"Measured with {level} live static Pages", fs["small"], QUIET)

    draw.line((90, 1000, 1910, 1000), fill=LINE, width=2)
    footer = f"10 fresh starts · 20 warm samples/workload · {level}-Page static waves · process-tree RSS · lower memory / higher throughput is better"
    text(draw, (90, 1028), footer, fs["tiny"], MUTED)
    text(draw, (90, 1063), "Controlled local fixtures; not a claim of universal compatibility or website performance.", fs["tiny"], QUIET)
    text(draw, (1910, 1063), "REPRODUCIBLE FROM REPOSITORY DATA", fs["tiny_bold"], MIMIC, anchor="ra")

    output.parent.mkdir(parents=True, exist_ok=True)
    canvas.save(output, optimize=True)
    receipt = {
        "checkpoint": date,
        "raw_sha256": hashlib.sha256((checkpoint / "raw.json").read_bytes()).hexdigest(),
        "summary_sha256": hashlib.sha256((checkpoint / "summary.json").read_bytes()).hexdigest(),
        "background_sha256": hashlib.sha256(background.read_bytes()).hexdigest(),
        "generator_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        "output_sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
        "concurrency_level": level,
    }
    output.with_suffix(".receipt.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8", newline="\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("checkpoint", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--background", type=Path, default=DEFAULT_BACKGROUND)
    args = parser.parse_args()
    render(args.checkpoint, args.output, args.background)


if __name__ == "__main__":
    main()
