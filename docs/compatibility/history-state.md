# History state storage

The Chrome 152 audit (`audit-20260911-3faacc6/relations/report.json` under private
captures) established that pushState/replaceState copy input and nested identity,
preserve repeated history.state identity, and reject functions with DataCloneError.

Both operations now copy before committing URL/state/length. The private storage
graph is never exposed; a second graph provides the cached history.state value.
Same-document traversal reconstructs that value, so application mutations of a
previously returned state cannot alter the stored entry. This remains a bounded
realm-owned graph representation, not a durable serialized session/BFCache format.

V8 uses its ValueSerializer/ValueDeserializer for ECMAScript values, cycles,
shared graph references, Map/Set, Date/RegExp, buffers/views and BigInt. Native
proxy/function rejection avoids invoking proxy traps. A serializer host predicate
rejects DOM, Window and Event objects even inside nested objects or collections.
SharedArrayBuffer and WebAssembly.Module are rejected for history storage.
Exceptions from application getters retain their original identity; copying does
not execute getters twice. Failed copies leave the entry and URL unchanged.

Goja uses a bounded graph-copy fallback for ordinary objects/arrays, Date/RegExp,
Map/Set, ArrayBuffer/views and BigInt. Native proxy detection rejects proxies
without traps. Other branded objects remain unsupported in that fallback.

Blob/File slots now explicitly reject with NotSupportedError, including nested
values; Chrome supports their serialization, so this is an unsupported boundary,
not claimed agreement. Before that guard V8 silently copied a Blob to an empty
object. ImageData, CryptoKey and other serializable browser brands still need
dedicated hooks and validation; the native ECMAScript serializer alone cannot
establish their browser semantics or reject every unsupported platform object.
Cross-document restoration and full PopStateEvent semantics remain separate gaps.

Tests: TestHistoryStructuredState (Goja/V8), existing History/FrameHistory tests,
and TestHistoryCloneRestoredBootstrap (ordinary and explicitly warmed snapshot).
