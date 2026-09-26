# Google Search response opt-in investigation — 2026-09-26

## Confirmed shared network defect

Google's intermediate Search response sent `Accept-CH` across multiple header
field lines. The first line requested `Sec-CH-Prefers-Color-Scheme`; later
lines requested high-entropy UA hints. The loader used `http.Header.Get`,
which returned only the first line. Consequently, the next same-origin
navigation omitted the accepted UA hints.

The loader now combines all `Accept-CH` field lines before updating the
canonical context-owned session. There is no Google-specific dispatch or
profile override. Focused regressions cover ordinary next-request opt-in and
`Critical-CH` restart with a leading unsupported hint and separate UA hint
lines. The existing redirect and iframe permission checks also pass.

## Live result and limits

A freshly built Mimic sent the accepted architecture, bitness, form factors,
full version, full version list, model, platform version and WoW64 fields on
the second Search navigation. It still received a redirect to `/sorry/index`
and HTTP 429, with no JavaScript exception notifications. Fixing this defect
does **not** establish Google admission or explain the remaining rejection.

The user's ordinary Chrome 154.0.8037.57 and incognito HARs both contain
successful Search HTML. The ordinary capture is a reload in an existing
session. The incognito capture starts at a Search request already containing
`sei`; it does not contain the preceding intermediate document. These are
useful successful network references, but they do not yet establish a matched
comparison of the intermediate program's execution. Frozen Chrome 152 was
not launched again in this investigation.

Private HARs, captures and extracted response bodies remain untracked under
`.build/google-serp-success-20260926/`. Their SHA-256 hashes are:

- Ordinary Chrome HAR: `ea5c4a1410014c947295352fe25168b819bec28b7b3ea421eb3de66f42ae9327`.
- Incognito Chrome HAR: `a6d07d8df9d53e5cd0270c1f06ada78be4fa3cbff2e47d3720f7084db8a06cea`.

No cookies, account identifiers, session tokens or captured application
bodies are included in public project files. Temporary header diagnostics
were removed. The full local test suite was not run.
