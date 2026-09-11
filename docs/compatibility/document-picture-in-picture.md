# Document picture in picture

The service creates an auxiliary top-level Frame in the opener Page's execution
agent. It has no DOM owner or parent frame. Its own Realm, document, WindowProxy,
viewport and origin use the existing browser state and cross-realm bridge. The
opener relation is distinct from the document that owns its lifetime; navigating
the opener closes the auxiliary context even after the opener's current Realm
has changed. No native window, renderer or OS API is required.

The service consumes transient activation without clearing sticky activation.
Creation resolves through the Page task queue after the trusted enter event.
Window.close marks closed immediately; the queued document teardown dispatches
pagehide/unload, clears the service window, and retires its Realm through the
existing retained-reference collector. Page teardown disposes all its realms.
Auxiliary history has zero traversal entries; push/replace still clone and retain
state and may update the about:blank fragment without changing opener history.

Frozen Chrome 152.0.7977.82 local oracle is checked into testdata as
`document_pip_oracle.js` and `document_pip_chrome152.json`. It checks validation,
activation, event order/trust, canonical objects, separate realm intrinsics,
inherited base/referrer, document mutation and close lifecycle. The private
capture is `.build/pip/oracle-07` in the isolated implementation worktree.

Requested sizes are viewport hints bounded by the environment screen, stored in
Go per auxiliary context. Native outer decorations, placement and window-manager
size overrides are environment observations: the local frozen headful Chrome
returned 1272x762 content in a 1280x800 outer window even for a 420x260 request.
Those OS-controlled values are not baked into the API or its semantic oracle.
