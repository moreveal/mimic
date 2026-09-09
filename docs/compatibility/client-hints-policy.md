# Client Hints document policy

The frozen Chrome 152 capture in
`compatibility/captures/semantic-checkpoints/client-hints-policy-chrome152.json`
uses two distinct loopback origins and a fresh isolated browser context for each
case. The existing browser's launch flags/profile are not asserted. The reusable
runner is `tools/compatibility/client_hints_policy_oracle.cjs`; set
`PLAYWRIGHT_MODULE`, `CHROME_CDP_URL`, and optionally `ORACLE_OUTPUT` to run it.
The capture includes the exact runner hash.

High-entropy hints require the top-level origin's Accept-CH opt-in and the
initiating document's effective permission. A cross-origin iframe's own opt-in
does not suffice. Either explicit parent header delegation or the iframe allow
attribute enables the measured hint; an explicit ancestor denial still disables
the committed child's requests. The initial iframe navigation separately uses
the container delegation, including when the ancestor header denies the feature.

Realm commit captures the document policy and inherited feature permissions.
Request projections contain the top-level opt-in URL and immutable per-feature
destination allowlists. Network code consults the existing synchronized opt-in
store, without reading the DOM or a live realm. Redirects recompute high-entropy
headers for their new destination. Feature-policy CH queries use this same
projection. Low-entropy document hints remain unchanged.

In the captured Chrome loopback cases, dedicated worker script requests and both
URL/blob worker fetches omit all Client Hints, even when their creator is opted
in and delegated. Requests carry that agent distinction explicitly. Worker
scripts also use Fetch destination `worker` and mode `same-origin`. These worker
measurements do not separately establish behavior for every HTTPS deployment.

This is the bounded policy subset for the six high-entropy UA hints already
produced by Environment. It supports explicit origins, self, wildcard, empty
allowlists, and iframe src/default delegation. Full structured-header error
recovery, wildcard subdomain matching, dynamic modification of already-committed
container policy, other Permissions Policy features, and worker response policy
containers are not implemented here. Explicit empty Accept-CH versus an absent
header remains an existing session-store limitation.
