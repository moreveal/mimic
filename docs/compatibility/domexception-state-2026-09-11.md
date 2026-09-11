# DOMException private state and shared Window/Worker codes

This package follows bda67db and is validated together with the
[Node name correction](node-name-2026-09-11.md). Production source starts at
cee1376 (a documentation-only successor). No historical report is overwritten.

| ID | Root cause | Repro | Chrome | Mimic before | Priority | Status | Fix | Limits |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| EXCEPTION-STATE | code reads public name; Window-only code table uses inherited object properties | domexception_state_oracle.js / domexception_worker_oracle.js | Private exception name determines a numeric legacy code in both environments | Public shadowing changes code; invalid receivers succeed; most worker codes are zero; constructor/toString/__proto__ names escape the table | P1 | Focused corrected | One shared code table, private slots and own-key lookup | Other exception interfaces are separate |
| EXCEPTION-CONVERSION | Constructor converts message twice and accepts Symbols | Same probes | One message conversion followed by one name conversion; Symbols throw | Duplicate side effects and accepted Symbols | P1 | Focused corrected | Convert each argument once before Error allocation | Arbitrary newTarget/prototype-access ordering remains unverified |
| EXCEPTION-OWN | Error construction publishes extra own stack/message | Same probes | No own string properties on these DOMExceptions | Own stack and message | P1 | Focused corrected | Retain native Error classification, remove stack, expose message through private slots | Native tag boundary below remains |
| HOST-RECORD | Go-to-V8 record conversion assigns through prototype setters | Worker probe / TestHostRecordsDefineOwnDataProperties | __proto__ is an own data property; inherited setters do not run | Worker result loses __proto__; arbitrary inherited setters/readonly properties intercept fields | P1 | Focused corrected | CreateDataProperty in ordinary and recursive callback map conversion | This is not complete structured-clone support or a fix for Go map property order |

Window focused differences improve **10 to zero** and Worker differences
**28 to zero**, each with zero Chrome control differences. These overlap and
must not be added as independent bugs. The Worker probe uses actual local Blob
workers in both browsers. It also exposed HOST-RECORD, which was fixed in the
general engine conversion instead of altering the probe or filtering the key.
The host-record regression fails on bda67db and passes on the changed source;
it covers direct records, JSON callback records and non-finite-number recursive
callback records with inherited setters and readonly properties.

An intermediate constructor change removed the stack before registering private
slots. Goja's lazy stack processing then invoked the name getter too early:
existing CharacterData error and fetch-abort tests failed. Registering state
before stack removal fixes both. The combined focused browser checks, including
Node/Attr, exception ordinary/snapshot/worker oracles and existing fetch-abort
and CharacterData cases, pass (3.817 seconds). Host-record focused engine test
passes (0.188 seconds). Fixed replay retains 205/236 matches and 19/28 original
representatives, with zero changed normalized observation leaves and zero
Chrome drift. Brand-corpus differences decrease from 558 to 556; the general
corpus remains 40 and state relations remain two. No differing records were
added or changed; two brand records were removed. These overlapping corpora are
not summed. The completed package validation is recorded below.

## Joint package validation

The three production commits are dd64c8f (Node names), 239eb58 (host records)
and 660713a (DOMException state). The full browser suite passes (450.650 s),
as do WebAPI, V8, CDP, network and scheduler packages. Targeted browser race
passes (16.437 s); full V8 and WebAPI race passes (3.992/2.633 s). This is not
a claim of full browser race success: the separate bootstrap experiment's
full browser race has six Goja deadline failures and no reported data race.

Fixed replay has 31 divergent probes, nine observational groups and 209
differing leaves, unchanged by this package; these are not root-cause counts.
The complete fast gate passes with 10/25 concurrent Pages and no native dump.
Against bda67db, median throughput is 69.42 to 71.01 Pages/s at 10 Pages and
71.48 to 69.67 at 25. Static/React recovery private memory is 159.73/170.54 to
170.93/163.82 MiB. Median completion latency rises from 29.32/81.96/485.39 to
30.61/90.44/498.50 ms for static/React/DOM. This single sequential pair has
mixed results; performance neutrality and snapshot P0 closure are not proven.
The unchanged full workloads were used, including their original warm-up rules.
Private raw logs are under cleanup-validation-20260911 and
node-exception-gate-after-20260911; public compact receipts accompany this report.

## Remaining native tag boundary

The supplemental probe includes Error.isError and Object.prototype.toString
after removing an exception's prototype. It improves **11 leaves to one**,
with zero Chrome control differences. Both engines still report Error.isError
true, but Chrome's tag becomes `[object Object]` while Mimic's native Error
backing produces `[object Error]`. Replacing the backing object with a plain
object would lose the native Error classification. This retained boundary is
not counted as fixed; native DOMException branding or an equally coherent
cross-realm solution is still needed. The complete supplemental observation is
preserved alongside the focused regression, not filtered from the differential.

[Evidence](node-exception-20260911/) contains before/after/control leaves, the
supplemental repro, binary hashes, probe hashes and Chrome metadata. The frozen
152.0.7977.82 headful profile is reused and marked unverified-reused-controlled.
No target site ran. Snapshot native crash investigation remains separate and open.
