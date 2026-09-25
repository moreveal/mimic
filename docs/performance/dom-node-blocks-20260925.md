# Rejected DOM node-block experiment (2026-09-25)

A removable candidate placed canonical `Node` objects in fixed 16-record blocks.
The existing ID-to-pointer map and public `Node` shape stayed unchanged. The
[compressed patch](../../tools/performance/pocs/dom-node-blocks-rejected.patch.gz) applies to
the feature commit `4f4e5ef` but is **not part of production**.

In the DOM-heavy [diagnostic](../../tools/performance/dom_density.go), five
documents with 10,000 `div`/`span` pairs each used 73,838,192 B median live Go
heap on the control and 71,506,576 B on the candidate across three alternating
fresh-process pairs: **-2,331,616 B (-3.16%)**. Cumulative Go allocation fell
2.22%. The parser benchmark dropped from 927 to 746 allocations per batch parse
and 4770 to 4586 per stream parse. A 32-record variant was rejected earlier
because its block tail grew allocated bytes per document; 16 records improved
both bytes and allocation count.

The small-DOM 100-Page fixture restored 100/100 Pages in both builds, but median
first-live process RSS changed by less than 1 MiB. Five control fast gates passed.
Of five candidate gates, one stalled during a 25-Page static wave: all 25 jobs
timed out after 30 seconds, and teardown timed out. The other four candidate
gates passed. This is **not** evidence that the block allocator caused the stall,
but it fails the required correctness gate. The experiment was removed rather
than accepted on a Go-heap measurement alone. The matched memory receipts and
binary hashes are in [provenance](data/dom-node-blocks-20260925/provenance.json).

The selected follow-up changes only the `Node` field layout; see the
[layout checkpoint](dom-node-layout-20260925.md). A larger hot/cold split remains
open and would need separate compatibility and process-memory validation.
