# Clearance completion investigation, 2026-09-10

## Result

Mimic has **not** reached the application. The ordinary binary still receives
403. Frozen Chrome 152.0.7977.82 reaches the application's 404 page, and a
subsequent GET in that same Chrome context also returns 404 without a challenge.

The new discriminating observation is **different server-supplied continuation
programs**, before the subsequent document navigation. This invalidates the
hypothesis that Mimic's transport simply changes a successful POST into GET.
It does not yet identify which input caused the server to choose a retry.

## Evidence

Private captures, not included in Git:

* `compatibility/private-captures/mimic-goal-baseline-20260910`: ordinary binary.
* `compatibility/private-captures/mimic-goal-navigation-20260910`: the same
  runtime with opt-in native navigation stacks.
* `compatibility/private-captures/chrome-goal-completion-20260910`: successful
  native run, original sources, response bodies, ExtraInfo, and revisit.
* `compatibility/private-captures/clearance-completion-offline-20260910`:
  private offline analysis artifacts and integrity hashes.

In the Mimic navigation capture, sequence 2699 records `Location.reload`,
`replaceReload: [true, true]`, and native V8 callers in the downloaded
orchestrator. The caller is a VM dispatch handler, so the stack alone does not
explain the decision. The diagnostic does not evaluate `Error.stack` or call
application stack hooks.

The final same-origin `/fo` response (Ray `a38b0c182a23da09-TBS`) sets clearance
and contains a 3,240-character encoded body. Applying the response decoder
present in that capture yields a 2,428-character VM program. This is decoding
the response program, **not decrypting cf_clearance**.

An offline extraction of the captured VM, with networking disabled and a small
recording document/location environment, reproduces the continuation:

1. Install a message listener and completion callback.
2. On completion, increment `cf_chl_rc_ni` and write its cookie.
3. Schedule `document.location.reload()`.

The native terminal response has a 3,660-character encoded body and yields a
different, 2,744-character program. Its completion callback creates a form,
sets POST and `application/x-www-form-urlencoded`, and appends hidden inputs.
The actual successful Chrome trace independently confirms that POST navigation.
The minimal offline environment does not implement all native page helpers;
its native-program replay stops at a page-specific helper after creating form
inputs. It is not claimed as a complete native replay or a browser substitute.

Thus the retry counter and GET are consequences of the received Mimic
continuation, rather than evidence that form submission was attempted and lost.
The actual server-side reason remains unknown.

## Network comparison

Merging native `requestWillBeSentExtraInfo` with the corresponding request
events shows matching final same-origin `/fo` request header values. The
corresponding child `/fo` headers also match apart from the per-run widget URL
in Referer. This compares CDP/loader observations; it is not an independent
packet capture of every wire header or a proof of identical TLS/QUIC behavior.

The new cold Mimic capture records low-entropy Client Hints first, followed by
a Critical-CH restart with accepted high-entropy hints. The earlier partial
capture's initial high-entropy hints are not evidence of incorrect cold-session
initialization.

The following earlier findings remain valid:

* Issued clearance is present, unchanged, in Mimic's subsequent Cookie header.
* Chrome also encounters the IPv6-only diagnostic host's DNS failure and passes.
* The cookie store lacks partition-key identity and selection. This is a real
  general compatibility gap, but this first-party run does not establish it as
  the cause of the retry.
* Matching request headers alone does not compare the contents of the submitted
  JavaScript observations. Those bodies differ and remain an unresolved input.

Cloudflare's documentation also distinguishes cookie presence from a universal
pass: [JavaScript Detections](https://developers.cloudflare.com/cloudflare-challenges/challenge-types/javascript-detections/)
stores detection results in clearance, while
[clearance levels](https://developers.cloudflare.com/cloudflare-challenges/concepts/clearance/)
determine which challenges a clearance can bypass. These general facts do not
establish this capture's particular server verdict.

## Diagnostic fixes

The native capture helper previously named script files using only session and
script ID. Reused IDs after navigation overwrote original challenge sources
with application sources. Names now also include execution context and source
hash; the manifest records the context. A regression test retains both sources
when session/script IDs are reused.

`MIMIC_NAVIGATION_DIAGNOSTICS=1` enables `navigationIntent` trace entries with
native V8 stack frames and available sources. The feature is off by default,
adds no JavaScript API, and is bounded to 16 frames per navigation, with a 2 MiB
source limit per frame. Other engines can emit intent without a native stack.
Treat enabled traces as private: application sources may contain credentials.

## Limits and next discriminating evidence

A follow-up instrumented Chrome capture is **not a valid success reference**:
the experimental capture attempted Fetch.enable on worker targets, which do not
support that domain, leaving worker setup incomplete. Its 403 cannot be
attributed to the application or used to compare unpacked final observations.

Automatic tool approval rejected the attempted launch of a freshly built
Mimic payload-diagnostic process with `blocked by policy`, without a more
specific reason. That launch did not occur. No successful fresh payload diff or
post-fix Mimic pass is claimed.

Next useful evidence is the owner-side Security Events verdict for the final
Mimic request and its repeated navigation: action, rule, client address, and
available bot/JA4 fields. The successful native session's initial Ray is
`a38b0cd01ac1acf5`; Mimic's repeated document 403 is
`a38b0c24dab7da09`. Absence of a challenge-platform subrequest from sampled
Security Events does not prove that the request did not reach the service.

Do not force a POST, reuse Chrome proof values, suppress the retry, or add
unrelated Web APIs to make this trace look successful. Isolate an actual
semantic defect with a local Chrome differential before changing runtime
behavior. A future success requires a normal Mimic application response and a
successful same-session revisit, not merely receipt of cf_clearance.

## Validation of the diagnostic changes

* `go test ./...`: pass; browser package 187.114 seconds.
* `go test -race ./internal/engine/v8 -run TestNativeStackCapture -count=1`:
  pass, 3.311 seconds. This is a focused race check, not the full race suite.
* `python -X utf8 -m unittest tools.compatibility.test_capture_native`:
  all five tests pass.
* `.build/mimic.exe` rebuilt. Navigation semantics are unchanged by this batch;
  the binary adds opt-in diagnostics, not a claimed fix for the server rejection.
* Both temporary Mimic servers started for the ordinary and navigation-stack
  captures were stopped. The user's Chrome process was left running.
