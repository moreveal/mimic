"""Render README charts from the published numeric checkpoint; no measurements run."""
import argparse
import hashlib
import json
from pathlib import Path
from statistics import median

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.lines import Line2D
from matplotlib.ticker import MaxNLocator

BG = "#0d1724"
FG = "#eef4fc"
MUTED = "#a4b5c9"
GRID = "#27374a"
COLORS = {"mimic": "#8ce8cf", "chrome": "#78aafa"}
SYSTEMS = ("mimic", "chrome")


def load_results(path):
    data = json.loads(path.read_text(encoding="utf-8"))
    if not data.get("finished"):
        raise ValueError("Charts require a completed benchmark")
    if "metadata" in data:
        if data["metadata"]["arguments"].get("smoke"):
            raise ValueError("Smoke runs cannot produce presentation charts")
        summary_path = path.parent / "summary.json"
        manifest = json.loads((path.parent / "manifest.json").read_text(encoding="utf-8"))
        for source in (path, summary_path):
            if hashlib.sha256(source.read_bytes()).hexdigest() != manifest["sha256"][source.name]:
                raise ValueError(f"Benchmark artifact hash mismatch: {source.name}")
        calibration = data.get("startup_calibration", {})
        if not calibration.get("complete"):
            raise ValueError("Incomplete startup calibration")
        startup = {}
        for system in SYSTEMS:
            rows = [r for r in calibration["rows"] if r["system"] == system and not r["excluded"]]
            if len(rows) != 10:
                raise ValueError("Startup comparison requires ten measured processes per runtime")
            startup[system] = {
                "cdp_ready_ms": {"median": median(r["cdp_ready_ms"] for r in rows), "n": len(rows)},
                "rss_mib": {"median": median(r["ready_memory"]["rss"] / 2**20 for r in rows), "n": len(rows)},
            }
        summary = json.loads(summary_path.read_text(encoding="utf-8"))
        data = {"checkpoint": data["metadata"]["date"][:10], "startup": startup,
                "concurrency": summary["concurrency"]}
    completed_levels = None
    for system in SYSTEMS:
        if data["startup"][system]["cdp_ready_ms"]["n"] != 10:
            raise ValueError("Insufficient startup samples")
        rows = sorted([r for r in data["concurrency"] if r["system"] == system and r["workload"] == "static"], key=lambda r: r["n"])
        levels = {r["n"] for r in rows if not r["stop"] and r["success_rate"] == 1 and r.get("waves", 0) > 0}
        completed_levels = levels if completed_levels is None else completed_levels & levels
    completed_levels = sorted(completed_levels or ())
    if not completed_levels or completed_levels[0] != 1 or len(completed_levels) < 2:
        raise ValueError("Static scaling chart requires at least two common successful measured levels")
    data["completed_levels"] = completed_levels
    return data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("results", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--preview", type=Path)
    parser.add_argument("--receipt", type=Path)
    args = parser.parse_args()
    data = load_results(args.results)
    args.output.mkdir(parents=True, exist_ok=True)
    if args.preview:
        args.preview.mkdir(parents=True, exist_ok=True)
    plt.rcParams.update({
        "font.family": "DejaVu Sans", "font.size": 14,
        "figure.facecolor": BG, "axes.facecolor": BG, "savefig.facecolor": BG,
        "text.color": FG, "axes.labelcolor": MUTED, "xtick.color": MUTED,
        "ytick.color": FG, "axes.edgecolor": GRID, "svg.fonttype": "path",
        "svg.hashsalt": "mimic-readme-20260913", "axes.titleweight": "bold",
    })

    def frame(title, subtitle, height=4.4):
        fig = plt.figure(figsize=(12, height), dpi=120)
        fig.text(.045, .91, title, size=24, weight="bold")
        fig.text(.045, .84, subtitle, size=12, color=MUTED)
        fig.text(.045, .035, f"MIMIC  /  {data['checkpoint']}  /  Windows x64 · Chrome 152 · controlled local workloads",
                 size=10, color=MUTED)
        return fig

    def style(ax, horizontal=False):
        for spine in ax.spines.values():
            spine.set_visible(False)
        ax.tick_params(length=0, pad=9)
        ax.set_axisbelow(True)
        ax.grid(axis="x" if horizontal else "y", color=GRID, linewidth=.7)

    def legend(fig, y=.76):
        handles = [Line2D([0], [0], color=COLORS[s], lw=5, label=s.title()) for s in SYSTEMS]
        fig.legend(handles=handles, loc="center right", bbox_to_anchor=(.965, y),
                   ncol=2, frameon=False, fontsize=13, handlelength=1.4)

    def save(fig, name, description):
        # Catch the footer/axis-label collision at full and README-scale size.
        for dpi in (120, 80):
            fig.set_dpi(dpi)
            fig.canvas.draw()
            renderer = fig.canvas.get_renderer()
            for note in fig.texts:
                bounds = note.get_window_extent(renderer)
                for ax in fig.axes:
                    labels = [ax.xaxis.label, ax.yaxis.label, ax.title, *ax.get_xticklabels(), *ax.get_yticklabels()]
                    for label in labels:
                        if label.get_text() and bounds.overlaps(label.get_window_extent(renderer)):
                            raise ValueError(f"{name}: overlapping text: {note.get_text()} / {label.get_text()}")
        fig.set_dpi(120)
        fig.savefig(args.output / (name+".svg"), metadata={"Date": None, "Title": description,
                    "Description": description, "Creator": "Mimic benchmark presentation"})
        svg = args.output / (name+".svg")
        svg.write_text("\n".join(line.rstrip() for line in svg.read_text(encoding="utf-8").splitlines())+"\n",
                       encoding="utf-8", newline="\n")
        if args.preview:
            fig.savefig(args.preview / (name+".png"))
            fig.savefig(args.preview / (name+"-readme.png"), dpi=80)
        plt.close(fig)

    lighter = all(data["startup"]["mimic"][k]["median"] < data["startup"]["chrome"][k]["median"] for k in ("cdp_ready_ms", "rss_mib"))
    fig = frame("A lighter starting point" if lighter else "Startup and memory", "Process startup · 10 fresh processes per runtime · medians", 5.4)
    for rect, key, title, unit in [
        ([.12, .31, .34, .33], "cdp_ready_ms", "CDP readiness", "ms"),
        ([.61, .31, .34, .33], "rss_mib", "Memory at CDP readiness", "MiB RSS"),
    ]:
        ax = fig.add_axes(rect)
        values = [data["startup"][s][key]["median"] for s in SYSTEMS]
        ax.barh([1, 0], values, height=.42, color=[COLORS[s] for s in SYSTEMS])
        ax.set_yticks([1, 0], ["Mimic", "Chrome"])
        ax.set_xlim(0, max(values)*1.32)
        ax.xaxis.set_major_locator(MaxNLocator(4))
        ax.set_title(title, loc="left", pad=23, size=16)
        ax.set_xlabel(f"{unit} · lower is better", size=12, labelpad=10)
        for y, value in zip([1, 0], values):
            ax.text(value+max(values)*.025, y, f"{value:.2f}", va="center", size=15, weight="bold")
        style(ax, horizontal=True)
    fig.text(.045, .10, "Process-tree RSS includes the initial page. This is not per-page memory.", size=12, color=MUTED)
    save(fig, "benchmark-startup", "CDP readiness and process-tree RSS: medians of ten fresh processes per runtime, from the published checkpoint.")

    levels = data["completed_levels"]
    last_n = levels[-1]
    last = {s: next(r for r in data["concurrency"] if r["system"] == s and r["workload"] == "static" and r["n"] == last_n) for s in SYSTEMS}
    advantage = last["mimic"]["throughput"] > last["chrome"]["throughput"] and last["mimic"]["rss_mib"] < last["chrome"]["rss_mib"]
    fig = frame("More work. Less active memory." if advantage else "Concurrency: memory and throughput", f"Static workload · common completed levels through {last_n} concurrent pages", 6.25)
    legend(fig)
    for rect, metric, title, unit in [
        ([.09, .30, .36, .35], "rss_mib", "Active memory", "GiB RSS · lower is better"),
        ([.58, .30, .36, .35], "throughput", "Throughput", "successful sessions/s · higher is better"),
    ]:
        ax = fig.add_axes(rect)
        ymax = 0
        for s in SYSTEMS:
            rows = sorted([r for r in data["concurrency"] if r["system"] == s and r["workload"] == "static"], key=lambda r:r["n"])
            rows = [r for r in rows if r["n"] in levels and r["success_rate"] == 1 and not r["stop"] and r.get("waves", 0) > 0]
            assert [r["n"] for r in rows] == levels
            values = [r[metric]/1024 if metric == "rss_mib" else r[metric] for r in rows]
            ax.plot([r["n"] for r in rows], values, color=COLORS[s], lw=2.6, marker="o", ms=5)
            ax.annotate(f"{values[-1]:.2f}", (last_n, values[-1]), xytext=(8,0),
                        textcoords="offset points", va="center", color=COLORS[s], weight="bold", size=14)
            ymax = max(ymax, max(values))
        ax.set_xlim(0, last_n*1.17)
        ax.set_ylim(0, ymax*1.16)
        ax.set_xticks(levels)
        ax.set_xlabel("Concurrent pages", labelpad=9, size=13)
        ax.set_title(title, loc="left", pad=27, size=16)
        ax.text(0,1.015,unit,transform=ax.transAxes,color=MUTED,size=11)
        style(ax)
    fig.text(.045,.105,"Memory: median with pages retained. Throughput includes page setup and teardown.\nThe separate recovery wait is excluded.",size=12,color=MUTED)
    save(fig,"benchmark-scaling",f"Static workload: active process-tree RSS and successful sessions per second at common completed levels through {last_n} concurrent pages.")

    if args.receipt:
        artifacts = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in args.output.glob("benchmark-*.svg")}
        receipt = {"checkpoint": data["checkpoint"], "source_sha256": hashlib.sha256(args.results.read_bytes()).hexdigest(),
                   "generator_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(), "artifacts": artifacts}
        args.receipt.write_text(json.dumps(receipt, indent=2)+"\n", encoding="utf-8", newline="\n")


if __name__ == "__main__":
    main()
