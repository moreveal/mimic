# Navigation request context

Navigation retains its initiator's document URL and transient activation at the
time it is requested. A direct Page/CDP navigation is user initiated; script
reload and Location assignments use the originating realm's state. Child
document requests have Fetch destination `iframe`, independently of their CDP
Document resource classification. Child self-navigation takes its initiator from
the child even though the embedding realm owns the navigation lifecycle.

Referrer-Policy is captured from committed document response headers. An iframe
attribute can override its embedding document policy. All eight policy tokens
are serialized at the network boundary, retaining SourceURL independently for
Fetch Metadata even when no Referer is sent. The default is
strict-origin-when-cross-origin, and credentials/fragments are excluded from
Referer. Dynamic `<meta name=referrer>` policy changes and inherited source URLs
for initial about:blank documents remain outside this change.

The local frozen Chrome 152 capture
`compatibility/captures/semantic-checkpoints/navigation-request-context-chrome152.json`
records direct navigation, script reload and Location assignment, cross-origin
iframes, response policy and element overrides. Focused browser tests compare
actual received headers to those observations. These semantic fixes do not by
themselves establish why a particular remote server rejects a request.
