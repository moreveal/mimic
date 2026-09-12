# Permission descriptors against frozen Chrome 152

Reference: headful Chrome 152.0.7977.82, two independent fresh profiles,
local HTTP loopback fixture. Probe and observations are retained in
`internal/browser/testdata/permission_descriptor_*`; the observation passport
contains binary hashes, arguments, geometry, security context and source revision.
Private raw controls: `.build/residual-permissions/after`.

The previous implementation deferred dictionary reads to a microtask, invoked
an author Proxy `has` trap, and exposed descriptor names as status names.
It also ignored clipboard permission flags when choosing the permission source.
The general conversion now runs synchronously while failures reject the returned
Promise; specialized dictionaries perform their observable second conversion.
Status names project the selected browser permission, while state and change
events continue to use the Context-owned Environment-backed permission store.
Push shares notifications, and unrestricted clipboard writes share clipboard-read.
The frozen profile defaults gesture-free fullscreen permission to denied.

Validation: both Chrome controls and the resulting Mimic observation have zero
differences across the name, descriptor, conversion, exception and clipboard flag
matrix. Focused ordinary/restored oracle tests pass; an additional regression
checks alias state changes and event delivery. No capture field or hash is used
by production code.
