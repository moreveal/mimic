# Performance residual: ownership and load registration

Reference: frozen headful Chrome 152.0.7977.82, fresh local A/B profiles; retained
oracle `performance_iframe_ownership_chrome152.json` includes raw observations,
metadata and provenance. Native controls agree. Parent and child timelines each
own their correct entries: the parent has an iframe resource, the child has its
own navigation. Navigation responseEnd is positive. No new Performance API was
added.

The apparent extra navigation in saved rPXg2 was localized to its collector:
first `performance.getEntries()`, then a PerformanceObserver for resource and
navigation. In the diagnostic Mimic run, registration occurred before load ended
(initial entry duration0, DOMContentLoaded167.195ms, load171.185ms). Navigation
was subsequently published once. Frozen Chrome's original collector ran after
load and had no future navigation callback. An independent early/late
registration control produces **two/one** observations in both runtimes. This is
a measured load-registration race, not a duplicate resource entry or a missing
observer suppression rule. Do not deduplicate the application's initial snapshot
against future observations in the browser implementation.

Pdbt7's current remaining differing leaf is the resource duration supplied to its
mark/entry projection. Other entry composition matches. The replay serves saved
responses immediately and does not reproduce wire latency.

The independent control did expose one unrelated existing projection error:
browser-owned fallback favicon timing used initiatorType `img`; frozen Chrome
reports `other`. This is corrected and the existing regression expectation is
updated to the measured result. Explicit icons keep their separate link pathway.

Focused validation covers favicon Resource Timing, early/late navigation
registration and initiating-realm clock ownership. Raw local evidence remains in
`.build/residual-canvas/performance-load-order` and the trace-only Go overlay in
`.build/residual-performance-parent`; the latter does not change production.
