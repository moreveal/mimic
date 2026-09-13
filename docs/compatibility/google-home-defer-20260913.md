# Google homepage deferred script execution — 2026-09-13

Follow-up to `8065bbd` (`fix(browser): repair form validation, script ordering
and request compatibility`). That commit preserved the previous work before
this independently scoped parser correction.

## Root cause

The two remaining homepage exception notifications represented the same
`TypeError: Cannot read properties of null (reading 'getAttribute')`, recorded
by both evaluation and script diagnostics. A temporary native V8 stack capture
located the call chain `_.Wg` → `_.Zg` → `_.lsa`; the caller supplied
`document.body`. The external classic script was parser-inserted in the head
with `defer`, but Mimic executed it immediately, while the body was still null.
The stack instrumentation was removed after diagnosis.

## Correction

Parser-discovered external classic `defer` scripts start fetching immediately
and execute in document order after parsing, before DOMContentLoaded. They
share the existing deferred module queue on the owning Page event loop.
Inline classic scripts ignore `defer`; `async` takes precedence when both
attributes are present. Fetch failures dispatch error and do not strand the
queue; successful fetches dispatch load even if execution throws. Classic
scripts retain currentScript identity and microtask checkpoints.

The common parser stream covers top-level navigation, child navigation and
document.write. Deferred completion after document.close is posted as a Page
task so a cross-realm caller unwinds before callbacks execute. Deferred
external scripts cannot implicitly replace the parsed document through a
destructive document.write. No Google-specific conditions or identifiers were
added to runtime code or regression fixtures.

## Evidence

A loopback fixture held the first deferred response while later resources
loaded. Frozen Chrome 152.0.7977.82 and the corrected Mimic produced the same
sequence:

```
tail
slow:interactive:true
micro:slow
load:slow
module:interactive:true
fast:interactive:true
micro:fast
load:fast
DCL
```

Before the correction, the classic scripts ran before `tail`, observed
`loading` and a missing body, and lacked their load callbacks. Regression
tests additionally cover iframe/document.write ordering, parallel fetches,
404 and throwing scripts, currentScript, and destructive writes.

Two fresh live runs used `.build/mimic-defer-final.exe`, new browser processes,
and no injected application patches. Google homepage reached `complete` in
both. In the first, Sign In followed by the reserved test address produced
“Не удалось найти этот аккаунт.” After the final iframe timing correction and
rebuild, the second run instead reached Google's browser-rejection page.
Both entire runs recorded zero `Runtime.exceptionThrown`, zero
`Network.loadingFailed`, and zero requests containing `jserror`. The original
exceptions are eliminated, but stable Google admission is not established;
the difference in server verdict must not be attributed to the small timing
change without independent evidence. No complete account login was attempted.

Private diagnostic captures remain untracked under
`compatibility/private-captures/`: `google-home-native-stack2-20260913-mimic`,
`defer-oracle-20260913-{chrome,mimic}`,
`defer-final-oracle-20260913-mimic`, and
`google-signin-20260913-defer-final-mimic` and
`google-signin-20260913-defer-verified-mimic`. They may contain application bodies
and session data and are not included in the commit.

## Known boundary

Validation: `go test ./... -count=1 -timeout=15m` passed (browser package
556.860 s; CDP 32.466 s). The final iframe timing guard was additionally
checked with the deferred, frame-navigation/load and navigation-performance
tests after the full run had started; that focused run passed in 13.708 s.
Logs are `.build/deferred-full-tests.log` and
`.build/deferred-frame-final-tests.log`. `git diff --check` passed.

An additional Chrome probe found a pre-existing readyState discrepancy for
document.open/write/close on an initial about:blank iframe: Chrome retained
`complete` while the deferred script and subsequent lifecycle events ran;
Mimic's existing document.open model resets it to `loading` and advances to
`interactive`. The new document.write regression asserts script/event order
and body availability independently of that separate state-model issue.
The navigation tests assert the measured readyState values exactly. This
change does not implement the remaining asynchronous parser-script or
document.write module capabilities.
