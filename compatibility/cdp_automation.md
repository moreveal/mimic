# Real automation-client CDP gate

`cdp_automation.py` runs the same 34 workflows through **Pyppeteer 2.0.0** and
**puppeteer-core 25.10.0** against an existing browser endpoint. It starts its own
controlled loopback HTTP fixture and a transparent WebSocket recorder. Client
behavior is neither mocked nor patched. The modern client uses flattened
tab/page sessions; Pyppeteer uses legacy nested Target messages.

The workflows cover connection/version, page creation, evaluation and promises,
remote handles and identity, property inspection and disposal, special values,
evaluation exceptions, uncaught timer errors and console arguments, selectors and delayed mutation waits, forms,
keyboard and pointer input, initial scripts and removal, exposed host functions
across navigation, setContent, navigation/reload/history/frame events/network
idle, response bodies, cookies, headers, interception fulfillment/POST overrides/
abort, viewport and user-agent coherence, simultaneous pages and isolated browser
contexts. There are **68 behavioral checks**, not 68 independent protocol methods.

## Recorded checkpoint

On 2026-09-12, the freshly built Mimic candidate with SHA-256
`9911b66a404f5c9c4445322193be432a0517912ee096d8b292c885d8ca7a4315`
matched the retained headful Chrome 152 reference in all 34 Pyppeteer checks,
all 34 Puppeteer checks, and all 16 interception scenarios. Two-session event
gating and delayed-request network idle were also inspected against Chrome.
Paint lifecycle observations are outside this runtime's rendering boundary.

The local captures are `.build/cdp-automation-candidate5/report.json`,
`.build/cdp-automation-interception-candidate5/report.json`, and
`.build/cdp-automation-events-candidate5/report.json`; these ignored diagnostic
files are reproduced by the commands below. The measured Chrome references and
their capture provenance are retained in this directory.

## Setup and execution

From the repository root on Windows:

```powershell
python -m venv .build/cdp-venv
.build/cdp-venv/Scripts/python.exe -m pip install -r compatibility/cdp_automation_requirements.txt
npm install --prefix .build/cdp-clients --ignore-scripts --save-exact puppeteer-core@25.10.0
go build -o .build/mimic-cdp.exe ./cmd/mimic
```

Use Node 22.12 or newer. Both clients reject unexpected installed versions.
The runner requires a **fresh controlled, headful Chrome 152.0.7977.82** endpoint
for the reference. Follow `docs/oracle-policy.md`; no headless or feature-override
launch is substituted. The runner owns and closes its pages and contexts, never
the externally started browser. Use a dedicated browser/profile for captures.

```powershell
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation.py `
  --endpoint http://127.0.0.1:9333 --oracle `
  --output .build/cdp-oracle/report.json

# In another terminal: .build/mimic-cdp.exe -listen 127.0.0.1:19222
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation.py `
  --endpoint http://127.0.0.1:19222 --binary .build/mimic-cdp.exe `
  --reference .build/cdp-oracle/report.json --output .build/cdp-mimic/report.json
```

`--clients pyppeteer` or `--clients puppeteer` selects one client. `--node` and
`--puppeteer-module` can select isolated installed runtimes. `--allow-failures`
is for exploratory captures: it changes the exit code, never the recorded result.
The default exits nonzero on failed or blocked workflows, worker failure, or any
unverified/different result when a reference is requested.

Each output directory contains:

- `report.json`: target identity, optional tested binary SHA-256, client/source
  versions, oracle provenance, check values/errors, method/event counts and
  differential classifications;
- per-client JSON results and `*.protocol.jsonl`: actual requests, responses,
  errors and events, including decoded legacy inner messages;
- client stderr logs when diagnostics were produced.

Protocol summaries distinguish calls, received responses, actual protocol
errors, and unanswered calls. IDs are scoped to connection and session, so nested
or flattened sessions cannot overwrite each other's accounting. Unreached checks
are `blocked`. Identical Chrome/Mimic failures are `reference-failure`, never a
compatible pass. Raw captures retain origins and protocol payloads; comparisons
use intentionally bounded behavioral results. Only controlled fixture data is
sent to this recorder.

`cdp_automation_reference.json` retains the measured 68-check Chrome reference;
`cdp_automation_interception_reference.json` retains the 16 interception probes.
Use either as the reference for its corresponding measurements. They preserve
capture provenance, origins, observed values and protocol evidence. Regenerate
them only from a successful fresh oracle capture:

```powershell
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation_report.py `
  --source .build/cdp-oracle/report.json --output compatibility/cdp_automation_reference.json
```

The same compactor accepts an interception capture. It refuses unprovenanced,
wrong-version, headless or failed references.

## Additional differential probes

```powershell
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation_events.py `
  --endpoint http://127.0.0.1:9333 --oracle --output .build/cdp-events/oracle.json
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation_interception.py `
  --endpoint http://127.0.0.1:9333 --oracle --output .build/cdp-interception/oracle.json
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation_interception.py `
  --endpoint http://127.0.0.1:19222 --reference compatibility/cdp_automation_interception_reference.json `
  --output .build/cdp-interception/mimic.json
.build/cdp-venv/Scripts/python.exe compatibility/cdp_automation_test.py
```

The event probe measures two sessions attached to one page, disabled/enabled
Page/Network/Runtime domains, Runtime context snapshots and network idle with a
request that remains active after load. Receipt time is separate from protocol
timestamps: Chrome's idle timestamp identifies the quiet boundary, not its later
event-delivery time.

The interception probe measures 16 cases: omitted/empty/default patterns,
request/response stage, URL glob/resource filters, per-request response overrides,
disable while paused, legacy HeadersReceived interception, and debugger detach
while a request is paused. Chrome's omitted Fetch patterns mean all requests at
the request stage; an explicitly empty list matches none. Request and response
pauses retain the same interception ID. Disabling or detaching releases paused
traffic.

## Deliberate boundaries

The fixture opts into the unload permission and installs an unload listener so
history tests exercise network navigation. Pyppeteer 2.0 updates its loader ID
only on a lifecycle `init` event and cannot reliably await Chrome 152 BFCache
restores. That old-client artifact is not used as a generic Mimic expectation;
BFCache restoration is outside this suite.

Pyppeteer serializes a native Python NaN as invalid JSON. Special-value argument
tests instead create and pass real NaN/Infinity/-0 remote handles, which both
clients support.

Screenshot/PDF/screencast output and media-device/playback domains are explicitly
excluded. The pointer-input check remains a real check and reports failure when
the runtime cannot provide the necessary geometry. A green result claims only
these measured workflows, not the entire Chrome protocol or full browser layout.
