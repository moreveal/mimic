# OPFS storage compatibility — 2026-09-12

Scope: the storage rejection/success branch behind DdIVt1/uUOw3 in
`manual-20260912-045102`. The former generated `StorageManager.getDirectory`
rejected because no backing file system existed. This change implements a shared
in-memory origin file system; no capture payloads, filenames, field constants,
or operating-system file APIs participate in production behavior.

## Ownership and behavior

A BrowserContext owns one store per canonical realm origin. Same-origin Pages,
inherited-origin about:blank frames and dedicated workers see the same entries.
Independent origins and contexts have distinct store identities and locks. Each
store has its own mutex; there is no global execution lock. Realm and worker
owners release access locks and discard uncommitted writable streams at teardown.
Context close drops its stores. Deletion releases file bytes and retains small
entry tombstones so stale handles cannot resurrect an entry.

The shared Window/worker binding projects directories, file handles, File
snapshots, directory iteration/resolution/removal, worker sync access handles,
and writable streams onto that state. Sync reads/writes track positions, retain
zero-filled gaps, and implement truncate/flush/idempotent close. Read-only and
unsafe shared access modes coexist as observed; exclusive modes and removal
respect live access locks. Writable streams stage bytes until close, discard them
on abort, and use the existing WritableStream queue/lock implementation.

File handles serialize by private store identity and node identity. The receiving
origin/context must validate that identity before reconstructing its own handle.
The same registration is consumed by structuredClone, messages, history and
IndexedDB. V8 keeps graph aliases, cycles and ECMAScript exotic brands. Platform
payloads are framed alongside native wire bytes: decoder objects are materialized
before ReadValue because V8 forbids JavaScript execution in ReadHostObject. The
native hook itself only returns a prepared local object. Access handles and
writable streams remain non-cloneable. The portable graph fallback uses the same
platform registration, without changing its pre-existing ECMAScript limitations.

## Frozen evidence

Frozen Windows headful Chrome **152.0.7977.82**, V8 **15.2.124.21**,
Chromium revision **1669021**, fresh controlled profile and separate A/B browser
contexts. Loopback secure origin; visible window 1280×800, viewport 1272×653,
DPR 1, not cross-origin isolated. The committed receipt includes actual metadata
and binary/probe hashes. No environment-derived times are equality expectations.

`internal/browser/testdata/opfs_oracle.js` and `opfs_chrome152.json` contain
20 top-level observations, including nested worker cases. Final A/B control and
fresh production comparisons both have **0 differing leaves**. The probe covers
root/entry identity; invalid names; missing/type-mismatched/nonempty/deleted
entries; bytes and MIME; Window exclusion of sync access; cursor/truncate/flush;
read-only/unsafe/exclusive access; closed errors; staged commit/abort; constructor
and iterator reflection; worker termination releasing its lock; inherited frame
origin; cyclic mixed graph clone; getter non-observation; history; IndexedDB;
and alias-preserving Window→worker→Window handle messages.

Private runner, measured binary, full A/B/production outputs, passport and logs
are preserved outside the disposable worktree at
`E:/GitHub/mimic/.build/residual-storage-delegated`. The private target capture is
not copied into committed fixtures. Browser timing and random payload values are
not represented as fixed semantic constants.

## Validation and scope limits

Passed:

```
go test ./internal/browser -run 'TestOPFS|TestStructuredCloneMatchesFrozenChrome|TestHistory|TestWorker|TestIndexedDB' -count=1
```

This includes ordinary/restored-bootstrap frozen OPFS and structured-clone
oracles; focused Goja/V8 Page sharing/context isolation; concurrent store lock and
write checks; teardown staging/lock cleanup; and existing history, worker and
IndexedDB regressions. Final OPFS/clone tests were repeated after the inherited
origin adjustment. No full-suite, race or performance gate was run.

This is an in-memory OPFS implementation, not a claim of complete File System
Access coverage. User-selected physical files, move/watch APIs and durable
cross-process persistence are outside this package. Directory enumeration order
is not asserted. File snapshots preserve captured bytes; invalidation after a
later external writer and the complete WebIDL conversion/error-message matrix
are not covered by these probes. A 1 GiB per-file allocation guard is an explicit
runtime resource boundary, not a Chrome disk-quota model. These limits are not
claimed to explain any remaining private VM branch; the parent replay decides
which residual observations still require investigation.
