# Chrome 152 oracle policy

Mimic targets observable behavior of normal desktop Chrome, not incidental
automation or headless behavior. The primary compatibility oracle is:

```text
Chrome 152.0.7977.82
Chromium r1669021 / d04cdb24d67b081f6cf80200ffc5233f44b61109
V8 15.2.124.21
Windows x64
headful
fresh controlled profile
chrome-152-windows-x64-headful-controlled-v1
1280x800 outer window (observed viewport is recorded)
no command-line feature overrides
```

## Authority and provenance

Headful Chrome is authoritative whenever an observation can depend on window or
display state, focus, visibility, geometry, graphics, media/devices, activation,
permissions, rendering, browser UI integration, automation exposure, or
Navigator/platform capabilities. Headless Chrome is a non-authoritative, explicit
regression source. It may be used only after the exact probe is shown invariant,
or when the expected result comes from standards/IDL and cannot depend on
presentation mode. When uncertain, use `compatibility/oracle_differential.py`.

Every retained Chrome observation has `captureMetadata` containing the exact
Chrome, Chromium and V8 versions, platform, browser mode, feature overrides,
observed viewport/window, secure/isolation state and Environment profile ID.
Origins are never erased when observations are compared. Historical captures
whose full launch configuration cannot be reconstructed use an explicit
`historical-...-uncontrolled` profile ID and cannot become generic expectations.

## Differential classification

Before changing compatibility behavior, classify a Chrome ↔ Mimic difference:

1. Chrome-version semantic;
2. selected Environment-profile state;
3. headful/headless-mode state;
4. automation/tooling artifact;
5. nondeterministic machine/session state.

Only (1), and deliberately selected (2), may feed generic compatibility behavior.
A value differing between headful and headless must remain attached to its mode.
The three-way tool emits `invariant`, `headless-specific`,
`environment-specific`, or `Mimic divergence`; its rows retain all three values
and both Chrome capture metadata blocks.

## Capture workflow

Start the exact Chrome binary twice with separate fresh profiles and pinned launch
configuration, then run:

```powershell
python compatibility/oracle_differential.py `
  --headful http://127.0.0.1:9333 `
  --headless http://127.0.0.1:9444 `
  --mimic http://127.0.0.1:19222 `
  --probes compatibility/probes.json
```

Exposure and Navigator tools default to `--browser-mode headful`; headless must be
requested explicitly. Navigator operations run in separate pages with permissions
reset between probes, so permission UI, focus and previous operations cannot
contaminate later observations. Clipboard probes never persist clipboard content.

Use `python tools/check_oracle_captures.py` to reject retained Chrome captures
without the required provenance. Tests select the authoritative headful source by
default; `chrome152.NewForMode(state.BrowserModeHeadless)` and the explicit
headless capture exist only for mode-scoped regression work.

The environment model selects a coherent presentation profile. It does not apply
property-by-property “not headless” patches: Product/UA, display/window, graphics,
permissions and capability behavior all derive from the selected headful or
headless Environment.

