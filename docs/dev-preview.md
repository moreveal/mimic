# Live development preview

Start Mimic with `-dev-preview`, then open
`http://127.0.0.1:9222/debug/preview/` in a browser. Select the CDP target;
`?target=<target-id>` selects it directly. Refresh targets after creating or
closing a target. The viewer is an observation surface, not page automation:
drive the real Page over CDP. The mirrored page fills the available browser
window beneath the sticky control header and scrolls inside its iframe. Its
visual viewport follows the viewer window; it does not resize the Mimic target.

The viewer displays the current DOM as an ordinary styled website, using the
viewer's browser for rendering. Mimic does not gain a renderer, GPU requirement,
or Chromium runtime dependency. This is a visual DOM mirror, not a screenshot
or proof that Mimic's geometry matches the viewer browser's layout.

The implementation reuses the existing canonical DOM snapshot projection,
including live form values, declarative shadow roots and adopted stylesheets.
Document CSSOM rules are projected as inline styles with relocated resource URLs.
Attached iframe documents are embedded as snapshots. Scripts, event attributes,
active embeds and form/navigation actions are removed; the viewer additionally
uses a sandbox without script permission and a script-free document CSP. Only
`allow-same-origin` is granted so the parent viewer can patch the mirrored DOM;
`allow-scripts` is never granted. Viewer actions do not drive the Mimic Page or
execute its JavaScript.

The Page command boundary checks existing DOM/resource, CSSOM, focus and viewport
revisions only while a viewer is subscribed. Debug-only form setter tracking
covers value changes that do not mutate attributes. A changed revision generates
one immutable HTML update over a separate WebSocket. There is no additional
polling timer; the existing CDP event loop supplies turn boundaries. A pending
publication coalesces turns for 50 ms without restarting its delay, so continuous
page activity cannot starve the initial snapshot or subsequent updates. A one-item
mailbox replaces stale updates so slow network writes do not block Page execution.
The wire still carries full immutable HTML snapshots. The viewer coalesces pending
updates per animation frame and reconciles the existing DOM by realm/node IDs.
Unchanged attributes, images, styles and nested documents are retained. Scroll,
focus and selection survive ordinary updates; a new document resets scroll.
The iframe gets one empty bootstrap document, not a new `srcdoc` per update.
Snapshot work itself runs under the Page command lock.

Dialog top-layer membership is projected separately from the `open` attribute.
The viewer restores native modal state after reconciling the connected tree, so
`:modal`, backdrops and centering apply without running site scripts in the mirror.
Closing a mirrored dialog also removes the viewer's top-layer membership.

Without `-dev-preview`, debug HTTP/WebSocket routes are not registered, debug JS
is excluded from the realm bootstrap, no debug form tracking is installed, and
no preview serialization, revision reads, resource requests or network messages
occur. The command unlock path has only a nil observer check. Even in dev mode,
without viewers no preview snapshots or version reads are performed. Bootstrap
cache keys include the debug source to keep dev and ordinary realms separate.

Images and fonts are loaded by the viewer browser from their original URLs.
They do not use Mimic's resource loader or credentials; protected resources,
cross-origin font restrictions and Page-local blob URLs may therefore differ or
be unavailable. Canvas/WebGL pixels and live video are not exported. Shadow
stylesheet CSSOM changes and shadow-contained iframe mapping are not fully
projected. Fonts, browser CSS support and layout can differ from Mimic's modeled
observations. This preview does not claim pixel equivalence to Chrome 152.

Keep the debug listener on loopback unless its DOM contents are intended to be
accessible to other users of that listener. Viewer WebSockets enforce same-origin
browser connections; the existing CDP listener's access model is otherwise unchanged.

Browser regression: serve `internal/cdp` with a local static server and open
`testdata/dev_preview_dom_test.html`. It checks repeated updates, node/document
identity, nested iframe and shadow content, scroll, focus, selection, identical
updates, script isolation and navigation.
It also checks modal centering, persistence across updates, closing and reopening.
`tools/compatibility/preview_dom_smoke.py` runs it in a temporary Chrome 152 context.
