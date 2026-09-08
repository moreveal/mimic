"""Shared metadata and validation for Chrome compatibility captures."""

from __future__ import annotations

import json
import urllib.request


PINNED_PRODUCT = "Chrome/152.0.7977.82"
PINNED_VERSION = "152.0.7977.82"
CHROMIUM_REVISION = 1669021
CHROMIUM_COMMIT = "d04cdb24d67b081f6cf80200ffc5233f44b61109"
PLATFORM = "windows-x64"
MODES = ("headful", "headless")


def endpoint_version(endpoint: str) -> dict:
    with urllib.request.urlopen(endpoint.rstrip("/") + "/json/version") as response:
        return json.load(response)


def product(endpoint: str) -> str:
    return endpoint_version(endpoint).get("Browser", "")


def default_profile_id(mode: str) -> str:
    return f"chrome-152-windows-x64-{mode}-controlled-v1"


def parse_size(value: str) -> tuple[int, int]:
    try:
        width, height = (int(part) for part in value.lower().split("x", 1))
    except (TypeError, ValueError) as error:
        raise ValueError(f"invalid size {value!r}; expected WIDTHxHEIGHT") from error
    if width <= 0 or height <= 0:
        raise ValueError("window dimensions must be positive")
    return width, height


async def prepare_page(browser, page, mode: str, window_size: str) -> None:
    """Apply the pinned geometry and foreground the page without a user gesture."""
    if mode not in MODES:
        raise ValueError(f"unsupported browser mode: {mode}")
    width, height = parse_size(window_size)
    target_id = page._target._targetId
    try:
        window = await browser._connection.send(
            "Browser.getWindowForTarget", {"targetId": target_id}
        )
        await browser._connection.send(
            "Browser.setWindowBounds",
            {
                "windowId": window["windowId"],
                "bounds": {"left": 0, "top": 0, "width": width, "height": height},
            },
        )
    except Exception:
        # Some headless implementations have no native browser window. The
        # observed viewport is still recorded below and must match the caller's
        # explicitly configured headless viewport.
        if mode == "headful":
            raise
    if mode == "headful":
        await page.bringToFront()


def _feature_overrides(arguments: list[str], declared: list[str]) -> list[str]:
    result = list(declared)
    for argument in arguments:
        if argument.startswith("--enable-features=") or argument.startswith("--disable-features="):
            result.append(argument)
    return sorted(set(result))


async def capture_metadata(
    endpoint: str,
    browser,
    page,
    *,
    mode: str,
    environment_profile_id: str,
    declared_feature_overrides: list[str] | None = None,
    context_states: dict | None = None,
) -> dict:
    """Return mandatory provenance for an oracle observation."""
    version = endpoint_version(endpoint)
    observed = version.get("Browser", "")
    if observed != PINNED_PRODUCT:
        raise RuntimeError(f"Chrome oracle drift: expected {PINNED_PRODUCT}, got {observed!r}")
    user_agent = version.get("User-Agent", "")
    detected_mode = "headless" if "HeadlessChrome/" in user_agent else "headful"
    if detected_mode != mode:
        raise RuntimeError(f"oracle mode mismatch: declared {mode}, observed {detected_mode}")

    command_line: list[str] = []
    try:
        command_line = (await browser._connection.send("Browser.getBrowserCommandLine")).get(
            "arguments", []
        )
    except Exception:
        pass
    target_id = page._target._targetId
    bounds = {}
    try:
        window = await browser._connection.send(
            "Browser.getWindowForTarget", {"targetId": target_id}
        )
        bounds = (
            await browser._connection.send(
                "Browser.getWindowBounds", {"windowId": window["windowId"]}
            )
        ).get("bounds", {})
    except Exception:
        pass
    context = await page.evaluate(
        """() => ({
          secureContext: isSecureContext,
          crossOriginIsolated,
          viewport: {width: innerWidth, height: innerHeight, deviceScaleFactor: devicePixelRatio},
          window: {x: screenX, y: screenY, outerWidth, outerHeight},
          platform: navigator.platform,
          userAgent: navigator.userAgent,
          visibilityState: document.visibilityState,
          hasFocus: document.hasFocus()
        })"""
    )
    secure_state = context["secureContext"]
    isolation_state = context["crossOriginIsolated"]
    if context_states is not None:
        secure_state = {name: value["secureContext"] for name, value in context_states.items()}
        isolation_state = {
            name: value["crossOriginIsolated"] for name, value in context_states.items()
        }
    return {
        "schemaVersion": 1,
        "chromeVersion": PINNED_VERSION,
        "chromiumRevision": CHROMIUM_REVISION,
        "chromiumCommit": CHROMIUM_COMMIT,
        "v8Version": version.get("V8-Version", ""),
        "platform": PLATFORM,
        "browserMode": mode,
        "commandLineFeatureOverrides": _feature_overrides(
            command_line, declared_feature_overrides or []
        ),
        "viewport": context["viewport"],
        "window": bounds or context["window"],
        "secureContextState": secure_state,
        "isolationState": isolation_state,
        "environmentProfileId": environment_profile_id,
        "profileFreshness": "fresh-controlled",
        "visibilityState": context["visibilityState"],
        "hasFocus": context["hasFocus"],
    }
