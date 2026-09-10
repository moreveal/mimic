# Differential compatibility fuzzing (bounded vertical slice)

`compat-fuzz` discovers browser-facing semantic differences through the same CDP
JavaScript evaluation in frozen Chrome 152.0.7977.82 and Mimic. It never modifies
the runtime. This first slice stops at introspection, a small realm/legacy corpus,
and deterministic event-loop invariants; it is not a Web Platform conformance suite.

## Run

Use the existing Windows Go build and Chrome oracle launch instructions in
`README.md` and `docs/oracle-policy.md`. Keep separate browser profiles and CDP
ports. Install the same Python transport used by existing compatibility scripts:

```powershell
python -m pip install -r compatibility/fuzz-requirements.txt
.\compat-fuzz.cmd --chrome-cdp http://127.0.0.1:19333 --mimic-cdp http://127.0.0.1:19322 --output .build/fuzz-run
```

`python compatibility/compat_fuzz.py` is equivalent. The `compat-fuzz` Python
launcher also accepts these flags. The output directory must be new, preventing
old findings from being mistaken for current results. Exit 0 means the run
completed (including divergences); 2 means infrastructure/probe execution errors.

The default is headful Chrome, 136 corpus cases and at most 100 surface probes.
All seven roots are visited before properties are sampled round-robin. Increase
`--max-surface-probes` to explore more of the discovered union; use `--corpus-only`
or `--filter document-all` for focused runs. `--timeout` bounds each CDP operation;
`--repeats` must be at least two. Sampling is deterministic, without a random seed.

The harness starts a loopback HTTP fixture on an available port. The exact origin
is shared by both engines; `localhost` is the cross-origin peer of `127.0.0.1`.
No public website is contacted by the corpus. Both supplied endpoints must run on
the local machine for the loopback fixture. Existing browser targets are not reused.

Chrome version and mode are checked through `oracle.py`; provenance records the
viewport, features and profile declaration. Only use `--fresh-profile --profile-id
<name>` when you actually launched a fresh controlled profile. An arbitrary caller
endpoint defaults to unverified freshness. Headless runs remain explicitly
mode-scoped and are not generic Chrome expectations.

## Output and replay

`summary.json` contains check/match/error/unstable counts, surface budget and
unique observational groups. Every group has `result.json` (category, member IDs,
Chrome/Mimic results, retained observation path, reduced probe and fixture origin)
and executable `repro.js` returning a Promise. Matches are counted but not saved.
Transport failures are diagnostics, not divergences. The first differing leaf per
probe is retained; equal category/path/typed result pairs share one group. These
are observational signatures, **not claims of a shared root cause**.

Replay a saved probe with a fresh output directory:

```powershell
.\compat-fuzz.cmd --chrome-cdp http://127.0.0.1:19333 --mimic-cdp http://127.0.0.1:19322 --replay .build/fuzz-run/GROUP/result.json --output .build/fuzz-replay
```

Replay restarts the fixture at its saved port; that port must be available. The JS
file can alternatively be evaluated via CDP on the stated fixture origin. It
includes its own helpers and setup. Origins are preserved rather than stripped.

## Design and deliberately bounded coverage

- Reuses `pyppeteer`, `oracle.prepare_page`, exact product checks and
  `oracle.capture_metadata` from existing compatibility infrastructure. Existing
  `differential.observe` shares a page between probes, so it is unsuitable for
  independent reduction candidates; this harness creates a fresh target and
  connection per evaluation. Timeout recovery has a live regression test.
- `fuzz_probes.py` owns discovery, structured probes, JS normalization and corpus.
  The walker unions inherited string properties from the seven requested roots.
  Symbol keys remain visible in `Reflect.ownKeys`, but are not individual walker
  subjects yet. Invocation is restricted to five pure query methods; unknown
  discovered methods are reflected but never blindly called.
- Reflection includes primitive/type, strict/loose null and undefined comparisons,
  truthiness, tags, prototype/constructor relations, descriptors, own-key order,
  and function name/length/source. HTMLDDA is tested with strict identity before
  normalization, preserving `document.all` separately from actual `undefined`.
  NaN, infinity, negative zero, Symbol and BigInt retain typed encodings.
- Exceptions retain class, name and message category. Category matching is a
  deliberately small heuristic, not exact-message compatibility. Prototype and
  constructor identity use explicit relations, not object serialization.
- The receiver/argument matrix covers correct receiver, `{}`, `null`, foreign
  ordinary object, and missing/undefined/null/string/object/Symbol arguments.
  Realm cases cover main, same-origin, cross-origin access, inside, detached,
  WindowProxy navigation and about:blank inheritance. This is a representative
  matrix, not every property crossed with every realm.
- Legacy cases cover document.all reflection/callability, Window/Document named
  access, HTMLCollection/NodeList access, liveness and collection identity.
- Timing covers clock monotonicity and Promise/queueMicrotask/timer/MutationObserver
  ordering. Resolution distributions and worker ordering are deferred: raw timing
  samples are not stable differential JSON. Known volatile Performance properties
  are excluded from value discovery; other unstable probes are counted separately.
- `compat_fuzz.py` owns execution, typed JSON diff, signature grouping, setup
  deletion reduction and persistence. A candidate must preserve the exact selected
  result pair on repeated executions. Final output is replay-checked. The reducer
  guarantees **1-minimal setup statements for a fixed observation**, not the globally
  shortest JavaScript AST. Helpers and compound observation expressions remain in
  repro files. Some compound setup statements contain multiple operations.

Environment-sensitive stable values can still differ across profiles. Findings
remain oracle-mode/profile-scoped candidates requiring investigation, never
instructions to change runtime behavior automatically. Expanding this tool should
add structured probe families/reduction candidates for DOM state, navigation,
workers, storage/network and WebIDL instead of special-casing runtime behavior.

## Validation

```powershell
python -m unittest discover -s compatibility -p 'test*fuzz*.py' -v
$env:COMPAT_FUZZ_CHROME_CDP='http://127.0.0.1:19333'
python -m unittest discover -s compatibility -p 'test*fuzz*.py' -v
```

Without the environment variable, live Chrome tests skip. See the measured
[slice report](compatibility/compat-fuzz-slice/report.md).
