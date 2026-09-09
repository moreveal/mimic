# Private native captures

`capture_native.py` records a new disposable browser context in an already running
Chrome CDP instance. It does not launch the browser, inspect existing tabs or
storage, or close the browser. Only the new context and its recursively attached
iframe/worker targets are recorded; the context is disposed at the end.

Install Python's `websockets` package, start the intended frozen Chrome separately,
then provide its verified local debugging port and an explicit destination URL:

```text
python tools/compatibility/capture_native.py compatibility/private-captures/run-name 9343 https://example.test/
```

Use a new output directory for every attempt. The targeted gitignore rule keeps
`compatibility/private-captures/` private: response bodies, POST data, storage and
cookies may contain credentials. Never promote raw captures into tracked fixtures.

The recorder saves request/response events and headers, available POST data,
decoded response bodies, parsed script sources including eval and worker scripts,
console/runtime events, final DOM snapshots, screenshot, owned-context storage and
cookies. Durable network messages retain bodies across process swaps. The manifest
indexes sources and bodies and hashes finalized files. Inspect error responses in
individual artifacts as well as transport/time-out errors in the manifest.

Browser-wide Tracing is omitted to avoid recording unrelated tabs. Raw TLS bytes,
deterministic random seeds and replayable server decisions are not captured.
Tokens expire, and the output is not a guarantee of deterministic offline replay.
No fresh-process or profile hash is claimed when attaching to an existing browser.

For the private iroshop capture made on 2026-09-09, successful **Cloudflare passage**
means the initial HTTP 403 `cf-mitigated: challenge` was followed by the application
document without that header and a `cf_clearance` cookie. The requested
`/mimic-e2e` route then returned a genuine Next.js 404; this is not an application
HTTP-200 success. The retained successful attempt has zero response-body,
script-source or POST-data retrieval errors and explicitly omits global Tracing.
