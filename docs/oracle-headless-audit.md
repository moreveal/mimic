# Audit of previous Chrome 152 headless expectations

Audit date: 2026-09-08. The exact Chrome `152.0.7977.82` in a fresh
headful profile was compared with the same binary in a fresh headless profile.
The canonical source is now `compatibility/captures/navigator-chrome152.json`;
the mode-specific copy is `navigator-chrome152-headless.json`.

## Expectations changed because of their previous headless origin

| Observation | Previous headless expectation | Canonical headful expectation | Classification |
|---|---|---|---|
| Product token in UA/HTTP UA | `HeadlessChrome` | `Chrome` | mode (3) |
| `permissions.query(geolocation)` | `denied` | `prompt` | mode (3) |
| `permissions.query(camera)` | `denied` | `prompt` | mode (3) |
| `permissions.query(clipboard-read)` | `denied` | `prompt` | mode (3) |
| `permissions.query(keyboard-lock)` | `denied` | `prompt` | mode (3) |
| `getCurrentPosition` without a user decision | `User denied Geolocation` error | remains pending | mode (3) |
| `clipboard.readText` without a user decision | `NotAllowedError` | remains pending | mode (3) |
| `getUserMedia({video:true})` without a user decision | `NotAllowedError` | remains pending | mode (3) |
| `keyboard.lock()` without a user decision | `InvalidStateError` | remains pending | mode (3) |
| Geometry of the selected profile | screen 800×600, outer 780×580, viewport 772×433 | screen 2560×1440 (available 2560×1392), outer 1280×800, viewport 1272×653 | Environment (2) |
| WebGPU adapter discovery latency | 250 ms on the headless machine | 205 ms in the selected headful profile (five measurements: 203–226 ms) | Environment (2) |

Four exposure files (`window-secure`, `window-insecure`,
`window-secure-isolated`, `worker-secure`) were recaptured with headful Chrome.
Global properties, descriptors, and prototypes did not change between modes;
only capture provenance and the UA in the capture context changed. Correct
surface semantics therefore required no rewrite.

## Observations not promoted to general expectations

- `navigator.connection.downlink/rtt` varied between runs: category (5),
  machine/session state. The regression test does not compare them as general semantics.
- Bluetooth availability and completion of Serial enumeration varied between sessions:
  categories (2)/(5). They are excluded from the general equality test and remain
  data specific to the Environment/backend.
- Permissions and open permission UI are isolated by page. The intermediate
  `Document is not focused` result was classified as a tooling artifact (4) and
  was not included in any fixture.
- The Clipboard probe could previously save the text it read. It now records
  only the outcome (`pending`, an error, or `{resolved:true}`); contents are not saved.

All 17 historical BrowserScan differential files received explicit metadata with
mode `headless` and profile
`historical-chrome-152-windows-x64-headless-uncontrolled`. They remain historical
evidence of fixes and are not production input.
