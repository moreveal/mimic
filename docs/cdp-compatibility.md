# CDP compatibility matrix

Target: Chrome 152.0.7977.82 Stable/Windows x64 at Chromium r1669021. Verification date: 2026-09-07. The complete known protocol surface comes from the bundle's pinned browser/V8 schemas; `expected` records calls observed from a real Pyppeteer 2.0 session.

| Method or event | Expected by client | Implemented | Behavior verified | Missing semantics |
|---|---:|---:|---|---|
| `Target.getBrowserContexts` | yes | yes | real Pyppeteer connect | incognito context creation/disposal |
| `Target.setDiscoverTargets`, `targetCreated` | yes | yes | page discovery | multiple targets and target filtering |
| `Target.attachToTarget`, `sendMessageToTarget`, `receivedMessageFromTarget` | yes | yes | legacy Pyppeteer session routing | multiple simultaneous attached sessions |
| `Page.enable`, `getFrameTree` | yes | yes | top and child frame tree projection | deeper nesting and every transient error-page state |
| `Page.navigate`, `frameNavigated` | yes | yes | 200 and 403 documents commit | full redirect/history/error-page semantics |
| `Page.lifecycleEvent`, `loadEventFired` | yes | yes | top/child DOMContentLoaded and load ordering; Pyppeteer `waitUntil: load` completes | network-idle tracking |
| `Runtime.enable`, `executionContextCreated/Cleared/Destroyed` | yes | yes | top/child realm creation, navigation replacement and frame removal observed | isolated worlds and non-default contexts |
| `Runtime.evaluate`, `callFunctionOn` | yes | yes | scalar and by-value object evaluation | remote object handles, getters and previews |
| `Network.enable` | yes | yes | event stream enabled | per-session buffering limits |
| `Network.setCacheDisabled` | yes | yes | canonical cache policy unit-tested | revalidation and full RFC cache behavior |
| `Network.setRequestInterception`, `continueInterceptedRequest` | yes | yes | real Pyppeteer request continuation | auth challenges and redirect interception details |
| `Network.requestWillBeSent`, `responseReceived`, `loadingFinished/Failed` | yes | yes | top/child document and script requests with frame/loader attribution observed | Chrome timing/protocol/wire detail fidelity |
| `Network.getResponseBody` | common | yes | bounded shared-loader body store | streamed and evicted-body errors |
| `Network.get/set/delete/clear cookies`, `Storage.getCookies` | common | yes | canonical cookie store tests/basic calls | SameSite, partitioned cookies, expiry completeness |
| `Network.setExtraHTTPHeaders` | common | yes | shared loader session state | forbidden-header validation |
| `Network.emulateNetworkConditions` | common | partial | offline state | latency/throughput/connection type |
| `Fetch.enable`, continue/fail/fulfill | common | yes | same interception layer | response-stage interception completeness |
| `DOM.getDocument`, `DOM.getOuterHTML` | common | yes | basic DOM access | full DOM domain node lifecycle and mutations |
| `Log.enable`, console/exception events | yes | partial | console and JS exception tracing/events | complete argument remote objects and stack traces |
| `Performance.getMetrics` | common | partial | coherent timestamp returned | Chrome metric set and navigation/resource timing |
| `Page.addScriptToEvaluateOnNewDocument` | common | yes | executes before page scripts | isolated-world selection/removal |
| `Page.setBypassCSP` | common | yes | policy/bypass behavior unit-tested | CSP hashes and strict-dynamic trust propagation |
| `Network.setUserAgentOverride` | common | no | — | must update JS identity, Client Hints and headers together |
| `Runtime.getProperties` | common | no | — | depends on remote object registry |
| `Page.getLayoutMetrics` | common | no | — | must derive from canonical viewport/layout state |

## End-to-end checkpoints

| Runtime | Current result |
|---|---|
| Real Chrome 152.0.7977.82 | pinned differential reference; the runner refuses a different product version |
| BrowserOxide 0.1.3 | documented baseline: challenge/resource/CSP path reaches timeout without a usable page |
| Mimic | Pyppeteer connects, enables interception/cache control, commits the Voxel 403 document, enforces nonce CSP, loads and executes the challenge script, delivers dynamic-script events and live timers, preserves a usable Page, and reports the next generic divergence through structured trace |

The matrix distinguishes stateful behavior from protocol-shaped success. A method is not marked verified merely because it returns `{}`.
