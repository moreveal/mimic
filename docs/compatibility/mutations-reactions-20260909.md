# Mutation records and custom element reactions (2026-09-09)

This domain pass uses production workloads only for prioritization. No GitHub
selector, URL, component name, or response shape participates in implementation.
Canonical nodes remain host-owned; observer registrations and reaction queues are
realm-local semantic state.

## Oracle and scope

The unmodified WPT sources come from Chromium commit
`d04cdb24d67b081f6cf80200ffc5233f44b61109`, WPT tree
`5251311032215fe14bbd74cd0867e14cb4b58e3f`. The oracle is headful Chrome
152.0.7977.82. `compatibility/domain_tests.py` verifies cached bytes against pinned
upstream and records source hashes, executable hashes, and Chrome metadata.
Local Windows endpoints additionally verify listener executable identity, since
hashing a file alone cannot establish which process answers the endpoint.

The runner disables only the testharness HTML reporter using its standard
`setup({output:false})` setting. It does not rewrite test assertions. An incomplete
or failed harness contributes no authoritative passes. Observed subtest results
are preserved separately to identify domain gaps.

## Coverage

| Pinned subset | Before observed pass | After observed pass | Chrome pass | Limitation |
|---|---:|---:|---:|---|
| MutationObserver, eight files | 82/129 | 101/129 | 128/129 | After: 22 Range failures, five Attr/namespace timeouts, one CDATA not-run |
| Custom Elements, eight files | 61/189 | 102/189 | 184/189 | Remaining cases primarily alternate Documents, cross-realm registries, document.write, customized built-ins, Attr/namespaces |
| Independent single-realm reaction/record probes | 1/10 | 12/12 | 12/12 | Batch mutation and token-coercion probes added after the initial ten |

The MutationObserver after capture has six completed successful harnesses out of
eight and **61 authoritative passes**. The 101 observed passes include successful
subtests from two files whose other tests time out. They must not be presented as
101 completed-harness passes. All eight Custom Elements harnesses complete.

The oracle itself fails one ProcessingInstruction oldValue assertion and five
Custom Elements assertions (including a newer form callback and constructor
Proxy property reads). These are recorded as oracle differences rather than
silently treating every WPT expectation as Chrome's actual behavior.

## Completed semantic layer

- Observer registration, option inheritance/conversion, records, disconnect,
  takeRecords, and transient detached-subtree observation until notification.
- Read-only MutationRecord prototype getters and static canonical NodeLists.
- Notification ordering checked against pinned Chrome, including the microtask
  boundary and observer creation order.
- One mutation transaction for fragment insertion, same-parent moves,
  replacement, textContent, append/prepend, and replaceChildren. Nested internal
  insertion/removal steps do not duplicate public records.
- Custom-element definition callback snapshots, callback conversion validation,
  construction/upgrade state, reactions after mutation, and shadow-inclusive
  lifecycle traversal. Constructor mutations preserve the upgrade-time reaction
  snapshot.
- DOMTokenList token validation and mutations use the shared attribute path;
  null-namespace attribute aliases use canonical ordinary attributes.

The independent probes exercise the generic semantics without WPT's dependencies
on document.write, iframe documents, or XML construction. Their pinned expected
results also run in `TestMutationReactionChrome152`.

## Remaining boundaries

This pass does not implement Range, Attr/NamedNodeMap mutation, full namespace
storage, CDATA/XML documents, alternate Document ownership, cross-realm custom
registries, customized built-ins, form-associated custom elements, or the complete
parser reaction machinery. Non-null namespace handling remains the existing
unsupported path. Those are explicit coverage boundaries, not evidence of full
DOM or Custom Elements conformance.

Benchmark validation is owned by the enclosing compatibility change. A fresh
DOM native profile was taken before this mutation-coordination change at
`.build/hydration-complete/mutations-profile-before`. No frozen workloads or
benchmark baselines were edited.

## Evidence

Local artifacts are under `.build/hydration-complete/`:

- `mutations-wpt-{baseline,oracle,after2-verified,after3}.json`
- `custom-elements-wpt-{baseline,oracle,after2-verified}.json`
- `mutation-probes-{baseline,after2-verified,after3}.json`
- `mutations2-process.json`, `mutations2-hash.json`, `mutations3-hash.json`

The files named `mutations-wpt-after2.json` and `mutation-probes-after2.json`
without `verified` are explicitly marked invalid: a port collision connected
that attempt to another executable. They are not evidence for the change.
