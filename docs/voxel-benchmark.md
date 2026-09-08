# Voxel end-to-end benchmark

This is a target-blind compatibility benchmark. No domain, URL-pattern,
challenge-vendor, script-name, or parameter checks are permitted in runtime
code.

Last run: 2026-09-08.

| Runtime | Version | Observed progress |
|---|---|---|
| Chrome reference | required 152.0.7977.82 | exact-version differential oracle; the runner rejects version drift |
| BrowserOxide | 0.1.3 baseline | receives the managed challenge, then its challenge/CSP/resource path times out without returning a usable Page |
| Mimic | current workspace | Pyppeteer connects and `goto()` returns the navigation `Response`; cache control/interception work; the 403 document commits; challenge and Turnstile scripts execute; the first `/fo/` returns 200; Blob Worker bootstrap executes; Page remains usable after the challenge stalls |

Current controlled checkpoint (30-second clean-profile runs):

- exact Chrome 152: document response 403; challenge and Turnstile resources
  loaded; this run did not submit `/fo/` and did not obtain clearance;
- Mimic: document response 403; `/fo/` returned 200; no `cf_clearance` was
  issued; the returned branch created and bootstrapped a Blob Worker but did
  not post a program to it;
- the two outcomes cannot be treated as a deterministic semantic comparison,
  because the service selected different managed-challenge branches;
- `fetch` and XHR transport work is now concurrent and browser-observable
  completion returns through the scheduler's Network task source;
- Resource Timing is derived from real transport phases without per-version
  timing scale factors and entries are ordered by fetch start;
- the Chrome 152 transport uses the PSK-capable uTLS profile: a local TLS 1.3
  regression proves a cold handshake followed by cryptographic resumption
  (`DidResume == false`, then `true`) on a forced new connection;
- main-document CDP `requestId == loaderId`, so ordinary Pyppeteer now binds
  and returns the navigation response from `page.goto()`;
- no site- or challenge-vendor-specific behavior exists in runtime code.

## Reconstructed execution checkpoint

The current V8-default binary reproduced the historically established path in
repeated clean-profile `voxel.shop` runs:

```text
initial 403 document
-> orchestrate and widget scripts execute
-> first /fo/ POST
-> /fo/ response 200
-> Blob Worker bootstrap and script execution
-> child frame document response 200
-> distinct child realm reaches DOMContentLoaded and load
-> iframe owner load
-> bidirectional parent/child messages
-> bounded sequence-number heartbeat
```

The available response branch then remained on its fully loaded child page
(`readyState == complete`) with no JavaScript exception, cookie, follow-up
network request, or final navigation. Its repeated messages contained the same
plain-object heartbeat shape in both directions and transferred no
`MessagePort` objects.

A target-independent two-origin probe now reproduces that exact generic
message pattern: child ready message, parent configuration, three timer-driven
sequence exchanges, and explicit microtask observations. Chrome
152.0.7977.82 and Mimic matched all 33 observable steps, including
`MessageEvent.origin`, `source`, `ports`, realm-local `data` prototypes, and
task/microtask order. Consequently the current heartbeat is not evidence of a
lost message or scheduler regression.

Later clean-profile runs did select the comparable post-`/fo/` child branch in
both exact Chrome and Mimic. Passive snapshots were aligned by the ordinal of
each child-to-parent message and repeated twice per runtime. Messages 1 through
4 matched (`init`, `requestExtraParams`, `translationInit`, `food(seq=1)`). At
message 5 Chrome emitted `fail` and issued its next `/fo/` 10--16 ms later;
Mimic emitted `food(seq=2)` and continued the sequence heartbeat. This is the
first deterministic event divergence on the current path.

At the immediately preceding message-4 boundary, the first stable field in the
ordered passive child-state snapshot was `document.hasFocus()` (`false` in
Chrome, `null` in Mimic). Other stable differences included `activeElement`,
cross-origin `frameElement`, and the child viewport/layout (`1x1` in Chrome,
top-level `772x433` in Mimic). Mimic's API trace showed that these reads came
from the diagnostic snapshot itself, not from the challenge before message 5,
so none is yet established as the cause and no runtime change was selected from
them.

An owner-controlled external acceptance URL was also used historically. Its
private hostname/path have been removed from the publishable notes. That run did
not reach its expected final application response; a different path was not a
valid substitute. No external acceptance check was repeated during stabilization.

The smoke driver supports `--settle-ms` to let the CDP event loop progress in
real time and `--summary` to emit bounded structured results.
