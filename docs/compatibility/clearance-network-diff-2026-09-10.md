# Chrome / Mimic clearance network-state comparison — 2026-09-10

Scope: network state around the final same-origin `/fo` response and the next
navigation. No browser API implementations changed during this investigation.
Cookie values and challenge proofs are intentionally excluded from this report.

## Captures

* Mimic: `compatibility/private-captures/manual-20260910-061345`.
* Chrome 152.0.7977.82: `compatibility/private-captures/chrome-clearance-20260910-network`.
* Independent Chrome run with an additional same-context GET:
  `compatibility/private-captures/chrome-clearance-20260910-revisit`.

Chrome captures used fresh disposable contexts in the existing frozen browser,
recorded Network events including ExtraInfo, script sources, response/request
bodies and final cookie state. They did not import Mimic cookies or change DNS,
proxy, browser arguments, or system networking. Main navigation results are
complete. Some reads of the already-destroyed Turnstile iframe failed; these
errors are retained in the capture manifests rather than hidden.

## Results

Mimic received a clearance on `/fo`, sent the identical value on its next GET,
and received 403 with `cf-mitigated: challenge` (Ray `a38aeb206cb1a893-TBS`).

Chrome first received the challenge, obtained clearance, submitted a POST to
`/mimic-e2e`, and reached the site's 404 page without `cf-mitigated`. The separate
control run observed:

| Request | Response | Evidence |
| --- | --- | --- |
| Initial GET | 403 challenge | Ray `a38afce50e60d951-TBS` |
| Automatic POST with clearance | 404, no challenge | Ray `a38afcfb8fb3d951-TBS` |
| Explicit later GET in same context | 404, no challenge | Ray `a38afda0a83dd951-TBS` |

The final page title was `IroShop — for distributors`, with the application text
`404 / This page could not be found.` This demonstrates disappearance of the
challenge in these Chrome runs, not a claim that the application route exists.

## Requested ten-point comparison

| State | Chrome | Mimic | Interpretation |
| --- | --- | --- | --- |
| 1. Final `/fo` Set-Cookie | cf_clearance; HttpOnly; SameSite=None; Partitioned; Secure; Path=/; Domain=iroshop.tech; one-year expiry | Same attribute set; different session token as expected | No obvious raw attribute mismatch |
| 2. Effective attributes | ExtraInfo associatedCookies shows `.iroshop.tech`, `/`, HttpOnly, Secure, SameSite=None, non-session, Medium priority, Secure source/443; no blocked reasons | Actual cookie object not captured by the manual recorder. Store code normalizes domain and uses domain/path matching; exact cookie value subsequently sent | Raw headers alone are not an effective-store snapshot |
| 3. Partition key | `{topLevelSite: https://iroshop.tech, hasCrossSiteAncestor: false}` on final same-origin response/navigation | No partition key in CookieStore's key: only domain/path/name (`internal/network/cookies.go`) | Confirmed model gap, but not sufficient to explain this fresh first-party request; partition keys are not transmitted as Cookie attributes |
| 4. Top-level site | https://iroshop.tech, directly confirmed by partition metadata | https://iroshop.tech, derived from top-frame navigation | Same logical site; Mimic does not record it in a cookie decision snapshot |
| 5. Initiator/frame/site-for-cookies | Main-frame navigation; CDP initiator `other`; associated cookie unblocked; no cross-site ancestor | Main-frame navigation; initiator `other`; no equivalent cookie inclusion/blocked-reason record | CDP initiator `other` does not identify the JavaScript call stack; site-for-cookies is not directly exposed in these captures |
| 6. Repeated navigation Cookie | cf_clearance, with no blocked reason | Exact issued cf_clearance plus cf_chl_rc_ni=1 | Presence and value continuity confirmed internally in Mimic; independent server-received headers still not captured |
| 7. Sec-Fetch-* | dest=document, mode=navigate, site=same-origin, no sec-fetch-user on automatic POST | Same on automatic GET | These fields agree despite different HTTP methods |
| 8. Client Hints | Full high-entropy hint set on repeated navigation | Same values on repeated navigation | Initial hint negotiation differs: Mimic starts with high-entropy hints already present; native first request shows low-entropy hints. This is a starting-state difference, not evidence of token rejection |
| 9. Referer | `/mimic-e2e?__cf_chl_tk=<token>` | `/mimic-e2e` | Confirmed difference in transient document/history state |
| 10. Connection/protocol | h3; existing connection reused across challenge and navigation; CF peer 172.67.151.28 in first native run | h3; same recorded connection reused; CF peer 104.21.0.150 | Different server addresses are normal DNS endpoints and do not reveal client egress IP. No observed protocol switch or connection replacement |

## Most substantial additional difference: navigation method and proof body

Native automatic navigation was POST, `application/x-www-form-urlencoded`,
2584 bytes in the first Chrome capture, with three opaque challenge fields.
It included Origin and `cache-control: max-age=0`. Mimic performed GET without
a body, Origin, or that cache directive.

Mimic trace sequence 2730 records an access to `Location.reload`; sequence 2739
then records the GET. Therefore the evidence does **not** show a POST silently
rewritten into GET by the transport. The execution took a reload path. It remains
necessary to identify why the completion paths differ: different server-provided
completion response, a client branch, history/form behavior, or another condition.
Do not fix this by forcing all reloads to POST or inventing challenge form fields.
The successful native follow-up GET also shows that GET itself is not inherently
incompatible with a valid clearance.

## IPv6 / brunhild

On this host, direct DNS queries returned no A address and two AAAA addresses:
`2606:4700::6812:1092`, `2606:4700::6812:1192`. An IPv6-only DNS name is valid.

The machine had no IPv6 default route (`::/0`). Its listed IPv6 addresses were
link-local, loopback or VPN-local; a direct IPv6 TCP connection failed with
Windows error 10051 (network unreachable). System getaddrinfo also failed with
11004 outside Mimic. Therefore an answer from nslookup does not establish usable
IPv6 connectivity. Changing the DNS server or bypassing getaddrinfo cannot create
a missing route. No explicit WithDisableIPV6 option was found in Mimic's
transport construction during this inspection.

Crucially, Chrome failed its brunhild request with `net::ERR_NAME_NOT_RESOLVED`
and nevertheless proceeded past the challenge. Thus this DNS failure is not a
sufficient explanation for Mimic's recurring 403 on this machine. It could still
be a signal or affect other environments; this experiment does not assign it
universal irrelevance. Network settings were not changed.

## Next evidence to collect

1. Locate the first difference in the completion response/callback that leads to
   native POST versus Mimic Location.reload. Preserve method, action URL, body
   construction and transient document/history URL rather than guessing from API
   missing counts.
2. Obtain the server Security Event for Mimic Ray `a38aeb206cb1a893` and compare
   rule/action with the successful native requests. The client trace does not
   reveal whether CF rejected the token or a separate rule re-challenged it.
3. Add explicit partition/site-for-cookies inclusion diagnostics and a local
   server echo comparison of serialized request headers. Cookie jar partitioning
   is a real general compatibility issue, but should not be labeled the cause of
   this 403 without a discriminating test.

## Reproduce

```powershell
python -X utf8 tools/compatibility/capture_native.py compatibility/private-captures/new-clearance-run 9343 https://iroshop.tech/mimic-e2e --revisit
python -X utf8 -m unittest discover -s tools/compatibility -p test_capture_native.py
```

`--revisit` saves cookies before an extra GET and the resulting state. It uses
the same disposable context; the default capture behavior is unchanged. Capture
helper tests: 4 passed. Runtime source code and the Mimic binary were unchanged.
