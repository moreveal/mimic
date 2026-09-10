# Manual trace comparison, 2026-09-10

Compared private captures `manual-20260910-165539` (Mimic) and
`iroshop-2026-09-09-chrome152` (Chrome). The latter's manifest identifies
Chrome **152.0.7977.82**. These are different executions, not deterministic
replays of identical server programs. No new live navigation was performed.

## Confirmed CDP differences

* Console arguments lose their types in Mimic. At trace sequence 1907,
  the third argument is `{type: "string", value: "NaN"}`; Chrome's
  `events.jsonl` line 303 has `{type: "number", unserializableValue: "NaN"}`.
  The numeric result is the same; its protocol representation is not.
  `internal/webapi/surface.js` converts every console argument to a diagnostic
  string before the CDP projection sees it.
* The same conversion replaces logged objects and functions with string
  placeholders. Mimic sequences 1887–1897 contain `[object]` and `[function]`;
  Chrome retains RegExp, Function and HTMLAnchorElement remote objects with
  object IDs. Mimic also omits `stackTrace` from console events.
  This loses inspector information, but is not evidence of different
  JavaScript coercion side effects or a cause of the page outcome.
* Network event timestamps use epoch seconds in Mimic, whereas Chrome exposes
  a monotonic clock separately from `wallTime`. The initial request has
  timestamp/wallTime `1789044954.770 / 1789044954` in Mimic and
  `106045.928257 / 1788963768.272303` in Chrome. Mimic additionally truncates
  wallTime to whole seconds. See `internal/cdp/server.go` network projections.
  Compare intervals within a capture, never absolute timestamps across them.
* DNS failures expose the Go/Windows transport error and request URL in Mimic's
  `Network.loadingFailed.errorText`; Chrome reports `net::ERR_NAME_NOT_RESOLVED`.
  Both requests target the same hostname. The failure itself is shared;
  the protocol error representation differs.

## Execution differences, without causal attribution

The interval between the second and third `/fo/` request is **11,266 ms** in
Mimic versus **3,310.235 ms** in Chrome (about 3.40 times longer). These are
timestamps from each capture's own request events. Request bodies and server
programs differ between attempts; this is not an isolated performance benchmark.

Mimic has a completed child timer task spanning trace sequences **2080–2497**,
lasting **6,521.386 ms** in host wall time. A child network task at **906–1694**
lasts **2,855.020 ms**. These durations locate long tasks, not the expensive
primitive; no CPU profile was captured in this investigation.

After four flow requests, Chrome performs a top-level POST and receives the
application's 404 document. Mimic instead performs another top-level GET and
receives another challenge 403. This is an outcome difference, not evidence
that the navigation primitive is wrong or that a POST should be synthesized.

## Unresolved lifecycle observation

Mimic emits two `load` lifecycle projections for one frame/loader at sequences
2447 and 2458, and again at 5573 and 5584. The first trace event names
`about:blank`; the second names an inherited document URL. Source inspection
finds separate initial-frame and document-stream completion paths. A
`document.open/write/close` sequence can generate additional lifecycle activity;
therefore this is **not yet classified as a bug**. The selected Chrome recorder
did not record Page lifecycle events, so absence there is not a difference.

## Excluded hypotheses and scope

Chrome also logs the styled `%c%d` NaN messages and encounters the same DNS
failure in a successful execution. Neither is evidence of a Mimic-only
failure. The Mimic recording contains no `Runtime.exceptionThrown` events;
caught, unlogged exceptions remain unknown. `semantic-missing` entries remain
low priority. No runtime changes were made on the basis of this comparison.
