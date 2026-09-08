# Fast performance iteration gate

Run with the frozen benchmark Python environment:

```
python tools/performance/fast_gate.py --output .build/fast-gate-UNIQUE
```

`--source` selects a source checkout for a before/after comparison. Each invocation builds its own executable into a new output directory. `build.json` records the build command, source revision/status, executable SHA-256 and frozen harness/workload hashes. Every process launch verifies and records the executable hash before invoking the frozen Runtime with that exact absolute path. A mismatch aborts immediately. Existing output directories are refused to preserve evidence.

The gate imports `benchmark/run.py`; it does not edit the frozen runner or workload fixtures. All six workload result checks are mandatory. Timings cover five measured warm iterations of DOM, static and React, then three measured static waves at N=10 and N=25. Each timing series also has one excluded warm-up, following the frozen suite. Memory is sampled with ten live static pages and ten live React pages in separate fresh processes, before teardown, immediately after teardown and after the same 250 ms recovery interval. No forced garbage collection is added. Concurrency retains the frozen memory-pressure and paging limits. Raw rows, failures and warm-ups are retained.

Run repository correctness regressions for each production change; this gate supplements those tests. Use the full frozen matrix only at a substantial improvement milestone. Fast gate timings cannot replace the original full-suite baseline or certify unmeasured N=50 targets.

Check executable mismatch handling with:

```
python -m unittest discover -s tools/performance -p test_fast_gate.py
```


At a substantial improvement milestone, use the full wrapper:

```
python tools/performance/full_gate.py --output benchmark/runs/UNIQUE-MILESTONE --chrome PATH-TO-PINNED-CHROME
```

It runs the unchanged full matrix. Its Runtime subclass verifies the just-built Mimic executable before every launch and also verifies the pinned Chrome executable against its initial hash; `launches.jsonl` records each check. Both wrappers reject a harness fingerprint that differs from the original baseline. The original benchmark files remain untouched.
