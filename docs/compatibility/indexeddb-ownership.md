# IndexedDB ownership

The Context owns the origin-partitioned database catalog, connections,
transaction queue and immutable committed storage snapshots. It retains no
engine values in records. Storage uses the existing in-memory Context lifetime
and needs no OS provider or native database.

A granted transaction owns one private working snapshot. It publishes its
changes atomically, with no independently synchronized Window record map.
Completion/open-success events run before the host releases the grant. Realm
teardown discards uncommitted state and releases queued jobs and connections.

Requests use the Page's database task source. Transaction activity follows the
creating task or request-event task, including its microtasks; it does not use
timers. The common event dispatcher supplies request → transaction → database
propagation. Unhandled request errors and uncaught listener exceptions abort the
transaction. Explicit abort retains its null transaction error.

Wrappers retain private owner bindings for borrowed operations. Graph encoding
preserves cycles, shared references, sparse arrays, binary views, BigInt,
Map/Set, dates, regexp and Blob/File values. Foreign values are serialized on
their owner. Proxies fail without invoking traps. Internal key paths are kept
separate from exposed mutable arrays; name lists are fresh snapshots.

Frozen Chrome 152.0.7977.82 fixtures are in
`internal/browser/testdata/indexeddb_*`. They exercise storage, indexes,
cursors, upgrades, blocked ordering, cloning, transaction rollback, realm
bindings and snapshot contracts. Go host tests cover Context isolation,
navigation teardown and atomic updates across four concurrent Pages.

The lifecycle oracle explicitly skips Goja because that engine cannot perform
a microtask checkpoint between native listeners. V8 uses the unchanged Chrome
expectation. Storage, realm and concurrency cases run on both engines.
