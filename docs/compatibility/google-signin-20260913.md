# Google sign-in: compatibility repairs and transport investigation

## Latest verification

The final fresh Mimic run (`google-signin-20260913-allfixes-mimic`), after tests
completed, stayed on the identifier page and displayed account-not-found for
the reserved-domain test identifier, matching Chrome's control result. It did
not navigate to the unsafe-browser rejection page. The actual account POSTs
include `charset=UTF-8`, `Sec-CH-UA-Form-Factors: "Desktop"` and
`Sec-CH-UA-WoW64: ?0`. There were zero Network.loadingFailed events and zero
account-page jserror requests. The final identifier request body was 4306 bytes.
The two separate homepage getAttribute exceptions remain.

Binary SHA-256:
`53bef197c1a9c59da884fe28dd0bdbfea6885d63d5008e50ffba0586c4878e48`.
This is one successful control run, not a measured success rate or proof of
which changed signal affected admission. No password or authentication was used.

## Extended investigation and repairs

After the initial three fixes, a fresh sequential pair of captures
(`google-signin-20260913-verified2-{chrome,mimic}`) still produced account-not-found
in Chrome and a server-directed rejection in Mimic. Both had zero failed network
requests; Mimic retained two homepage exceptions. The Mimic binary SHA-256 was
`80484c77dbde2aef7904a499614d85b1d08b8634698e553d06f0122659ef81be`.
An earlier `verified-mimic` attempt stopped at homepage observation because Python
could not print multilingual text under CP1251; it was not a submitted login trial.

### Additional confirmed repairs

* Accepted `Sec-CH-UA-Form-Factors` and `Sec-CH-UA-WoW64` were missing from the
  default network projection. Form factors were also absent from
  `getHighEntropyValues`. They now derive from shared UA metadata, including CDP
  overrides, and participate in document permissions and redirect cleanup.
  Native Chrome measurements cover default Desktop, omitted/empty/explicit
  overridden form factors, and WoW64. Page tests check opt-in, projection identity
  isolation and suppression on a redirect to an origin without permission.
* XHR string bodies did not normalize existing charset parameters to UTF-8.
  A local receiver confirmed this independently of Google. The fix follows the
  measured Chrome 152 behavior, preserving header spelling and quoting, including
  its legacy scan inside a quoted parameter. Explicit empty string bodies receive
  the default type; null and GET/HEAD bodies are distinguished. No charset is
  appended to an existing `text/plain` without a charset parameter, matching Chrome.
  Regression tests send Russian text and emoji through both JS engines and check
  the actual received body and header. This is a string-body change, not a claim
  of complete XHR body-type or response-decoding support.
* The project Playwright CLI wrapper decoded each output Buffer independently,
  which could corrupt UTF-8 split across pipe chunks. Its stdout/stderr readers
  now use incremental UTF-8 decoding. The real CLI fixture passes Cyrillic and
  emoji through input, DOM observation and snapshot output. Its HTML declares
  UTF-8 as well. The live-comparison Python module explicitly configures diagnostic
  stdout/stderr as UTF-8; a redirected subprocess check passes with CP1251 as its
  initial encoding. No global console settings or installed CLI files were changed.

Local receiver evidence is retained privately in
`request-edges2-20260913-chrome`, `request-edges-fixed-20260913-mimic` and
`request-hint-overrides-20260913-chrome`. The charset scan is also consistent with
[the pinned Chromium implementation](https://chromium.googlesource.com/chromium/src/+/d04cdb24d67b081f6cf80200ffc5233f44b61109/third_party/blink/renderer/core/xmlhttprequest/xml_http_request.cc).

### Outgoing-request comparison

Chrome's request events were combined with `requestWillBeSentExtraInfo`; comparing
only its first event to Mimic would falsely report many missing Chrome headers.
Cookies, session values and opaque payload contents remain in private captures.

| Observation before/at identifier submission | Chrome | Mimic, before these additional repairs |
| --- | --- | --- |
| Account traffic protocol | H3 | H3 |
| UA, language, accepted encodings, Origin, Fetch Metadata, account POST priority | Same observed values | Same observed values |
| Extra accepted hints | Form-Factors and WoW64 sent | Missing, now repaired |
| Account POST charset spelling | UTF-8 | utf-8, now repaired |
| Identifier form-encoded body bytes | 4450 | 4633 |
| Request events through identifier submission | 66 | 32 |

The counts include ancillary homepage traffic and one Chrome data URL; they are
not counts of required authentication steps. Chrome sent homepage telemetry and
font requests absent from Mimic. Both sent browserinfo and batched account RPCs.
Browserinfo reported equal screen dimensions but different viewport dimensions,
consistent with the different window configurations. Payload length and per-session
values alone do not identify a failed browser primitive or the rejection rule.
The account cookie-name set at submission matched in the earlier rejected pair;
ordering differed. The initial successful Mimic capture also omitted the two hints,
so their absence is not established as a sufficient cause of rejection.

### Fresh lower-level measurements

The current transport was rebuilt and compared with the frozen headful Chrome
using a local TLS/QUIC receiver, not just CDP or a fingerprint string.

| Layer | Result |
| --- | --- |
| TCP TLS ClientHello | Fresh Chrome cold/resumed normalized observations match the retained oracle; current Mimic wire regressions pass. A separate H2 receiver directly captured matching cold ClientHellos from both. |
| QUIC TLS ClientHello | Cold and resumed normalized extension payloads, cipher ordering, session-ID length and transport parameters match between fresh captures. |
| QUIC JA4 | Both cold `q13d0313h3_55b375c5d22e_eb028bd37c08`; both resumed `q13d0314h3_55b375c5d22e_40246181ac92`. |
| HTTP/3 control stream | SETTINGS order/values match: 1=65536, 6=262144, 7=100, 51=1, followed by GREASE. SETTINGS/GREASE precede request PRIORITY_UPDATE frames. |
| HTTP/2 | Both send ordered SETTINGS 1=65536, 2=0, 4=6291456, 6=262144 and initial connection WINDOW_UPDATE increment 15663105. First request dependency/weight match in the probe. |
| Runtime HTTP/3 decoding | Dynamic QPACK, trailers and connection reuse pass focused real-socket and race tests. |

Artifacts: `.build/google-wire-review-h3-comparison.json`,
`.build/google-wire-review-h2-comparison.json`, `.build/google-wire-regressions.log`.
The H2 receiver is a separate ignored diagnostic copy of the existing network
probe, adding reads of TCP TLS records and decrypted H2 frames. The retained
frozen fixture was not replaced. Probe workloads differ after the first request:
Chrome navigates and fetches; the transport client makes plain GETs. Their later
header-block sizes, request counts and priority weights are not equivalent-workload
comparisons and were not treated as defects.

The local probe temporarily trusts its own loopback certificate and removes that
trust afterward; the QUIC reference records its force-origin flag. This does not
prove matching packet pacing, ACK/loss recovery, migration, connection scheduling,
DNS/HTTPS-record handling, real ECH negotiation or outbound QPACK compression on
the Google route. The outgoing QPACK encoder remains static-only. A cleartext
receiver also exposed the already explicit net/http fallback's different header
ordering; it does not exercise the Google HTTPS transport. System packet capture
was unavailable: `pktmon status` returned access denied. No driver, elevation,
system capture configuration or Google-specific transport override was introduced.

Two homepage getAttribute-on-null exceptions persist in separate diagnostic runs.
Logging null DOM lookups found guarded optional lookups, not a demonstrated bad
selector result. The experiment preserved return values and was confined to a
diagnostic homepage run; no instrumentation was installed in production. Their
underlying cause and the server's admission decision remain unresolved.

The original three-fix full suite passed (`.build/google-fixes-tests-final.log`).
For the additional repairs, the complete suite passed every package except CDP
(`.build/google-allfixes-tests.log`). Its child-parser interruption test passed ten
isolated repetitions but failed again in the full CDP package. Inspection found
that its 20 ms wait after scriptStart did not establish that compilation had
finished or that the tested DOM write had occurred. The fixture now emits a
console marker after that write and the test waits for it before interrupting;
the snapshot-content assertion is unchanged. The entire CDP package then passed
(`.build/google-allfixes-cdp-barrier.log`). No production change was needed for this
test synchronization repair. These runs collectively validate all packages;
there was no additional complete-suite run after the test-only correction.
Focused browser/state tests, the 27-capture provenance check, and the real
Playwright Unicode flow also passed.

## Implementation follow-up

The three diagnosed defects have now been repaired in the general runtime:

* Constraint validation derives live ValidityState flags from the existing
  control state. It supports stable object identity, required/type/pattern and
  numeric constraints, custom validity, barred controls, invalid events and
  validation before activated form submission. Custom errors and editing state
  are shared with isolated worlds. Numeric user input distinguishes an unfinished
  value from programmatic sanitization; form reset clears editing state but keeps
  custom validity. The retained Chrome oracle has 95 matching observations.
* Dynamic classic scripts with async=false fetch concurrently and execute in
  insertion order, including failed loads. Force-async belongs to canonical DOM
  nodes, so parser creation, attribute changes, cloning and isolated worlds agree.
  The ordered queue belongs to the document's main realm and is released at teardown.
* HTTP/3 consumes dynamic QPACK encoder instructions and decodes relative and
  post-base fields, with capacity/blocked-stream limits, eviction, wrapping,
  acknowledgments and context cancellation. State is isolated per connection.
  The request encoder remains static-only. A local real-QUIC test covers headers
  that arrive before their table entry, trailers, and reuse across two POSTs.
  The transport adapter now exposes trailer values populated at body EOF.

No Google hostname, obfuscated identifier or response value enters production
logic. Frozen network settings and existing tests were not weakened.

### Live observations after the changes

Two submitted diagnostic captures currently establish different outcomes:

| Build/capture | Result | Diagnosed errors |
| --- | --- | --- |
| `google-signin-20260913-fixed-mimic` | Identifier page: account not found, matching Chrome's response to the nonexistent address | No account-page jserror requests; no failed network requests |
| `google-signin-20260913-final-mimic` | Server-directed rejected page | No account-page jserror requests; no failed network requests |

The latter build includes additional form-editing and canonical script-state
corrections. These are different builds and sessions, so the observations do not
isolate which input caused the server outcome to differ. In both runs the earlier
badInput, module-constructor and QPACK errors are absent. Both retain two
getAttribute-on-null exceptions on the Google home page, a separate issue from
the three diagnosed defects. There is no claim of reliable Google admission or
successful authentication: only a reserved-domain test identifier was submitted.

Binary SHA-256: initial fixed build
`e67249a6090abab9809ffa234b465899f0078368e12c60c9e5afd6ef56ee317c`;
subsequent final diagnostic build
`41db2087c7951cdb7cac32720093e7c5d9343bce724507686ebf1cc2442e02d0`.
The optional-surface installer guard was corrected after these captures; it does
not remove or replace ValidityState in the full Chrome bundle.

### Validation and remaining boundaries

New regressions cover the Chrome form oracle, trusted invalid events and submit
cancellation, isolated-world state, numeric user/programmatic input, script async
state, out-of-order script fetches and failed dependencies. Existing keyboard,
form-control and script-cleanup tests pass alongside them. QPACK tests include
the RFC 9204 Appendix B vectors, Huffman strings, relative/post-base literals,
wrapped counts, eviction, malformed input, blocked limits and cancellation,
concurrent static responses, and independent connections. Focused QPACK and
real-socket HTTP/3 tests also pass under the race detector.

The first full repository test run found an optional-surface initialization
regression when ValidityState was omitted from a selected bundle. This is fixed;
all three failing test families pass on rerun. The final complete run is recorded
separately in `.build/google-fixes-tests-final.log`.

Native validation UI is not rendered by reportValidity. Validation-message
localization is limited to the implemented Russian/English cases; complete native
wording for every email, numeric/date and length failure is not established.
Numeric editing does not establish complete native number-widget/caret behavior.
These limits must not be described as exhaustive constraint-validation or Chrome
network equivalence. The initial diagnostic report below is retained as provenance.

## Result

On 2026-09-13 a fresh Mimic build from `c7d73c949512f0aacf8562fbc370c9d55eb20efc`
was compared with frozen headful Chrome 152.0.7977.82. Both opened
`https://www.google.com/`, followed its account sign-in link, entered the same
reserved-domain test identifier, and activated Next. No password, real account,
or existing browser profile was used.

* Mimic: the page navigates to `/v3/signin/rejected` and displays the Russian
  equivalent of "This browser or app may not be secure".
* Chrome: the identifier page displays "Couldn't find your Google Account".
* Mimic's identifier RPC response itself contains the rejected-page URL. The
  decision is therefore delivered by the server; it is not merely a local
  unsupported-browser message caused by an uncaught exception.
* Both report the same JavaScript User-Agent string. This does not establish
  equivalent headers, transport, environment or server-side classification.

There is one submitted trial per runtime. The preceding exploratory trials only
loaded the identifier form: its type is text, so an initial email-type selector
did not submit. No success rate or claim about all addresses follows from this.

## Confirmed ordinary JavaScript defect

The sign-in application posts an error report containing:

```
TypeError: Cannot read properties of undefined (reading 'badInput')
```

Its form handler reads `e.validity.badInput`. A separate about:blank probe with
new input elements, without Google code or requests, confirms:

| Observation | Mimic | Chrome 152 |
| --- | --- | --- |
| input.validity, text/email/number | undefined | ValidityState object |
| required empty input: valueMissing / valid | unavailable | true / false |
| email value "hello": typeMismatch / valid | unavailable | true / false |
| number value "42": valid | unavailable | true |
| input.willValidate in these cases | undefined | true |

The Chrome object retains identity across the observed reads and its properties
update with the control value. Mimic exposes a checkValidity function, but this
probe did not invoke it, so no correctness claim about that method is made.

This is a general form-validation defect with a direct link to a real application
exception. It is not proven to cause the server's rejection.

## Other findings and causal limits

1. Mimic reports a module-load error, `_.lEb is not a constructor`, before
   submission. Saved script responses include a use and a definition in separate
   chunks. Later errors concern undefined prototypes and a missing application
   service. Dependency/load/evaluation ordering deserves an isolated reproduction;
   response arrival order alone does not prove execution order or its root cause.
2. On the Google home page, Mimic reports two getAttribute-on-null exceptions.
   They are separate from the form's caught errors sent through the jserror
   endpoint. A Runtime.exceptionThrown-only audit would miss the latter.
3. Home-page ancillary POSTs to the OneGoogle service and logging service fail
   with `qpack: expected Required Insert Count to be zero`. The HTTP/3 profile
   advertises dynamic table capacity 65536 and 100 blocked streams, while the
   vendored HTTP/3 implementation ignores QPACK encoder/decoder streams and
   documents no dynamic-table support. This is a concrete transport capability
   inconsistency. The observed sign-in RPCs return HTTP 200; these ancillary
   failures are not established as the rejection cause.
4. Chrome's submitted diagnostic trial records zero Runtime.exceptionThrown and
   zero Network.loadingFailed events. Mimic records two and four respectively;
   one failure is an aborted request during navigation. These are top-level CDP
   observations, not exhaustive traces of every iframe or caught exception.

Relevant implementation locations: `internal/webapi/form_controls.js`,
`internal/network/chrome152_transport_profile_test.go`, and
`third_party/quic-go-utls/http3/conn.go`.

## What was observed being sent

The account page sends a browserinfo request with a compact array of environment
measurements, including dimension-shaped values; a batchexecute RPC for page
initialization and identifier submission; and, in Mimic, jserror reports containing
exception traces, module-load failures, page context, build/experiment metadata
and elapsed/wall-clock timestamps. Ordinary request metadata and session cookies
are additional observations available to the service.

The browserinfo arrays differ in viewport-shaped values and their final numeric
group. The meaning of every flag has not been traced to its producer. Different
window sizes are not a browser defect. Opaque RPC values were not reverse
engineered, and this investigation does not claim a complete inventory of
fingerprinting, telemetry, iframe activity, or hidden decision inputs.

## Proposed implementation, in separate changes

### First: coherent constraint validation

Extend the existing authoritative control state and semantic bindings, rather
than supplying a constant validity object. Implement the ValidityState owner
relationship, stable identity and realm-correct accessors; derive flags from
current control state. Cover required, applicable type mismatch, pattern,
range/step constraints and custom validity with Chrome-measured behavior. Preserve
the distinction between user-entered invalid input and programmatic sanitization.
Implement coherent willValidate, validationMessage, setCustomValidity and
checkValidity semantics, including invalid-event cancellation and controls barred
from validation. Define reportValidity's observable boundary without inventing
native validation UI. Do not infer every flag's rules from these three probes.

Add focused synthetic Chrome 152 differential tests: empty/filled required text,
invalid/valid email, numeric sanitization and user edits, disabled/readOnly/barred
controls, custom error set/clear, live identity, cross-realm receivers, and invalid
event behavior. A test that only reads badInput=false is insufficient.

### Second: independently reduce module initialization

Capture actual dependency and script execution ordering, then construct a local
fixture with delayed dependencies. Check parser-inserted versus dynamically
inserted scripts, async=false ordering, load/error delivery, and document lifetime.
Only a reproduced semantic divergence should determine the implementation change.
The obfuscated constructor name must never enter runtime logic or regression tests.

### Third: make HTTP/3 claims match implementation

Support the advertised QPACK dynamic table and stream coordination, with local
response-decoding, blocking/unblocking, cancellation and connection-isolation
tests. Until that exists, explicitly document/constrain the unsupported capability
rather than treating a matching SETTINGS fingerprint as functional compatibility.
Do not silently retry non-idempotent POSTs after ambiguous transport failure.
Existing frozen expectations must not be weakened to hide the mismatch.

### Completion criteria

Passing local form and ordering tests, eliminating the corresponding application
exceptions, and valid HTTP/3 response handling are compatibility criteria. A changed
Google verdict requires a new independent observation and must not be inferred
from the disappearance of errors. Google's published policy also permits rejection
of automated or embedded browsers, so complete JavaScript execution does not
guarantee admission. An application needing Google authentication can use the
supported external-browser OAuth flow.

Sources: [Google supported-browser help](https://support.google.com/accounts/answer/7675428?hl=en)
and [Google developer guidance](https://developers.googleblog.com/guidance-to-developers-affected-by-our-effort-to-block-less-secure-browsers-and-applications/).

## Evidence and limits

Private records live in `compatibility/private-captures/google-signin-20260913-*`
and `google-form-probe-20260913-*`; response bodies and session values are not
promoted into tracked files. The submitted pair has the `submit-` prefix.
Exploratory capture retrieves bodies after navigation, so some old-document
responses are unavailable; the retained account RPC response establishes the
reported rejection. The helpers are `.build/google_signin_observe.py` and
`.build/google_form_probe.py`; the fresh binary is `.build/mimic-google-analysis.exe`.

Browser launch commands and version responses are retained. Chrome uses a new
headful profile, but this is a diagnostic comparison, not a newly certified oracle
fixture: window geometry was not pinned and complete oracle metadata was not
collected. Input used CDP insertText and a script-driven button click in both
runtimes. No private existing tabs were inspected. Owned browser processes were
terminated after each run. The initial investigation made no implementation
changes; the follow-up changes are described above.
