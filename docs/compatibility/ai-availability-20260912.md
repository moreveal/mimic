# AI interface publication and unavailable backend

The missing `Summarizer.availability` operation was not specific to Summarizer.
Retained IDL uses Blink's conditional `Exposed(Window Feature, Worker Feature)`
spelling. The current parser understands it, but the surface-catalog projection
did not normalize the old retained key. Generation skipped those interfaces;
the frozen exposure subsequently reconstructed constructor/prototype shapes,
without constructor-static members. LanguageModel, Translator and
LanguageDetector had the same loss.

The projection now uses the same conditional-exposure normalization as capability
feature gating. Retained IDL and Chrome expectations are unchanged. Existing
captured exposure and feature selection continue to decide which constructors
exist; Writer and Rewriter remain absent in the selected profile.

Four already exposed AI interfaces have coherent static availability/create
entry points. Dictionary conversion runs synchronously with Promise rejection on
failure, preserving field order, iterator acquisition and nested expected-input
and simple prompt conversions. Availability resolves `unavailable` because no
model service is attached. Creation respects abort reasons and the intersection
of document/ancestor/container policy; policy denial rejects with NotAllowedError.
An allowed request rejects explicitly with NotSupportedError for the absent
backend. No model/session objects or successful fake creations are produced.

Frozen headful Chrome 152.0.7977.82 observations:

- 98 observations under explicit denied AI permissions policy agree between two
  fresh Chrome profiles and a fresh Mimic binary. They cover all four static
  pairs, reflection, receiver borrowing, invalid/throwing dictionaries, ordered
  fields, sequence iterator getter count, nested expected types/simple prompts,
  BigInt conversion, unrestricted Infinity and abort-before-policy behavior.
- A separate default-policy cross-origin iframe (127.0.0.1 parent, localhost
  child) agrees for all four interfaces: unavailable and NotAllowedError.
- Ordinary top-level Chrome availability can remain pending while its model
  service initializes. The bounded observation timed out; that timeout is not a
  semantic expectation. Unavailable backend state is an explicit Mimic capability
  boundary, not a claim about every machine's top-level Chrome state.

Local receipts: `.build/style-lifecycle-compat-delegated/ai-iterator-final` and
`ai-cross-origin` in the main checkout. The former's probe and Chrome binary hashes
are retained with `internal/browser/testdata/ai_availability_chrome152.json`.
The cross-origin receipt's script hash covers its inline controller/child source;
its generic probe-file hash belongs to the separate dictionary probe.

Regression coverage passes in ordinary and restored bootstrap realms, plus same-
origin backend absence, default cross-origin denial and iframe allow denial.
Existing selected navigator, exposure/catalog, cross-realm and document-all tests
pass. Four generator unit tests pass. The generator's deterministic projection
checks pass; its later full artifact-manifest check is blocked by pre-existing
stale manifest data (unlisted `window-secure-member-order.md`, followed by a
`window-insecure.json` hash mismatch). Only the changed catalog hash is updated.

Boundaries: no model execution/download, no multimodal prompt-content conversion
(structured prompt content explicitly rejects NotSupportedError), no complete
AI worker/backend implementation. This is a bounded correction of publication,
unavailable capability, policy and common dictionary semantics, not full AI API
support. Performance and Trusted Types behavior are outside this change.

Follow-up iterator controls preserve author-thrown TypeError identity across
iterator getters/calls/next and string/number coercion. Context is attached only
to private conversion failures. Frozen Chrome does not call iterator.return when
a yielded value fails DOMString conversion; the regression explicitly preserves
that behavior. Primitive iterator/result diagnostics are covered too.
