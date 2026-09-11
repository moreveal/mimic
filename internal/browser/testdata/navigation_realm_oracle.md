# Navigation realm ownership observations

The accompanying `navigation_realm_*_chrome152.json` files were captured from
Chrome 152.0.7977.82, V8 15.2.124.21, in the controlled Windows headful profile.
Each file retains the shared `captureMetadata` and exact local fixture origin.
Reproduce with:

```powershell
.build/fuzz-venv/Scripts/python.exe tools/compatibility/capture_navigation_realm_oracle.py --chrome-cdp http://127.0.0.1:19333
```

The fixture serves identical HTML from `127.0.0.1` and `localhost`, so changing
the host tests an actual origin boundary while keeping document contents equal.
The initial child is `about:blank` and inherits the parent origin.

## Observed ownership boundary

Both same-origin and cross-origin navigation preserve the captured WindowProxy.
Captured Documents, bodies, ordinary objects, constructors, functions and unique
symbols remain usable. Old documents remain mutable and mutations do not enter
the replacement document. Old Document.defaultView becomes null. A retained
function can still read its old `document` binding and return its `window`
binding as the stable WindowProxy.

Saved `eval` is different from a saved user function. After same-origin
navigation, calling the saved eval still evaluates against the old document.
After cross-origin navigation, the measured calls `oldEval('document')`,
`oldEval('window')` and `oldEval('17')` return undefined without throwing. The
test records result type and a primitive case as well as identity comparisons.
The same cross-origin gate returns undefined for a non-string object argument
and for source that would throw; while same-origin those cases return the
original object and throw Error respectively. Saved eval from the last
same-origin document still executes after the iframe element is removed.

Captured parent and child WindowProxy values round-trip through the retained
realm without changing identity. A unique symbol also round-trips; a symbol
property on a retained object remains the same property when both object and
key pass into the replacement same-origin realm. The equivalent attempt to
obtain a new function from the cross-origin current Window throws SecurityError.

These surviving capabilities do not authorize reading the current cross-origin
Window: its document, constructors, arbitrary properties and `in` checks throw
SecurityError; the iframe element's contentDocument becomes null.

There is a separate global-proxy observation: a retained function reading
`globalThis.savedMarker` reads the replacement Window after same-origin
navigation and throws SecurityError after cross-origin navigation. The full
capture preserves this observation. The focused Go ownership test explicitly
leaves this field unasserted while native global-proxy retargeting remains
unsupported; it must not be reported as passing full Chrome equivalence.

## Observed activity boundary

The lifecycle probe invokes the same function with the same parent callback
before and after navigation. While active, queueMicrotask, a fulfilled Promise
reaction and a zero-delay timeout report in that order. Invoked synchronously
after navigation, the function still returns the old Document, but none of the
three callbacks reports during the subsequent 100 ms parent timer.

The callback is a lexical parameter, so this result does not depend on reading
a missing old global variable. The bounded observation does not claim that
all task types or all future scheduling cases are covered.

After iframe removal, contentWindow becomes null, both captured Documents remain
readable, and a retained function from the last document still returns that
document. Both Documents have null defaultView. Full Page teardown has no
surviving JavaScript observer and is checked by internal ownership/resource
tests rather than by inventing a Chrome-facing teardown observation.

## Additional ownership and invocation edges

`navigation_window_edges` retains additional diagnostic observations separately
from the initial enforced corpus. After cross-origin navigation, invoking saved
eval through its `call`, `apply`, or `bind` methods also returns undefined in
Chrome, as do direct invocation and Reflect.apply. The exact capture retains
these fields even when indirect invocation remains an implementation boundary.

The stronger `navigation_nested_window` capture is deliberately split into two
CDP Runtime.evaluate calls: run `_setup.js`, then `_oracle.js` in the same page.
An iframe srcdoc script exposes its own child's WindowProxy. The parent exports
only this WindowProxy, navigates its iframe and removes it before the first call
returns. The second call can still read the nested document, observe null
defaultView, and evaluate `17` in the detached nested window. This exercises
collection between calls without accidentally pinning the ancestor through an
exported eval function. The simpler single-call nested case in
`navigation_window_edges` does not establish this stronger lifetime guarantee.

The exact srcdoc capture remains **unresolved in Mimic**: the existing child
navigation path reads only `src` and never executes this srcdoc script. Its
export is therefore undefined before navigation; resulting TypeErrors must not
be diagnosed as a remaining WindowProxy-retention failure. The enforced
`TestWindowReferenceRetainsDescendantContextAcrossNavigation` loads its ancestor
script over HTTP, explicitly checks that the nested WindowProxy was created,
and uses separate evaluations to isolate the lifetime fix. Srcdoc support is
an independent outstanding capability, not an expectation to normalize away.

Capture these additional cases without changing the earlier captures:

```powershell
.build/fuzz-venv/Scripts/python.exe tools/compatibility/capture_navigation_realm_oracle.py --chrome-cdp http://127.0.0.1:19333 --probes navigation_window_edges navigation_nested_window
```

`navigation_cross_window_reflection` records the exact cross-origin Window
reflection surface independently. Its toString tag is `[object Object]`.
`then`, Symbol.toStringTag, Symbol.hasInstance and Symbol.isConcatSpreadable
read as undefined, while Symbol.iterator, Symbol.toPrimitive and an arbitrary
unique symbol throw SecurityError. The capture also preserves the tag's own
descriptor and complete own-key order; these are separate observations from
access to the current cross-origin document.
