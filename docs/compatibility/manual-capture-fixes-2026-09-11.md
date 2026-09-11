# Offline manual capture follow-up, 2026-09-11

Baseline: f3d032e. Source: ignored `manual-20260911-201641` capture.
The capture has no verifiable build revision or paired Chrome control; this is
not a measured site success or Chrome compatibility improvement.

## Confirmed implementation defects

- Bootstrap host-call recording serialized a returned object synchronously and
  could replace a successful call with a serialization exception. The capture
  stack points at that recorder. A synthetic throwing Proxy reproduces the
  failure without any site script. Recording errors now reject the optional
  seed at finalization while preserving live call results and restoring the
  original host. Cross-origin access checks remain unchanged. This is not a
  resolution of the historical native crash family.
- CDP response bodies were always converted to JSON strings, losing invalid
  UTF-8 bytes. Such bodies now use base64. A local HTTP fixture verifies exact
  byte preservation through the actual WebSocket CDP response, plus a text
  control. MIME/charset-based Chrome projection parity is not claimed.

## Remaining work

ICO decoding is still unsupported. The recorded unknown-format error is not
fixed by preserving CDP bytes. Implement a bounded decoder and establish frame
selection, dimensions, alpha and mask behavior against frozen Chrome before
claiming support. The original capture cannot recover bytes already replaced
by text serialization.

`structuredClone`, `Document.lang` and debugger-wait trace annotations are not
new confirmed defects. No target site or captured scripts were executed.
No Chrome control/general differential or performance experiment was run for
this internal recording and byte-preservation package. No performance claims.

## Validation

Focused bootstrap and CDP regression tests: PASS.
Targeted race for both regressions and existing private-intrinsic snapshot
restoration: PASS.
Full browser/CDP/image suites: PASS (285.414s / 2.197s / 0.402s).
This full run preceded the final failure-flag refinement described below.
The final refinement stores only a failure flag, avoiding retention of a thrown
object and correctly handling `throw null`. All bootstrap snapshot/capture
tests and the targeted browser race were rerun after that refinement.
