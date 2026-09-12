# Metadata producer closure, manual-20260912-045102

This is private-capture localization, not a new generic browser oracle. Literal
field names identify the investigated records and never enter production code.

The six upstream values are copied from the parent API's `requestExtraParams`
message into `_cf_chl_opt`, with `|| 0`, and then into the payload. The parent
API defines `Q()` as `Date.now()`. Exact expressions are:

| Field | Upstream expression |
| --- | --- |
| tZwbF3 | `Q() - y.turnstileLoadInitTimeTsMs` |
| wOvYJ5 | `Q() - o.widgetRenderStartTimeTsMs` |
| eaaP6 | `o.widgetInitStartTimeTsMs - o.widgetRenderEndTimeTsMs` |
| Blsob5 | `o.widgetRenderEndTimeTsMs - o.widgetRenderStartTimeTsMs` |
| poqG1 | `o.widgetParamsStartTimeTsMs - o.widgetInitStartTimeTsMs` |
| uGyjw9 | `Q() - Gt` |

The load-init timestamp is assigned by `Q()` during API state initialization.
Render timestamps bracket widget construction/insertion. The init timestamp is
assigned on the widget `init` message; the params timestamp is assigned on
`requestExtraParams`. `Gt=Q()` starts that handler's extra-parameter collection.
These are clock intervals, not API counts or capability bits.

`twvE0` is `_cf_chl_opt.BHwY7 - _cf_chl_opt.CXYBD4`; both endpoints are assigned
`Date.now()` in the child's bootstrap/run path. The original native scan shows
1789177044981 - 1789177044391 = 590 at payload write IP61871.

`myWtu3` is also an elapsed clock interval. A focused handler-step diagnostic
shows the `Date.now()` call at IP36536 yielding register127=1789190512003;
handlerST at36537 loads saved local timestamp1789190511692 into register124;
handleruj at36540 subtracts, yielding register126=311; handleruk at36544 writes
myWtu3=311 (next instruction36548). This proves its arithmetic rather than
classifying it from numerical variability. The symbolic name of the saved local
start marker is not retained by the VM and is not needed to classify the interval.

`KDQCx4` is the child watchdog's **overrun-begin count**, not a server verdict.
A1000ms interval checks `Date.now()-OE > (_cf_chl_opt.bKOG4 || 10000)` while
`!uUOrc2 && !OS() && !MxFzg4.uqoZ1`. On expiry it calls `h()`. That routine
returns when the relevant kill-switch `H('xOdFQ6')` applies or its `Q` latch is
already set; otherwise it initializes the counter to0 if absent, increments it,
sets the latch, and sends a parent message with `event:'overrunBegin'`.
Thus different executions can legitimately produce0/1 when they cross the
watchdog deadline. Timer correctness still requires independent deadline checks;
this localization does not license changing time or suppressing watchdogs.

Evidence is retained under `.build/residual-media-final-delegated`:
`metadata-deob.html`, `metadata-step.cjs`, and `metadata-step/receipt.json` plus
the full diagnostic traces. Original source records are
`root-217c00a4-847e-40b5-b2e0-7ccea2b91e79.json` (child HTML/string table) and
`root-20fe0bfd-220d-4999-af49-ee6ae61167a6.json` (parent API), within the named
capture. Static string-table rotation resolves the child's property references;
no captured response or VM program was edited in place. No runtime changes or
new compatibility expectations are made by this report.
