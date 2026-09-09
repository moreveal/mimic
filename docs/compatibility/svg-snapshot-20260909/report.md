# SVG snapshots and detached image loading

SVG fragment parsing now passes the element's namespace and qualified name to
the existing `golang.org/x/net/html` parser. Self-closing SVG shapes remain
siblings; foreign-content integration points still create HTML descendants.
Serialization preserves SVG element names, case-sensitive attributes and parsed
qualified attribute prefixes instead of projecting them as ordinary HTML.

Namespaced attribute values and their namespace metadata live in the canonical
DOM. SVG `use` links set through `setAttributeNS` survive snapshot export, and
namespace lookup, removal, Attr metadata and cloning retain that association.
The existing attribute map cannot represent two attributes with the same
qualified name in different namespaces; such a collision is explicitly
unsupported rather than silently overwriting one of them. This is not a claim
of complete XML or SVG API coverage.

Setting an image's `src` starts its document-owned request even while detached.
Updates in one task coalesce, replacement requests cancel their predecessors,
and obsolete completions do not dispatch events. Insertion does not initiate a
second request. Requests started before document load participate in the same
load-blocker accounting as other resources. Teardown cancels and joins requests.
This retains the existing transport-level image loading boundary; full image
decoding and intrinsic-size behavior are outside this change.

Image transport starts use ordinary resource tasks, after priority script
starts. Assigning them the lowest task priority let unrelated timer work delay
the observed background request by 14.6 seconds even when its response was
already cached. The repair does not suppress third-party scripts or identify
particular websites.

## Snapshot readiness

`load` is not an application-hydration completion signal. Capture remains a
projection of the current DOM, not a hidden wait or an extra page execution.
`compatibility/save_snapshot.py` now accepts `--wait-selector` and `--timeout`
so callers can identify the state they want to save. It polls a boolean through
the existing CDP evaluation API without requiring remote DOM object handles.
The standalone desktop exporter has the same optional readiness control.

## Verification

The SVG fixture in `TestSVGFragmentNamespacesAndSnapshot` was independently
executed in Chrome 152.0.7977.82 and returned `ok`. It covers sibling paths,
gradient/attribute casing, a foreignObject integration point, namespace lookup,
stable Attr identity and prefix, removal, cloning and portable output.
The detached-image regression checks coalescing, one request after insertion,
and image-before-window-load ordering.
Replacement-request coverage verifies that an in-flight request is canceled
and cannot emit an obsolete event. All Go tests passed; focused SVG, attribute
and image tests also passed with the race detector. Pinned compatibility
metadata validation passed. The fresh-build fast gate passed all six frozen
correctness workloads, concurrency waves and memory/teardown waves.

[Chrome fixture output](chrome152-svg.json), [export checks](export-validation.json)
and [performance receipts](performance.json) retain compact evidence.

BrowserScan was captured only after its background class and result appeared.
The exported file was then opened in the same frozen Chrome at 1170 x 1300:
the full logo, UA lettering, Navigator dots, CDP interior line, status marks and
background are present. There are zero empty use references and zero failed
image elements. The automated Chrome reference reports its own WebDriver state;
that differing result and variable advertisements are not visual-export defects.

The observed successful export took 1633.63 ms to load, 767.65 ms to reach the
selected state and 1167.56 ms to capture (3608.11 ms total before teardown).
These are live-network observations, not a controlled performance comparison.
An earlier fixed-delay capture took 36.36 s and still missed the background;
fixed sleeps do not establish readiness. BrowserScan teardown also stalled in
one diagnostic run and required stopping that diagnostic process; this batch
does not claim that teardown or all live-site latency issues are resolved.
