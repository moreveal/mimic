# Request language, credentials and storage access

The Chrome 152 reference exposes navigator.languages as ["ru-RU"] in both Window
and Worker. A loopback HTTP echo server nevertheless receives a full Worker
Accept-Language list: ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7. Document fetch uses only
ru-RU,ru;q=0.9. This difference also occurs outside the archived remote workload.
CDP Fetch interception omitted the Worker's late-added language header; the
retained oracle therefore records what an actual HTTP receiver observes.

Locale.Languages now retains the complete ordered preferences. The profile's
ReduceAcceptLanguage setting projects the first language into Navigator and
document requests, while Worker request headers use the complete list. A request
identifies its initiating realm independently of its resource destination: loading
a worker script from a document is not a Worker-initiated fetch. Author-supplied
Accept-Language continues to override defaults.

Fetch credentials were previously retained in the JavaScript Request but dropped
at the host boundary. The shared Fetch request now carries the mode to the loader;
XHR carries withCredentials as well. The loader uses the effective credentials
policy for both outgoing cookie selection and incoming Set-Cookie acceptance.
Omit stays effective across redirects, and redirect processing discards derived
storage-access headers before recomputing them for the next target.

Request-owned top-level and initiating document URLs provide the cookie context.
Sec-Fetch-Storage-Access is emitted for potentially trustworthy, credentialed
requests in a cross-site cookie context. Same-site requests, requests without
credentials and top-level navigations omit it. An iframe's same-origin subresource
can still have a third-party cookie context relative to the top-level document.
The current profile allows unpartitioned cookies and therefore yields active;
its existing global cookie-disable state yields none. Schemeful site comparison
uses the public suffix list and treats IP addresses as individual sites.

Document.hasStorageAccess and hasUnpartitionedCookieAccess return Promises from
the same current-document cookie configuration; inactive parsed documents reject.
The current model does not implement per-site third-party-cookie exceptions,
storage-access grants/activation, Activate-Storage-Access retries, partitioned
cookies or full SameSite cookie filtering. Full iframe sandbox-origin construction
also remains a separate limitation; opaque-origin request metadata is handled
when that origin is already represented by the realm.

The general header conditions are consistent with the primary
[Storage Access Headers draft](https://privacycg.github.io/storage-access-headers/#sec-fetch-storage-access-header).
Frozen Chrome measurements, rather than the draft alone, establish the supported
profile's output, including effective same-origin credentials inside an iframe.
The blocked-cookie DevTools probe did not change localhost behavior and is not
evidence for native blocked-cookie parity.

Reference captures under compatibility/captures/semantic-checkpoints:

- language-realms and language-network: JS and received request headers.
- storage-access: cross-site iframe navigation, child fetch and access query.
- storage-fetch: Fetch credential modes and XHR withCredentials.
- fetch-credentials: cookie sending, Set-Cookie acceptance and redirect omission.

Tests run the exact fixtures on V8 and Goja; cookie semantics are also exercised
inside Workers. Additional state/network checks use a different language profile,
IP/registrable-domain classification and opaque request origins. The capture
helper uses an ephemeral loopback server and disposable Chrome context, with a
bounded evaluation wait. No remote challenge was contacted by these probes.
