# GitHub document construction and profile loading

## Failures and repairs

Repository hydration reached a sanitizer call to
`document.implementation.createDocument` and failed because the method was
missing. XML/XHTML document construction now uses the Page's canonical node
arena, with independent document ownership, namespace-sensitive root names,
content type and no Window. Chrome 152.0.7977.82 supplied the reference results.

Profile hydration also encountered an unsupported `:modal` selector. Dialog
open/modal state now distinguishes `show()` from `showModal()` and clears modal
membership on disconnection. This implements observable state and selector
matching; it does not implement a renderer, dialog focus management or a complete
top-layer subsystem. XML construction likewise does not imply a complete XML
parser or serializer.

Native CPU profiling identified repeated `getElementById` calls in custom-element
callbacks traversing the profile through millions of JavaScript/Go crossings.
The lookup now traverses the same canonical DOM under one read lock and returns
the first literal ID match in tree order. It does not interpret IDs as selectors
or maintain a second index requiring synchronization. Detached roots, duplicate
IDs, moves, inert documents and template boundaries have regression coverage.

Custom-element reactions now use per-element reaction queues and an invocation's
own pending element list. A nested attribute mutation no longer drains unrelated
siblings' pending connected callbacks. The nested callback order was measured
independently in frozen Chrome 152.

Static `modulepreload` links now start concurrent fetches shared with module
scripts and imports through a realm-owned module response map. HTTP `no-store`
does not cause a second module fetch. Linking and evaluation remain on the Page's
event loop, and teardown cancels/joins the fetches and releases their responses.
Dynamic modulepreload links retain their existing loading path; unifying that
path with the module map remains a limitation.

## Validation

- `go test ./...`: passed.
- Focused XML, dialog, ID lookup, custom-element reentry and module-preload tests
  with `-race`: passed.
- `python tools/generate_compat.py --check`: passed.
- Fast performance gate: all six frozen correctness workloads, concurrency
  waves and teardown/memory waves passed; the frozen harness was unchanged.
- `tools/check_repository.py` still reports existing local build/cache artifacts
  and machine-specific paths in baseline files. Those artifacts and immutable
  baseline data were not rewritten to satisfy the check.

Live repository and profile navigations completed without JavaScript exceptions
or console errors in the recorded run (11.50 s and 12.05 s). The profile previously
exceeded a 35-second diagnostic timeout. The unchanged external snapshot exporter
also saved both pages successfully. The repository export was opened in frozen
Chrome and contained its file list and README instead of the error fallback.
The final-build profile export loaded in 14.81 s and captured in 10.89 s.

The upstream alert SVG still returned HTTP 404; a profile statistics image
returned HTTP 504. These responses remain visible as resource warnings.
The previously reported `Bind must be called on a function` error did not
reproduce in fresh-build BrowserScan runs; its cause is not established by this
change.

[Validation and build receipts](validation.json),
[Chrome document/dialog observations](chrome152-document-dialog.json), and
[Chrome nested-reaction observations](chrome152-custom-reactions.json) retain
compact evidence. Live network timings are observations rather than controlled
performance ratios; see the [performance report](../../performance/report.md).
