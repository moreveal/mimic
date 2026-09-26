# Google Search execution comparison — 2026-09-26

## Status

The remaining Google rejection is **not resolved**. The experiments below
establish several counterexamples and repair a defect in the comparison
infrastructure. They do not identify Google's private admission rule.

This follows [the response opt-in investigation](google-serp-20260926.md).
Raw captures, application programs, cookies and request URLs remain private
under the ignored `.build/google-serp-success-20260926/` directory.

The private `execution-proof-manifest-20260926T044007.json` records hashes
for 51 evidence artifacts, the tested binary, and saved diagnostic scripts.
Its SHA-256 is
`7d94d32d81141449408edabd25e444c542d145be715fba3cebc090e4595c482b`.

## Network/execution crossover

A diagnostic bridge uses the actual frozen Chrome 152 Mimic transport.
Both the initial document and the subsequent Search request can pass through
that bridge. The static request headers come from a Mimic capture; cookies
and referrers belong to the fresh executing browser context. Response bodies
are saved before committing the subsequent navigation.

| Execution environment | Network/header handling | Observed subsequent response |
| --- | --- | --- |
| Chrome 152, ordinary minimal launch | Mimic transport and header set for both documents | HTTP 200, 9 result headings |
| Mimic, first document loaded normally | Same bridge for the subsequent request | HTTP 302, no result headings |
| Mimic, both documents forwarded, after the fulfillment fix below | Same bridge and header set | HTTP 302, no result headings |

These results demonstrate that the Mimic transport and static header set can
carry a successful session. They do not establish equality of execution,
fresh server challenges, cookie values, timing, or all session history.
The result-heading counts describe returned HTML, not a completed rendering
of the result page.

## Confirmed fulfillment defect

The initial crossover with both Mimic documents forwarded was confounded:
the intercepted response bypassed the loader's cookie and Client Hint
acceptance. Its browser-visible response timing also remained zero.
Consequently, only the script-created cookie was sent in that experiment.
It must not be treated as a matched execution-only comparison.

A local HTTPS fixture, with every response fulfilled locally, isolated the
same behavior without Google:

| Observation | Frozen Chrome 152 | Mimic before | Mimic after |
| --- | --- | --- | --- |
| Script-visible response cookie | Present | Absent | Present |
| HttpOnly response cookie on next request | Present | Absent | Present |
| Accepted bitness hint on next request | Present | Absent | Present |
| Response start/end include a 30 ms fulfillment wait | About 36 ms | Zero | About 33 ms |

The shared response-state helper now accepts all `Accept-CH` field lines and
updates the canonical cookie store for both received and fulfilled responses.
Credential restrictions still apply. Cache reads do not repeat these effects.
Fulfillment records its measured browser wait separately from socket timing;
no transport observations are invented. This change covers request-stage
fulfillment; it is not a claim that every response-stage interception behavior
or synthetic-response metric is now Chrome-equivalent.

The focused regression failed before the fix on cookies, Client Hints,
redirect cookies and timing. It passes afterward, including credentials-omit
coverage. The live forwarded Mimic case still receives HTTP 302 after the
fix, so this defect alone does not explain the ordinary Search rejection.
An ordinary, non-fulfilled first response already had nonzero network timing
and all five observed session-cookie names before this fix.

## Saved-program execution evidence

The successful Chrome 152 bootstrap was retained and replayed locally.
No replay request contacted Google. A fixed random sequence was used only
in offline analysis to align the program's randomized inspection choices.

The diagnostic instrumentation records selected property reads, calls,
arithmetic/comparison operations and serialization inputs. The program
reconstructs functions from their source; the recorder preserves the original
source at those observed reconstruction boundaries. This is diagnostic
instrumentation, not a production browser behavior change.

In the latest matched-capacity replay, Chrome and Mimic each produced 1,049
recorded observations in the same sequence, created the script cookie, and
requested the next document without a page exception. Recorded comparison
results agreed. Recorded serialization differences were attributable to
heap usage/capacity, clocks and durations, network estimates, and window
dimensions. Earlier zero-timing branches varied with intercepted-request
timing; they did not establish a failure in the ordinary network path.

This is **not** a complete CPU-instruction trace or proof that all final
encrypted output bytes agree. Equal cookie lengths are not evidence of
equal contents. Two attempted full register-recorder runs produced no API
observations and no script cookie in either runtime; those captures are
retained as invalid instrumentation controls and excluded from conclusions.

## Controlled counterexamples

Successful response bodies and the fresh bootstrap for each live control
were preserved. Existing browser processes were reused for local replay.
Additional launches addressed specific questions that the saved evidence
could not answer.

1. **Built-in extension surface:** disabling ordinary extensions still left
   the component extension active. Disabling background component extensions
   removed the observed `chrome.runtime` member. Chrome still received HTTP
   200 with 9 result headings through the Mimic bridge.
2. **Heap limit:** a real V8 old-generation limit of 4,000 MiB produced an
   observed document heap limit of exactly 4 GiB, matching Mimic. This used
   V8 configuration, not a replacement API getter. Chrome still received
   HTTP 200 with 9 result headings.
3. **Execution duration:** the same reference with 8x CPU throttling still
   received HTTP 200 with 9 result headings. The measured interval from
   bootstrap delivery to the next intercepted request was 503 ms. This
   interval includes browser and diagnostic overhead; it is not pure VM CPU
   time.
4. **Window dimensions:** resizing the real reference window reproduced
   outer dimensions 1296×808 and inner dimensions 1280×720. Combined with
   the previous extension, heap-limit and CPU controls, it still received
   HTTP 200 with 9 result headings (469 ms delivery-to-request interval).

The extension distinction is also described in Chromium's
[component loader](https://chromium.googlesource.com/chromium/src/+/refs/heads/main/chrome/browser/extensions/component_loader.cc).
Actual recorded observations, rather than the flag name alone, established
whether the extension surface disappeared in the frozen reference.

These are counterexamples to each listed condition being sufficient for
rejection. They do not exclude interactions with other variables or changes
in the server's state between independent sessions. Heap usage values and
network estimates have not been causally isolated. No arbitrary replacement
of those observations is justified by this evidence.

## Verification and remaining boundary

Only focused tests were run: fulfillment/session behavior, existing response
Client Hints, Critical-CH restart, SameSite selection, and the previously
changed platform-member, form-control, callable-metadata and timing behavior.
The full local suite was not run. The current Mimic diagnostic binary was
rebuilt before repeating the fulfillment fixture and live crossover.

The worktree also contains earlier measured fixes for platform-member order,
`HTMLInputElement.textLength` exposure, and legacy CSI clock/lifecycle
observations. Those compatibility fixes did not establish Google admission.

The supported conclusion is narrower than a server-side root cause:
the captured program reaches its next-request path in both runtimes, the
Mimic transport can deliver admitted responses, and several conspicuous
environment differences are individually insufficient explanations. A claim
that a particular remaining field causes rejection still requires a controlled
counterfactual or server-side decision evidence.

## Legacy event divergence and focused fix

The saved failing Mimic bootstrap from the later crossover experiment exercises
legacy mouse event creation. An instrumented offline replay recorded 313 events
in Mimic versus 1055 in Chrome 152. At the divergence, Chrome observes
`initMouseEvent` and `screenY`; Mimic subsequently reads `screenY` from an
undefined value. A separate local document oracle confirms that Mimic rejected
`MouseEvent`, `MouseEvents`, `UIEvent`, and `UIEvents` with `NotSupportedError`.
Chrome creates the corresponding events.

The shared fix supports these creation aliases and initializes the existing
canonical UI/mouse event slots. Event reinitialization resets cancellation and
propagation flags and is ignored during dispatch. Focused tests cover aliases,
parameters, defaults, integer conversion, reinitialization, dispatch protection,
and callable arity. Existing CustomEvent and platform member-order tests pass.

After the fix, the same instrumented bootstrap records 1051 events, with the
mouse-event failure removed. Sequence alignment leaves one four-event Chrome
block involving indexed string access and `charCodeAt`; this remaining difference
has not been classified. Both uninstrumented saved bootstrap cases also reach
the next navigation without page exceptions. These are offline semantic results;
no fresh Google server acceptance is established by this checkpoint.

Private captures, recorder output, and oracle scripts remain in ignored `.build`.
No historical challenge answers or cookies are added to public source.
