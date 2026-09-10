# Compatibility boundary diagnostics

All current JavaScript calls reporting a semantic-missing boundary use
`semanticMissingAt` with an implementation source site. Window and worker
hosts share the same recorder. Existing names and event kinds remain stable.
A source inventory test rejects new calls without a source site. Source line
labels describe the current implementation and should be updated when moved.

Fields: operation, site, realm or worker, category, reasonAvailable, and optional
bounded detail/context/reason. Approximation notices are distinguished from
unsupported boundaries. reasonAvailable=false explicitly means the branch has
not supplied a detailed reason; it is not a claim that the server cause is known.

Font-size resolution includes source value, target and failing ancestor,
parent/root size and expression failure. Shaping failures preserve the backend
error, font family, size, weight, style, shaping flags, bounded text and full
rune count. Author-facing errors remain sanitized. These details are private
capture data and may include page text or local resource paths.

Context is capped at 2048 Unicode characters and marks truncation. Diagnostics
do not enumerate author objects, invoke their getters, or capture JS Error.stack
(which can invoke author hooks). They do not alter public exceptions, API return
values, or existing reporting frequency/deduplication.

This covers existing explicit semantic boundaries, not arbitrary future bugs,
every caught JS exception, all network failures, or a Cloudflare verdict. A new
boundary should supply a branch-specific reason and the minimal primitive
inputs needed for a standalone reproduction, in addition to its source site.
