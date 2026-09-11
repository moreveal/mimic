# Platform bindings and bootstrap package

The fresh baseline is **6bfa245**, on the cleanup branch, with an independent
baseline checkout. Production changes are **70a5335, d549280, b5a4028, fea1c82**.
**9e65946** records the new artifact's canonical LF byte hash; its parsed frozen
observations are unchanged.
The earlier deferred-child implementation remains enabled; **44b1df1** makes
the snapshot warmup test actually observe its deferred realm. All changes use
signed conventional commits. This is a completed semantic package, with
incomplete performance qualification, not completion of the broader cleanup.

Only local fixtures ran against frozen Chrome **152.0.7977.82**, Windows x64,
headful, controlled reused profile, no feature overrides, viewport 1272×653.
Profile freshness is explicitly `unverified-reused-controlled`. No target site
ran. Source state, executable SHA-256, probe hashes, full before/after values,
all-path transitions and validation receipts are in
[platform-bindings-20260911](platform-bindings-20260911/). Existing reports retain
their original revision scope.

| ID | Root cause | Repro | Chrome | Mimic before | Priority | Status / commit | Limits |
| --- | --- | --- | --- | --- | --- | --- | --- |
| PLATFORM-COLLECTION | Indexed proxy traps bypassed prototype methods and private receiver state | `collection_binding_oracle.js` | Canonical methods, receiver/arity/conversion checks, live intrinsic iteration and owner dispatch | Fresh closures, stub borrowed methods, missed shadowing and generic iteration | P1 | Corrected / d549280 | Form named-item overloads, RadioNodeList and all indexed define/delete cases are not claimed |
| PLATFORM-SINGLETON | Navigator, Screen and History getters closed over the method's realm | `singleton_accessor_oracle.js` | Reject forged/proxy receivers; borrowed getters read the receiver's owner | Forged receivers accepted; foreign receiver reads local state or throws incorrectly | P1 | Corrected / b5a4028 | History methods and broader interface inheritance remain separate |
| PLATFORM-ORDER | Alphabetical descriptor captures were used without actual member publication order | `window_member_order_oracle.js` | Observed string-key order; user reinsertion subsequently changes it | Constructor/member installation order exposed instead | P2 | Corrected / fea1c82 | Four prototypes and AbortSignal static members in secure Windows; incompatible nonconfigurable prefixes are preserved |
| BOOTSTRAP-METADATA | Bootstrap repeatedly redefined unchanged descriptors and allocated full native source strings | Existing native-function oracle, shared corpus and startup profile | Function shape and mutation behavior preserved | Repeated unnecessary allocation/publication | P2 | Implemented / 70a5335 | Full performance gate remains incomplete below |

Focused differing observations change **164→0**, **214→0**, **121→0** for
collections, singleton accessors and member order respectively. Each short
Chrome control has zero differences. These are observation counts, not numbers
of independent defects. The collection implementation shares one NodeList view
for host queries, childNodes and fragment queries, uses intrinsic Array methods,
and registers derived form collections with the base interface's owner binding.
The new publication data reorder existing descriptors once; they do not alter
reflection output dynamically or add capabilities.

The initial singleton diagnostic stopped while trying to transfer a parent
object into child history, then on an uncaught pre-fix borrowed getter failure.
The final fixture creates state in its owner realm and records expected getter
exceptions explicitly. The original failing source is retained privately. A
cross-realm History state-clone issue remains a separate candidate, not a claimed
fix or a fully minimized current-tree root cause.

| Fixed input set | Before | After | Differing observation records | Observational groups |
| --- | --- | --- | --- | --- |
| 236 fixed probes | 205 matches | 210 matches | 209→88 | 9→4 |
| 28 original representatives | 19 matches | 24 matches | 131→10 | 9→4 |
| Brand matrix | 555 differing records | 495 differing records | 60 removed | Not a root-cause count |
| General corpus, 15 cases | 40 differing records | 40 differing records | Unchanged | Not added to other sets |
| State relations | 2 differing records | 2 differing records | Unchanged | HTMLDDA boundary remains |

All normalized paths were compared, including changed values on paths that were
already different. An additional comparison walks unequal-length arrays rather
than stopping at their container. No new differing path, previously matching
probe regression, invalid case, incomplete case or Chrome observation drift was
found. Group keys use the fuzzer's exact category/path/expected/actual signature;
the first differing path is used only for observational grouping. These groups
are not proven root causes. The corpora overlap and are not summed.

The full 15-case Chrome→Chrome general control has zero differences and no
invalid/incomplete cases. New focused discovery remains separate from the fixed
236 probes and 28 representatives. A new broad discovery/standalone sweep was
not run in this package.

Final validation ran once for the integrated package:

- Focused ordinary/restored-snapshot browser checks pass, 6.112 s; transport
  checks pass, 0.110 s.
- Full browser suite and WebAPI, V8, Goja, CDP, network, scheduler and Chrome
  bundle tests pass. The command completes in 262.916 s; browser test output
  reports 257.670 s. There are 1360 pass events across tests and subtests.
- Targeted browser race passes, 49.498 s, 80 pass events, including the six
  previously failing Goja deadline scenarios. This is not a full browser race.
  Full WebAPI/V8/Goja/CDP/network/scheduler race packages pass, with no race
  warning. The concurrent numeric snapshot regression is included.
- Python harness tests pass: 23 tests, six opt-in Chrome tests skipped. An
  initial invocation had three import errors from the wrong module search path;
  corrected discovery passes. Both outcomes are recorded.
- Opt-in lifecycle/profile tests, native frame microtasks in Goja and packages
  without tests remain explicit skips in the receipt. Snapshots and the native
  iterator-result cache remain enabled; no deadline or expectation was weakened.

This also completes ordinary-suite and targeted-race integration validation of
the previously interrupted iteration/encoding package: c15286e keeps
Headers/URLSearchParams iterators live over private state; 42fd5ff shares the
replacement UTF-8 decoder with form parsing; 4ad0caa validates TextEncoder
arguments against native typed-array state. 6bfa245 covers the same operations
in actual Blob Workers. Their checked-in frozen oracles all pass here. Their
previous focused counts are not added to this fresh baseline's improvement.

Performance remains qualified. Two matched Goja five-Page comparisons measured
cold startup 499.532→482.715 ms and, after releasing our idle test servers,
540.095→520.743 ms (about 3–4% lower). Warm startup in the latter pair is
389.010→388.170 ms. Ordinary close-plus-250-ms private memory is
475.13→427.08 MiB in that pair; the earlier pressure-affected pair moved in the
opposite direction, 443.65→463.31 MiB. Diagnostic post-GC live heap in the latter
pair is 21.78→21.83 MiB. Forced GC is diagnostic only; it is not a recovery policy.

Both initial fast gates stop at the host-memory guard before their first
concurrent workload executes. After releasing only our owned Mimic and oracle
processes, both unchanged full gates pass all six mandatory workloads, all
10-Page waves and the 25-Page warmup's 25 operations. Both still stop at the
15%-available-memory guard during that warmup. The measured 25-Page waves and
final dedicated memory waves therefore do **not** execute. No native dump or
unexpected process exit occurs. These are failed/incomplete gates, not passes.

In the latter pair, median DOM/static/React completion is
517.87/27.74/90.03→518.84/25.97/94.67 ms. Median measured 10-Page throughput is
74.19→73.16 Pages/s. Thus this package does not establish a V8 throughput or
blanket performance improvement. The full gate requires sufficient host RAM to
stay above its unchanged guard throughout the load. Exact recovery samples,
process status, failed attempts and build identities are preserved in
[performance.json](platform-bindings-20260911/performance.json).

Remaining work is explicit:

| ID | Classification | Evidence / remaining requirement |
| --- | --- | --- |
| NAMED-ACCESS | Confirmed semantic defect | `legacy.window-named` still lacks the observed WindowProperties/Document named projection |
| DOCUMENT-FOCUS | Incomplete implementation | 24 fixed probes expose missing hasFocus behavior/receiver checks; a real focus/lifecycle state model is required |
| WINDOW-SURFACE | Candidate composite observation | `surface.window` still differs; no single root cause is inferred from this group |
| HISTORY-CROSS-CLONE | Candidate from incomplete diagnostic | Minimize the preserved child-history argument case against the final tree before changing serialization |
| NATIVE-REALM | Architecture limited | Retained native closures/global-proxy retargeting, saved-eval call/apply/bind access gates and strong bridge/DOM-arena lifetime remain separate boundaries |
| ENVIRONMENT | Environment dependent or unmeasured | Codec/device/GPU observations and TLS/H2/H3 are not established by loopback HTTP/1.1 results |
| SNAPSHOT-P0 | Open broader failure family | The separate [numeric allocator evidence](snapshot-numeric-allocator-2026-09-11.md) supports its specific fix; it does not prove all native failure signatures have one cause |
| PERFORMANCE-GATE | Incomplete environment-constrained validation | Complete the unchanged 25-Page measured and teardown-memory workloads when the host can maintain its RAM guard |

No maximal-compatibility claim or explanation of a target site's response is
derived from these probe counts.
