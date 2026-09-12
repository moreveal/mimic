# Live browser comparison — 2026-09-12

80 final read-only-probe trials across 16 URLs. These are observations of the retained binaries on this machine and network, not general compatibility rates.

**What the runs establish**

Lightpanda reached readable content sooner on most ordinary pages in this corpus.
Mimic's distinctive result was access through the selected Cloudflare checks,
with substantial latency and CDP responsiveness limitations elsewhere. The
different default browser identities mean these results do not isolate the
reason for Cloudflare's decisions.

- **ScrapingCourse `/cloudflare-challenge`:** Mimic received an initial document
  HTTP 403 with `cf-mitigated: challenge`, followed by HTTP 200 at the same URL
  at **10.39 seconds**. The exact success message, “You bypassed the Cloudflare
  challenge! :D”, was observed at **13.74 seconds** and reconfirmed. Both
  Lightpanda modes and Headless Chrome remained on the challenge after the
  35-second observation window. This is a completed challenge-to-content case.
- **LowEndTalk:** Mimic received the forum with HTTP 200 and observed content at
  **20.88 seconds**. It was not initially served an interstitial. The other
  configurations received challenge 403s and did not reach the forum within
  the window. This establishes differential admission, not challenge solving
  by Mimic on this particular URL.
- **IroShop `/mimic-e2e`:** Mimic transitioned from a challenge 403 to an
  application 404 at **10.32 seconds**. The interstitial cleared, but the old
  requested route does not exist; it is not counted as successful content.
  Other configurations remained challenged. **The IroShop homepage returned
  content in all four configurations.** Path-level results cannot be promoted
  to a claim about the whole domain.
- **BrowserScan:** Mimic obtained the explicit `Normal` verdict, Chrome obtained
  `Robot`, and neither Lightpanda mode produced a verdict within 35 seconds.
  This is a result from one detector, not a stealth guarantee or proof of
  complete Chrome semantics. A loaded detector shell without a verdict is not
  a completed test.
- **TodoMVC:** all four configurations created exactly one local React task
  with the expected text through CDP text input and Enter. This verifies one
  actual interaction, rather than only the presence of server-rendered HTML.
- **Mimic limitations:** GitHub emitted DCL at 28.11 seconds and load at 30.53
  seconds, but complete text extraction did not finish within the probe's
  per-command deadlines. SpigotMC also failed that observation contract.
  Separate exploratory lightweight reads obtained GitHub content at 38.42
  seconds and Spigot text at 40.16 seconds; the latter coincides with the
  configured navigation cap and does not establish a complete lifecycle.
  These are not proofs of permanent website incompatibility.
- **Modrinth:** Mimic yielded a catalog sample at 29.88 seconds, but the required
  later confirmation did not complete. Default Lightpanda initially yielded
  catalog content and then displayed the application's `Error 500: invalid
  argument` page. Resource/CORS-enabled Lightpanda and Chrome retained the
  catalog. The displayed error is not a claim that the main HTTP document
  returned status 500.
- **Amiibo semantic difference:** the same Sandy card text was retrieved in
  every configuration. Mimic kept the document title `Amiibo Character`;
  Lightpanda and Chrome changed it to `Sandy`. Data availability and exact
  browser behavior are separate observations.

**Results: seconds to observed content**

Each cell reports the median of successful trials, with success/attempt count. A single observation is not a statistically established performance claim. Challenge/error pages are excluded from successful content timings.

| Site | Mimic | Lightpanda default | Lightpanda + resources/CORS | Chrome headless |
|---|---|---|---|---|
| example | 0.73 s (1/1) | 0.25 s (1/1) | 0.25 s (1/1) | 0.25 s (1/1) |
| hackernews | 1.58 s (1/1) | 0.91 s (1/1) | 0.98 s (1/1) | 0.82 s (1/1) |
| wikipedia | 1.38 s (3/3) | 0.51 s (3/3) | 0.86 s (3/3) | 0.55 s (3/3) |
| github | not confirmed | 0.51 s (1/1) | 3.46 s (1/1) | 1.42 s (1/1) |
| react | 3.65 s (3/3) | 0.51 s (3/3) | 0.78 s (3/3) | 0.85 s (3/3) |
| amiibo | 0.65 s (1/1) | 0.25 s (1/1) | 0.25 s (1/1) | 0.25 s (1/1) |
| lowendtalk | 20.88 s (1/1) | challenge remains | challenge remains | challenge remains |
| lowendbox | 2.71 s (1/1) | 0.26 s (1/1) | 0.27 s (1/1) | 0.56 s (1/1) |
| spigotmc | not confirmed | 0.53 s (1/1) | 1.04 s (1/1) | 1.28 s (1/1) |
| modrinth | content seen; repeat confirmation timed out | not confirmed | 1.61 s (1/1) | 1.09 s (1/1) |
| mangadex | 2.31 s (1/1) | 1.63 s (1/1) | 1.83 s (1/1) | 1.52 s (1/1) |
| browserscan | 2.09 s (1/1) | not confirmed | not confirmed | 1.28 s (1/1) |
| iroshop | challenge → app 404 | challenge remains | challenge remains | challenge remains |
| iroshop-home | 1.19 s (1/1) | 0.27 s (1/1) | 0.28 s (1/1) | 0.51 s (1/1) |
| todomvc | 1.50 s (1/1) | 0.48 s (1/1) | 0.46 s (1/1) | 0.60 s (1/1) |
| cf-lab | 13.74 s (1/1) | challenge remains | challenge remains | challenge remains |

The three-run series have small samples and visible network/runtime variance:

| Site/configuration | Median s | Min–max s |
|---|---:|---:|
| Wikipedia / Mimic | 1.378 | 1.371–1.388 |
| Wikipedia / Lightpanda default | 0.514 | 0.513–0.515 |
| Wikipedia / Lightpanda + resources/CORS | 0.865 | 0.805–1.059 |
| Wikipedia / Chrome headless | 0.548 | 0.545–0.559 |
| React / Mimic | 3.647 | 3.522–4.038 |
| React / Lightpanda default | 0.508 | 0.507–3.533 |
| React / Lightpanda + resources/CORS | 0.781 | 0.574–2.027 |
| React / Chrome headless | 0.851 | 0.542–0.899 |

The retained decimal precision describes recorded samples, not uncertainty.
No p95 or universal speedup is estimated from three observations.

**Measured contract and limits**

- Same frozen Mimic binary throughout: SHA-256 `88c51622064f982509d1179bc382b561cfe80d476522d7ccb7d62071dad60f9a`. Built near the start from `c3610c3c8042a6ab7c7a94ea710f3f0feb1bb8c0` plus the then-current working tree. Other ongoing user work changed the checkout later; it did not change this executable.
- Lightpanda `1.0.0-nightly.9268+909108e29`, SHA-256 `eb50fc78e575a99ad9b667ffdf7891b188f083f6d93c8411ac33d8809bed1d8b`, Ubuntu WSL. Chrome 152.0.7977.82 and Mimic run natively on Windows.
- Fresh process and profile per attempt; serial browser runs, alternating order by site/round. OS/DNS caches were not flushed, the host was not reserved exclusively for measurement. A separate Windows/WSL trace check observed the same public egress IP, in GE.
- Browser defaults and native identities were retained: Mimic uses its default headful environment profile, Chrome uses headless=new, Lightpanda advertises itself. UA, language, TLS implementation, resource policy and Windows/WSL differ. This compares usable configurations, not isolated engine implementations under identical fingerprints.
- Lightpanda default disables iframe/image/stylesheet/Worker loading. The second mode enables all four and experimental CORS. This still does not establish identical resource behavior to Chrome. No proxy rotation, imported clearance, CAPTCHA solving or challenge-specific runtime modifications were used.
- CDP observations use Runtime.evaluate with read-only DOM traversal, excluding script/style/noscript/template text. No cloneNode, DOM removal or DOM writes occur in the final content probe. Polling is nominally 250 ms, with a 3-second per-command deadline. Times include CDP and extraction overhead; fast results hit the polling floor.
- The external `perf_counter` navigation clock starts immediately before `Page.navigate`, after browser/Page creation and CDP setup. Startup and Page creation are excluded from the table. No browser virtual clock is used. Retained DCL/load numbers are the first observed events and may describe a challenge document rather than the final application.
- Content requires the recorded selector/text criterion and no observed challenge or HTTP error. It is reconfirmed at least three seconds after first observation. This does not prove continuous visual stability, full hydration or every application workflow. DCL/load timings and errors are retained separately.
- BrowserScan final trials additionally require an explicit Test Results verdict. TodoMVC separately creates one local task using DOM focus, CDP insertText and Enter, then checks its exact text and count. Its action timing includes an intentional 500 ms observation delay and is not an input-speed benchmark.
- The final general observation window is 35 seconds. Mimic also has a 40-second navigation cap. A separate exploratory lightweight-probe diagnostic used 45 seconds; those times are not mixed into the table.
- CPU, throughput, per-page memory and teardown retention were not measured in this campaign. Request/error counts are observations, not proof that each runtime implements CDP reporting identically.
- Browser processes are deliberately terminated after observation. The raw process exit code recorded after cleanup must not be interpreted as a crash count. All experiment listeners were absent after the runs.

**Exploratory-probe correction**

The first campaign cloned the body before extracting text. On Amiibo it coincided with 117 repeated image requests in Mimic and 121 in resource-enabled Lightpanda; the final read-only probe records one image request in each, as in Chrome (default Lightpanda records none). Therefore the first campaign is retained only as diagnostic evidence and is excluded from the final timing table. The short Amiibo/CF success pages and actual LowEndBox/Spigot markup were used to calibrate readiness before the final campaign. The initial English-only challenge detector also missed a localized Chrome challenge; final trials additionally inspect cf-mitigated and localized titles.

**Evidence**

- [Per-trial CSV](live-browser-comparison-20260912.csv) and [per-trial JSON](live-browser-comparison-20260912.json).
- [Compressed final raw evidence](../../.build/live-browser-comparison-final-20260912/evidence.zip), excluding Chrome profile directories.
- Final raw samples, events, logs, manifests and the frozen runner are retained in `.build/live-browser-comparison-final-20260912/`. The executable remains `.build/mimic-live-comparison-20260912.exe`.
- Exploratory receipts remain in `.build/live-browser-comparison-20260912/` and `.build/live-browser-comparison-light-probe-20260912/`. They must not be pooled with final timings.
- `tools/compatibility/live_browser_comparison.py` reproduces a campaign. Never reuse an output directory for a different binary or probe: existing trial receipts are skipped.
