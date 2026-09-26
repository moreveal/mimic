# Anti-bot checks as a compatibility gate for Mimic

Status: development and diagnostic methodology. The behavioral reference is
frozen headful Chrome `152.0.7977.82`, as defined in
[oracle-policy.md](../oracle-policy.md).

## 1. Purpose and boundaries

An anti-bot system is an integration sensor: it exercises JavaScript, DOM,
realms and lifecycle, networking, Client Hints, performance, media, graphics
readbacks, and the scheduler at the same time. Its final outcome (`pass`,
challenge, reload, or block) is useful, but it is neither a browser specification
nor evidence of a root cause.

The gate answers two separate questions:

1. **Semantic Gate:** Has Mimic moved closer to Chrome 152's observable behavior
   along the program path that was actually consumed?
2. **Live Outcome Gate:** Does an authorized control deployment pass with the
   environment and network conditions held constant?

The first gate is mandatory before accepting a fix. The second is a periodic
integration signal. It must not reject an otherwise correct change merely
because server policy, IP reputation, the challenge program, or a risk decision
changed.

Run live checks only against an owned or explicitly authorized deployment, with
rate limits and without attempting to circumvent third-party access controls.
Challenge scripts, answers, cookies, and tokens are private artifacts and must
not be committed.

## 2. Established repository findings

The history `7542e83 -> 827bcb0 -> 8c75f5b -> 5ce41e7 -> 4d29dff`
established these process invariants:

- Replay must select responses by occurrence, method, URL, initiator, realm
  context, and navigation cycle. Reusing the first GET creates false coverage.
- Critical-CH restarts, cache/blob/synthetic responses, and actual wire
  responses must be accounted for separately.
- Offline replay proves coverage of a saved execution path. It does not prove
  fresh server acceptance of a newly generated payload.
- Field equality is not the only acceptance criterion. Clocks, randomness,
  environment, server responses, and bounded graphics require classification.
- A stable difference must be connected to its producer, consumer, and control
  flow branch. An obfuscated field name alone is insufficient because one label
  may have multiple producers.
- A/B repeats are mandatory. A difference already unstable across two Chrome
  runs cannot become a Mimic constant.
- Instrumentation can change timing and branches. Descriptor/access hooks first
  localize an area; a minimal probe without the instrumented challenge must then
  test the hypothesis.
- A fix must model general browser behavior and have a frozen-Chrome oracle. It
  must not check a URL, detector name, token, or captured hash.

## 3. Gate architecture

```text
                authorized live run
               /                   \
      Chrome 152 capture       Mimic capture
               \                   /
          provenance + integrity check
                       |
             normalized phase timeline
                       |
        A/B stability and field registry
                       |
          consumed-path differential
                       |
             minimal Chrome oracle
                       |
      structural fix + regression test
                       |
     offline replay of the frozen exchange
                       |
           semantic gate / live canary
```

Use three tracks:

| Track | Frequency | Blocks merge | What it proves |
| --- | --- | --- | --- |
| Focused oracle | Every fix | Yes | A specific behavior matches Chrome 152 |
| Frozen offline replay | Every relevant fix | Yes | The saved path did not regress and the capture is fully accounted for |
| Authorized live canary | Scheduled or before a checkpoint | Not by itself | Current end-to-end compatibility in one recorded environment |

## 4. Experiment contract

Create `experiment.json` before running a series and do not change its conditions
within that series. Record:

- A unique `experimentId`, UTC time, purpose, and hypothesis.
- Commit, dirty state, and SHA256 hashes of the Mimic binary and capture tools.
- Exact Chrome, Chromium, and V8 builds and the profile ID.
- Windows build, CPU, RAM, timezone, locale, display/window/viewport, and scale.
- Headful/headless mode, launch flags, CDP endpoint, and automation client/version.
- The deployment URL in a private manifest and a stable public route label/hash.
- Fresh/warm profile and cookie, cache, and storage policies.
- Network context: egress identity label, proxy/VPN, protocol, DNS, and connection reuse.
- Start condition, timeout, settle condition, and terminal observable state.
- Probe/instrumentation versions and their SHA256 hashes.
- Redaction policy and the capture bundle file inventory.

Run at least four independent profiles in alternating order: `Chrome A`,
`Mimic A`, `Mimic B`, and `Chrome B`. If the server program, challenge headers,
or top-document body hash changed between controls, classify the series as
**server-drifted** and exclude it from causal differential analysis.

Do not compare warm Chrome with fresh Mimic, different viewport/locale/timezone
settings, headless with headful, or absolute timestamps from different clock
domains.

## 5. Capture bundle format

Store every run in a separate ignored directory:

```text
private-captures/<experiment>/<runtime>-<repeat>/
  manifest.json
  events.jsonl              # CDP events in receive order
  trace.json                # authoritative Mimic trace; Mimic only
  root-<request-id>.json     # Network.getResponseBody envelope
  frames.json               # frame/session/loader topology
  outcome.json              # observations only, not interpretation
  process.json              # PID/start time/binary hash/build info
  console.txt
  stderr.txt
  hashes.sha256
```

At minimum, retain:

- All `Target`, `Page`, `Runtime`, `Network`, and `Log` events with `sessionId`.
- Requests, responses, failures, redirect chains, methods, resource types,
  initiators, frame/loader/context IDs, request post data/hash, and response body/hash.
- Headers before redaction and protocol/timing/cache/service-worker/connection fields.
- Console remote-object types, exceptions, stack traces, and lifecycle events.
- Top-frame outcome: URL, status, `cf-mitigated`, title, readyState, reload count,
  navigation method, and whether a subsequent application document was reached.
- For Mimic, `Mimic.getTrace`, including scheduler, resource, semantic-missing,
  surface-missing, unsupported, and internal error records.

Redact secrets only in a derived copy. Encrypt or retain the raw bundle locally;
analysis should use stable keyed hashes, lengths, and structural shapes. Plain
SHA256 is insufficient protection for low-entropy cookie and token values.
Compute `hashes.sha256` before analysis and never modify the originals afterward.

## 6. Capturing Chrome 152

1. Start the exact headful binary with a separate fresh profile and the
   configuration and [normal browser launch rules](../oracle-policy.md#normal-browser-launch-and-recording)
   in `docs/oracle-policy.md`. Launch directly and attach the recorder; verify
   the unmodified automation state and preserve the full command line. Check
   `/json/version`; a product
   mismatch immediately invalidates the series.
2. Attach one recorder through CDP, enable domains before navigation, clear
   cookies/cache according to the experiment contract, and register every
   target/frame/session before page execution.
3. Install instrumentation through `Page.addScriptToEvaluateOnNewDocument` only
   in a separate diagnostic run. The control outcome run must not wrap prototypes.
4. Save every response body by `requestId`, using base64 for binary data. Never
   decode arbitrary bytes as UTF-8.
5. End capture on a semantic condition: the next application document, a new
   challenge cycle, an explicit block, or a bounded timeout, not merely `load`.
6. Close the profile and record process identity and hashes.

The existing development probe provides the first layer of consumed-surface
inspection:

```powershell
python compatibility/chrome_api_trace.py `
  --chrome http://127.0.0.1:9223 `
  --url <authorized-url> `
  --settle-ms 8000
```

It helps locate APIs accessed before the first flow request, but does not replace
a full CDP trace and affects the execution environment. For a narrow semantic
oracle, use the three-way runner:

```powershell
python compatibility/oracle_differential.py `
  --headful http://127.0.0.1:9333 `
  --headless http://127.0.0.1:9444 `
  --mimic http://127.0.0.1:19222 `
  --probes compatibility/probes.json
```

A headless result is non-authoritative until the exact probe has been shown to
be mode-invariant.

## 7. Capturing Mimic

1. Build once from a recorded commit and retain the binary SHA256 and build
   information. An unidentified `go run` process is not proof of its source revision.
2. Use the same route, profile/environment, viewport, cache/storage policy,
   recorder, and semantic stop conditions as Chrome.
3. Capture the full trace alongside CDP events without waiting for a stalled
   browser operation to complete:

```powershell
python compatibility/get_trace.py `
  --endpoint http://127.0.0.1:19222 `
  --url-contains <stable-path-fragment> > .build/run/trace.json
```

`--summary` and `--compact` help navigate an investigation, but retain the
unfiltered raw trace for replay and later analysis.

4. Save response bodies as `root-<request-id>.json`. A
   `Network.getResponseBody` error, a synthetic/cache response, and an absent
   body are distinct states and must not collapse into an empty string.
5. Perform a second independent run. Do not reuse the Page, realm, profile, or
   challenge state.

## 8. Normalization and analysis

### 8.1. Compare phases before events

Partition each run by observable boundaries:

1. Initial navigation and Critical-CH restart.
2. Bootstrap and resources.
3. Child-frame/worker creation and lifecycle.
4. Each challenge cycle and its flow requests.
5. Submission.
6. Reload, block, or the next application document.

Match by `phase + occurrence + method + normalized route + initiator + frame
role`, not by request ID, absolute sequence, or wall clock.

### 8.2. Normalize only proven dynamic data

Permitted normalization includes UUID/route segments after producer validation,
session IDs, request IDs, epoch origin, and secrets. Always retain the raw value,
normalization rule, and normalized value. Do not normalize an unknown difference
merely to make it disappear.

Compare time using happens-before ordering, intervals within one clock domain,
position relative to a known deadline, and A/B buckets or distributions rather
than absolute equality.

### 8.3. Difference registry

Give every leaf a row with this schema:

```text
phase, block, field, producer, consumer, chromeA, chromeB, mimicA, mimicB,
chromeStable, mimicStable, class, branchEffect, evidence, owner, status
```

Allowed `class` values:

- `equal`;
- `chrome-nondeterministic` / `mimic-nondeterministic`;
- `timing`, `random`, `secret`, `server-response`;
- `environment-profile`, `headless-specific`, `automation-artifact`;
- `protocol-projection-only`;
- `bounded-model` / `explicitly-unsupported`;
- `stable-mimic-divergence`;
- `unknown`.

Prioritize work in this order:

1. A stable divergence that changes a branch or submitted value.
2. An exception, lifecycle/order, or identity divergence before submission.
3. A stable consumed value without a proven branch effect.
4. A missing API that the program actually reads.
5. Protocol-only diagnostic differences.
6. Unused surface and unstable noise.

The number of `unsupported` records is not the number of root causes. Access
tracing may deduplicate names, while repeated realms can emit repeated records.

Generate a quick structural report without inferring passage:

```powershell
python tools/compatibility/summarize_trace.py `
  .build/run/trace.json `
  --output .build/run/summary.json
```

### 8.4. Causal ladder

A change is admitted only with the deepest available evidence level:

1. The API was accessed.
2. Its value differs stably from Chrome.
3. The value entered a computation or field.
4. That field changed a branch or asynchronous ordering.
5. The branch changed a request payload or navigation.
6. The outcome changed in a controlled live A/B experiment.

Level 6 is desirable, but server nondeterminism may obscure it. Levels 2-5
require a separate minimal Chrome probe and regression test.

## 9. Offline replay

Verify the harness before use:

```powershell
go test ./tools/compatibility/offline_replay -count=1
go build -o .build/offline-replay.exe ./tools/compatibility/offline_replay
.build/offline-replay.exe `
  -capture compatibility/private-captures/<capture> `
  -out .build/replay/<commit>-raw
```

Always run an unmodified replay first. Explicit diagnostic transformations may
then run into separate output directories:

- `-rebase-http-dates` applies one common shift to HTTP dates.
- `-dynamic-segment '<regexp with one capture group>'` maps a proven dynamic route ID.
- `-environment-overlay <json>` supplies independently measured environment state.
- `-body-overrides <json>` supplies an instrumented copy of a saved script body.

Every transformation must appear in the receipt and may not replace the raw run.

Accept `replay-result.json` only when:

- There was no live fallback.
- The stop reason is known and matches the capture boundary or expected timeout.
- The required navigation cycles were reached.
- There are no unexplained `unusedFixtures`, `unmatched-local` records, context
  mismatches, or response-queue exhaustion.
- Request method, body hash, and header differences have been reviewed.
- Binary, input, and tool hashes match the manifest.

A replay cannot establish that Cloudflare accepted a new payload. A saved
response does not execute the server evaluator, and TLS/H2/H3, wire timing, IP
reputation, and a fresh server decision are not reproduced.

## 10. Fix cycle

1. Record a pair of pairs and classify server drift.
2. Find the first stable divergent consumed observation, not the final different navigation.
3. Connect the observation to its producer, consumer, and branch using narrow
   temporal or descriptor instrumentation.
4. Create a minimal origin-agnostic probe for frozen Chrome.
5. Fix Mimic's authoritative state model; do not patch the anti-bot system, a
   field name, or a site.
6. Add a focused regression for identity, realm, lifecycle/order, and teardown
   where relevant.
7. Run focused tests, race/concurrency tests for shared Page state, and the
   relevant full suites.
8. Replay the same frozen capture before and after the change and retain receipts.
9. Update the field registry with what changed, why each residual is acceptable,
   and which oracle supports that conclusion.
10. Only then run a new live A/B series. If the server program changed, begin a
    new experiment lineage instead of mixing the data.

## 11. Gate criteria

### Merge Gate: PASS

- No new stable consumed divergences exist before submission.
- The corrected divergence matches headful Chrome 152 in a minimal oracle.
- There are no new uncaught exceptions, scheduler ownership/lifecycle
  violations, or Page isolation regressions.
- The replay coverage receipt is clean for the declared scope.
- Every residual has an evidence-backed classification and explicit boundary.
- Tests were not weakened, and the implementation contains no URL-, token-, or
  captured-hash-specific behavior.

### Merge Gate: FAIL

- A new unknown or stable divergence appears on the consumed path.
- Instrumentation is the only evidence and there is no control without hooks.
- Replay reuses an occurrence, hides an unused fixture, or has a live fallback.
- Outcome is declared from status or DOM text without top-frame and network evidence.
- A change copies a captured value rather than modeling the underlying semantics.

### Live canary

Report live results separately as `PASS`, `CHALLENGE`, `BLOCK`, `INCONCLUSIVE`,
or `SERVER_DRIFT`. Include outcome rates, a confidence interval when the sample
size permits, and the environment characteristics. One live pass is a useful
checkpoint, not a permanent guarantee.

## 12. Checkpoint artifacts

Commit only safe derived data:

- The methodology and schema version.
- A redacted manifest and hash inventory.
- The field registry and notes.
- A replay receipt without URLs, bodies, cookies, or tokens.
- Minimal oracle probes and regression tests.
- A concise report covering scope, first divergence, fix, evidence, residuals,
  limitations, and the live outcome as a separate signal.

Raw captures, decoded payloads, and instrumented challenge scripts remain in
`compatibility/private-captures` or `.build`. Version material gate/harness
changes. Analyze old captures with their original harness version or an explicit
migration that produces its own receipt.

## 13. Immediate implementation plan

1. Unify the Chrome CDP recorder and Mimic manual recorder under one bundle schema.
2. Add automatic provenance, hash, topology, and completeness validation.
3. Extend the summarizer with phase/occurrence matching and field-registry generation.
4. Create an immutable corpus from `manual-045102` (complete semantic closure)
   and `manual-160739` (live pass with known timing branches).
5. Run that corpus offline in CI and publish receipts/diffs only.
6. Keep the live canary outside merge CI, with a limited cadence, fresh profiles,
   pair-of-pairs controls, and an explicit `SERVER_DRIFT` result.
7. Add a minimizer that removes unused scripts/responses while preserving the
   first divergent branch. Every reduction step must reproduce the divergence in
   Chrome and Mimic or be rolled back.

This process turns an opaque binary anti-bot result into a source of prioritized
compatibility work, while keeping Chrome 152 as the only normative oracle and
offline replay as a reproducible causal-analysis tool.
