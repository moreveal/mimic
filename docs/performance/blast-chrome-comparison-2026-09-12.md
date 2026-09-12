# Chrome comparison for blast-test.py

Date: 2026-09-12. Chrome/152.0.7977.83, separate headless profile; existing Mimic server at 127.0.0.1:9222. Three alternating Chrome/Mimic runs on the live site, reusing each browser page and caches. Original script unchanged; wrapper overrides only CDP endpoint and records method durations. Chrome profile starts fresh; Mimic profile was already running. This is a live workload comparison, not an isolated CPU benchmark.

All first-run outputs: 5 categories, 27 forums, 39 subforums.

| Median duration (ms) | Chrome | Mimic |
|---|---:|---:|
| Navigation | 1826.98 | 4093.81 |
| Title | 35.70 | 31.07 |
| Browser info | 15.13 | 2343.64 |
| Forum parsing | 8.15 | 15.19 |
| Statistics | 7.23 | 26.56 |
| Total | 1952.14 | 6522.19 |

Mimic total durations: 6698.774, 6522.193, 5756.161 ms.
Chrome total durations: 3139.313, 1952.138, 1590.201 ms.

The multi-second browser-info delay reproduces only in Mimic here. Earlier scheduler trace showed a 2543 ms timer task overlapping the same call. Navigation includes network and script execution; these measurements do not isolate their individual contributions.
