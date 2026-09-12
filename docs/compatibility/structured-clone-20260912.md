# Structured clone compatibility, 2026-09-12

`structuredClone(Object(611n))` and `structuredClone(/gul e/)` returned
`undefined`: the Window operation was only a generated exposure stub. They did
not reveal a BigInt/RegExp bug in V8's serializer. History already used the native
serializer, while messaging exported JS values through Go/JSON, independently
losing brands, aliases and cycles. Navigation had a separate graph codec which
omitted boxed primitives.

The public operation, Window/MessagePort/Worker/BroadcastChannel message data and
Navigation state now share a realm-local codec. V8 owns ECMAScript serialization;
only immutable wire bytes cross runtime owners, and decoding creates receiver-
realm values. The existing history path uses that same native engine primitive.
The old handwritten History/Navigation graph duplication was removed; Goja has a
single bounded graph fallback. IndexedDB retains its existing platform-aware
storage codec, which passed the covered boxed/RegExp round-trip. localStorage and
sessionStorage are string APIs, not structured-clone consumers.

ArrayBuffer transfer validates WebIDL sequence conversion before serialization,
reads the transfer iterable once, validates all entries before detaching, and
rechecks detached buffers after author getters run. Worker capture traces retain
an independent JSON diagnostic of the already-cloned value plus the authoritative
wire. Diagnostics do not reread author getters or call author `toJSON` methods.
Non-JSON values retain the wire with an explicit diagnostic boundary.

## Validation

The retained `internal/browser/testdata/structured_clone_oracle.js` and
`structured_clone_chrome152.json` cover 45 cases. Two fresh Chrome browser contexts
against frozen Chrome **152.0.7977.82 / V8 15.2.124.21**, then the freshly built Mimic
binary, produced **0 Chrome A/B and 0 Chrome/Mimic differences**. The headful process
was launched with its startup window hidden; the capture records the actual hidden
visibility and zero outer-window observations, without representing these as
normal visible geometry. These are ECMAScript serialization probes, with no
presentation-dependent expectations.

Covered: Number/String/Boolean/BigInt boxes, RegExp source/flags/lastIndex reset,
ignored author hooks on native builtins, cycles and aliases, Map/Set, shared buffer
views, Date, Error/cause, property descriptors/getter exceptions, proxy rejection,
ArrayBuffer transfer (duplicates, invalid iterable/items, getter-induced detach,
failure atomicity, resizable buffers), whole foreign graphs and borrowed child-
realm cloning, history, Navigation, MessagePort, window.postMessage, Worker,
BroadcastChannel and IndexedDB.

Regression `TestStructuredCloneMatchesFrozenChrome` passes both ordinary and
restored-bootstrap V8 realms. Existing browser tests selected by
`History|Navigation|Message|Worker|IndexedDB|Structured` pass, including Goja worker
lifecycle/fetch/microtasks and readable capture traces. The webapi and V8 engine
packages are also checked. See the compact JSON receipt for binary/probe hashes;
full local evidence is outside the disposable worktree at
`E:/GitHub/mimic/.build/structured-clone-delegated`.

## Measured remaining boundaries

These are separate from the fixed public stub and are not claimed as compatible:

- A local graph containing foreign bridge proxies, e.g.
  `structuredClone({b:frame.contentWindow.eval('Object(8n)')})`, still raises
  `DataCloneError`. Cloning a whole foreign graph and borrowing its clone operation
  pass. Native serialization cannot consume the bridge proxy inside a mixed graph.
- Blob cloning raises `NotSupportedError`; the native platform serialization
  delegate is not implemented. IndexedDB's existing Blob/File codec is unchanged.
- DOMException becomes a plain Error (`name: Error`, empty message), unlike Chrome's
  preserved DOMException. This is an incorrect platform-brand boundary, not a pass.
- WebAssembly.Module cloning raises `DataCloneError`; Chrome clones the module.
  SharedArrayBuffer serialization remains explicitly unsupported and was not part
  of the non-isolated fixture.
- Public transfer support here is ArrayBuffer-only. This does not implement stream,
  MessagePort-in-graph, or platform transfer support. Existing message transport
  transfer-list semantics were not expanded beyond their existing port handling.
- The Goja fallback is bounded and uses script-visible brand/tag observations. The
  V8 frozen result must not be read as an adversarial equivalence claim for Goja;
  poisoned tags/getters and full platform serialization need separate work.

The boundary audit is retained separately in the receipt; its differences were
not converted into passing Chrome expectations or folded into the 45-case pass.
No other VM payload findings or website-specific branches were modified.
