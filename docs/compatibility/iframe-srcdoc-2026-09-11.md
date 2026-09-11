# Iframe srcdoc navigation, 2026-09-11

This package follows [selector bindings](selector-bindings-2026-09-11.md) and
the separate [native first-chance investigation](snapshot-firstchance-2026-09-11.md).
The semantic baseline is clean `ed3b7d6` (runtime `d50d3ec`). P0 remains open.

## Confirmed cause and correction

| ID | Root cause | Repro | Chrome | Before Mimic | Priority | Status | Fix | Limits |
|---|---|---|---|---|---|---|---|---|
| NAV-SRCDOC | Iframe navigation considered only src; srcdoc was reflection-only | iframe_srcdoc_oracle.js; original navigation_nested_window setup/oracle; state-relations-audit | Commits inline document at about:srcdoc, executes scripts, preserves WindowProxy | Fetches src or stays blank; nested export never exists | P1 | Implemented, validation below | frame.go / realm.go | Existing sandbox and retained-base-URL limits remain |

The inline document now enters the same navigation sequence, parser, script
execution, load dispatch and realm retirement path as fetched child documents.
Attribute contents are captured when scheduling; a later mutation supersedes
that navigation. Attribute presence matters, so empty srcdoc still commits an
empty document. Changing src while srcdoc exists leaves the current document
alone. Removing srcdoc starts navigation to src (or about:blank when absent).
There is no resource fetch for the inline document itself.

The new frozen Chrome oracle checks origin/base inheritance, script execution,
empty srcdoc, superseded writes, src shadowing, fallback navigation, stable
WindowProxy identity and retained document/node/function observations. It runs
in ordinary and restored snapshot realms. An additional test executes the
original nested-window setup and observation separately, in both V8 and goja,
so the ancestor is not accidentally pinned by one JavaScript evaluation.

## Fixed-set results

| Observation set | Before | After | Chrome→Chrome |
|---|---:|---:|---:|
| Original 236 probes, complete normalized match | 178 | 178 | 236 |
| Original 28 representatives, complete match | 12 | 12 | 28 |
| General corpus matches | 12/15 | 12/15 | 15/15 |
| General differing records | 41 | 41 | 0 |
| Brand matrix differing records | 1319 | 1319 | 0 |
| State-relations differing records | 4 | 3 | 0 |
| Original nested-window supplemental differing leaves | 5 | 0 | 0 |
| Standalone sweep matches | 113/130 | 113/130 | headful/headless mode comparison |

The fixed 236 observations have no newly differing paths, regressed matching
probes or Chrome drift. The previous official fuzzer's 17 observational groups
are unchanged because every fixed probe's observations remain unchanged; this
package does not claim a new discovery/minimization run. The six disappearing
records in two overlapping corpora are one confirmed cause, not six fixes.
The new focused oracle is supplemental evidence, not an addition to the frozen
236-probe denominator.

The fresh navigation replay still has two native globalThis-retargeting leaves
and three saved-eval call/apply/bind leaves. Cross-origin reflection and lifecycle
remain matched. HTMLAllCollection borrowed methods and live imported prototype
reflection account for the three remaining state-relations records.

The historical srcdoc status in navigation-ownership.md and navigation_realm_oracle.md
is superseded by this report; their original observations remain unchanged.
This correction does not establish native global-proxy retargeting, delegated
eval gates, precise bridge garbage collection, or reclamation of old DOM arena
nodes. Sandbox enforcement and retained document fallback-base behavior require
their own Chrome evidence and implementation work. Loopback results do not
establish any TLS/H2/H3 behavior.

## Validation receipts

- Full browser suite PASS, 246.284 s; webapi, DOM, network, scheduler, CDP and
  V8/goja/QuickJS engine suites PASS: [log](iframe-srcdoc-20260911/full-tests.txt).
- Affected navigation race PASS, 13.965 s. Expanded srcdoc tests, including the
  subsequently added two-evaluation descendant test, race PASS, 16.258 s:
  [log](iframe-srcdoc-20260911/srcdoc-race.txt). Full browser race was not run.
- Fresh fixed probes, representatives, navigation replay and all-leaf corpus
  comparisons/control: [results](iframe-srcdoc-20260911/results.json). The new
  focused Chrome oracle also passed an independent same-source control.
- Unchanged full fast gate PASS with first-chance native monitoring: all cold/
  warm DOM/static/React workloads, 10/25-Page concurrency, and static/React
  memory-after-teardown waves completed. No dump was produced. [Receipt](iframe-srcdoc-20260911/gate.json).
  The debugger affects timings; no speedup or native-crash repair is claimed.
- The initial focused test failed on the unmodified runtime, as expected;
  `.build/srcdoc-before.txt` retains that failure. Private captures also retain
  every differential command's nonzero divergence exit; those are not test passes.

Fresh tested binary SHA256:
`00edf13647ed3bae6892b58018e75a5a749b9fcd51f781112600574d5bb66f4f`.
The separately built monitored-gate executable has its own exact build identity
in the gate receipt. Both contain the same production source changes.
