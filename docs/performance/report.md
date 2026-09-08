# Performance architecture pass

Frozen harness SHA-256: `ce1fce42fa9b9e03f105900601db4d6d7fa9b0d9cda51d5096322357277673e7`. Original `benchmark/results` is unchanged.

## 01: Page concurrency

Page-owned command locks replace the server-wide lock. Context bootstrap runs outside the registry lock; shared localStorage and permission delivery have their own synchronization. A network-barrier regression proves an independent Page can evaluate while navigation is blocked.

Full harness: `benchmark/runs/01-page-concurrency`; machine memory pressure stopped density growth at N=1. No N=50 throughput or marginal RAM conclusion is possible. Chrome cold DOM contains one `net::ERR_ABORTED`, retained without retry or exclusion.

Mimic warm medians (ms), mechanically extracted from the unchanged comparator:

| Workload | Metric | Frozen | After | Change |
|---|---|---:|---:|---:|
| static | session_create_ms | 118.28 | 85.87 | -27.4% |
| static | execution_ms | 2.82 | 2.97 | 5.5% |
| static | completion_ms | 127.51 | 90.26 | -29.2% |
| dom | session_create_ms | 95.33 | 78.78 | -17.4% |
| dom | execution_ms | 1560.32 | 1271.19 | -18.5% |
| dom | completion_ms | 1661.46 | 1341.98 | -19.2% |
| react | session_create_ms | 121.60 | 69.43 | -42.9% |
| react | execution_ms | 166.03 | 89.06 | -46.4% |
| react | completion_ms | 302.94 | 168.72 | -44.3% |

These single-session deltas are observations, not causal claims for concurrency: memory pressure and background load differed. Correctness: all 12 browser/workload gates passed; Go suite and CDP race suite passed. Initial browser race run passed (396.302s); subsequent storage/permission changes require the final full rerun.
