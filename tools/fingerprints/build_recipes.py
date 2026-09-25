"""Build Windows GPU/font recipes from paired fingerprint records.

The input collection is intentionally external and is never copied into the repo.
Only modeled, jointly observed fields are exported. Canvas/WebGL pixel hashes,
browser identity, network observations and raw font inventories are excluded.
"""

import argparse
import hashlib
import json
import pathlib
import re


PINNED_SOURCES = {
    "intel-uhd-12lp": "9020owjejeed730979c2011.txt",
    "nvidia-rtx-4070-super": "2854tjmsqscu089436r9.txt",
    "amd-rdna2-integrated": "1153moevrymx369533r2000.txt",
    "intel-uhd-gen9": "3363tkfhqzlb817185j9.txt",
}


def compatible_candidate(source):
    match = re.search(r"Chrome/(\d+)", source.get("ua", ""))
    if not match or int(match.group(1)) < 140:
        return False
    if "Microsoft Windows" not in source.get("tags", []):
        return False
    gl = source.get("webgl_properties") or {}
    if "Direct3D11" not in gl.get("unmaskedRenderer", ""):
        return False
    gpu = source.get("webgpu") or {}
    high, low = gpu.get("highPerformance"), gpu.get("lowPerformance")
    if not high or not low:
        return False
    if (high.get("info") != low.get("info") or high.get("limits") != low.get("limits")
            or set(high.get("features", [])) != set(low.get("features", []))):
        return False
    if high["info"].get("vendor", "").lower() not in gl.get("unmaskedVendor", "").lower():
        return False
    return (high["limits"].get("maxTextureDimension2D") == gl.get("maxTextureSize")
            and gl.get("maxRenderBufferSize") == gl.get("maxTextureSize"))


def candidate_records(source_dir):
    pinned = {name: recipe_id for recipe_id, name in PINNED_SOURCES.items()}
    records = []
    for path in sorted(source_dir.glob("*.txt")):
        try:
            raw = path.read_bytes()
            source = json.loads(raw)
        except (OSError, ValueError):
            if path.name in pinned:
                raise
            continue
        if not compatible_candidate(source):
            if path.name in pinned:
                raise ValueError(f"pinned source is no longer compatible: {path.name}")
            continue
        digest = hashlib.sha256(raw).hexdigest()
        recipe_id = pinned.get(path.name, f"windows-d3d11-{digest[:12]}")
        records.append((recipe_id, digest, source))
    if len(records) < 40 or not set(PINNED_SOURCES).issubset({row[0] for row in records}):
        raise ValueError("expected at least 40 compatible records including pinned examples")
    return records


def camel(name):
    parts = name.lower().split("_")
    return parts[0] + "".join(
        part.upper() if part in ("2d", "3d") else part.title() for part in parts[1:]
    )


def constants(catalog, interface):
    entry = next(item for item in catalog if item.get("name") == interface)
    return {
        str(int(member["value"], 0)): member["name"]
        for member in entry["members"]
        if member.get("kind") == "constant" and isinstance(member.get("value"), str)
    }


def number(value):
    if isinstance(value, dict):
        return [number(value[str(index)]) for index in range(len(value))]
    if isinstance(value, list):
        return [number(item) for item in value]
    if isinstance(value, str):
        return float(value) if "." in value else int(value)
    return value


def webgl_capabilities(source, baseline, names):
    observed = source["webgl_properties"]
    result = json.loads(json.dumps(baseline))
    for kind in ("webgl", "webgl2"):
        target = result[kind]
        for code, entry in target["parameters"].items():
            const = names[kind].get(code) or names["webgl"].get(code)
            if not const:
                raise ValueError(f"unknown WebGL parameter {code}")
            field = camel(const).replace("Renderbuffer", "RenderBuffer")
            if kind == "webgl2" and field + "2" in observed:
                field += "2"
            if field not in observed:
                raise ValueError(f"missing WebGL observation {field}")
            entry["value"] = number(observed[field])
        target["anisotropy"] = number(observed["maxAnisotropy"])
        target["extensions"] = observed["extensions2" if kind == "webgl2" else "extensions"].split(",")
        for shader, shader_name in ((35632, "FragmentShader"), (35633, "VertexShader")):
            for precision, label in ((36336, "LowFloat"), (36337, "MediumFloat"),
                                     (36338, "HighFloat"), (36339, "LowInt"),
                                     (36340, "MediumInt"), (36341, "HighInt")):
                suffix = "2" if kind == "webgl2" else ""
                field = f"{shader_name}{label}{suffix}"
                target["precision"][f"{shader},{precision}"] = {
                    "rangeMin": number(observed[f"rangeMin{field}"]),
                    "rangeMax": number(observed[f"rangeMax{field}"]),
                    "precision": number(observed[f"precision{field}"]),
                }
    overrides = {}
    for kind in ("webgl", "webgl2"):
        current, original = result[kind], baseline[kind]
        overrides[kind] = {
            "parameters": {
                code: entry for code, entry in current["parameters"].items()
                if entry != original["parameters"][code]
            },
            "precision": {
                code: entry for code, entry in current["precision"].items()
                if entry != original["precision"][code]
            },
            "extensions": current["extensions"],
        }
        if current["anisotropy"] != original["anisotropy"]:
            overrides[kind]["anisotropy"] = current["anisotropy"]
    return overrides


def adapter(value):
    info = value["info"]
    return {
        "Vendor": info["vendor"],
        "Architecture": info["architecture"],
        "Device": info.get("device", ""),
        "Description": info.get("description", ""),
        "SubgroupMinSize": info.get("subgroupMinSize", 0),
        "SubgroupMaxSize": info.get("subgroupMaxSize", 0),
        "IsFallbackAdapter": info.get("isFallbackAdapter", False),
        "Features": value["features"],
        "Limits": {key: number(raw) for key, raw in sorted(value["limits"].items())},
    }


def build(source_dir, repo_dir):
    baseline = json.loads((repo_dir / "internal/state/webgl_capabilities.json").read_text())
    catalog = json.loads((repo_dir / "chrome/152/generated/surface-catalog.json").read_text())
    names = {
        "webgl": constants(catalog, "WebGLRenderingContext"),
        "webgl2": constants(catalog, "WebGL2RenderingContext"),
    }
    recipes = []
    for recipe_id, digest, source in candidate_records(source_dir):
        match = re.search(r"Chrome/(\d+)", source["ua"])
        gl = source["webgl_properties"]
        high = source["webgpu"]["highPerformance"]
        low = source["webgpu"]["lowPerformance"]
        if (not high or not low or high["info"] != low["info"]
                or high["limits"] != low["limits"]
                or set(high["features"]) != set(low["features"])):
            raise ValueError(f"ambiguous WebGPU adapter policy {digest}")
        gpu = adapter(high)
        if gpu["Vendor"].lower() not in gl["unmaskedVendor"].lower():
            raise ValueError(f"WebGL/WebGPU vendor disagreement {digest}")
        maximum = number(gl["maxTextureSize"])
        if gpu["Limits"]["maxTextureDimension2D"] != maximum:
            raise ValueError(f"WebGL/WebGPU texture-size disagreement {digest}")
        system = {}
        for name, font in sorted(source["systemfonts"].items()):
            if font["fontStyle"] != "normal" or font["fontWeight"] != "400":
                raise ValueError(f"unsupported system font style {digest}")
            system[name] = f'{font["fontSize"]} {font["fontFamily"]}'
        if "Arial" not in source["fonts"] or "Segoe UI" not in source["fonts"]:
            raise ValueError(f"missing system font resource {digest}")
        caps = webgl_capabilities(source, baseline, names)
        if number(gl["maxRenderBufferSize"]) != maximum:
            raise ValueError(f"WebGL texture-size disagreement {digest}")
        recipes.append({
            "ID": recipe_id,
            "SourceSHA256": digest,
            "SourceChromeMajor": int(match.group(1)),
            "Graphics": {
                "Vendor": gl["unmaskedVendor"],
                "Renderer": gl["unmaskedRenderer"],
                "MaxTextureSize": maximum,
                "WebGPU": gpu,
            },
            "WebGL": caps,
            "Fonts": {
                "SansSerif": "Arial",
                "SystemUI": "Segoe UI",
                "System": system,
            },
        })
    return sorted(recipes, key=lambda recipe: recipe["ID"])


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    repo = pathlib.Path(__file__).resolve().parents[2]
    args.output.write_text(json.dumps(build(args.source, repo), indent=2) + "\n")
