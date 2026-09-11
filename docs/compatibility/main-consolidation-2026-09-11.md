# Main consolidation, 2026-09-11

Merged the existing main tip `8552a76`, semantic cleanup `b943313`, and hydration
branch `3860355`. The signed merge commits are `941f1bb` and `5d81e63`.
`0462bf5` retains hydration's local Window frames/length projections alongside
cleanup's native cross-realm reflection. Media, FileReader, IntersectionObserver
and XMLHttpRequestEventTarget registrations from both sides are preserved.

The first full browser run exposed the omitted local frame projection; its
failure is retained. Focused tests and the corrected full browser run pass.
Targeted browser race, core race, all 11 selected TLS connection-race tests
(using the root module replacements), four snapshot-tool Python tests and three
repetitions of the snapshot/native numeric regressions pass. A direct nested
TLS-module invocation failed to build against its different dependency graph;
the production root-module invocation passes. No target site or performance
gate was run. Exact commands, failures, skips and log hashes are in the
[receipt](main-consolidation-20260911.json).

All ten other local branches were audited and deleted. Their code changes are
already ancestors or patch-equivalent to the merged main. The only unmatched
documentation patch, `b2ce7c8`, differs from `886ab78` solely in insertion context
and a trailing blank line; its report contents are already present. The remote
had only main before publication.

All 34 other worktree registrations were removed at the user's explicit request,
including experimental/diagnostic worktrees and eight dirty worktrees. Thirteen
unmatched experimental commits were not merged into production. Committed
history is retained in the verified `.build/pre-main-consolidation.bundle`;
dirty tracked/untracked files were backed up in
`.build/main-consolidation-dirty-worktrees.zip`. These archives are local and
ignored, not branches or published captures.

The old youtube-twitch checkout's contents and registration are removed. Its
empty directory remains: Git could not remove it, and the subsequent explicit
directory removal was rejected by automatic policy review. No alternate deletion
method was used. Only `E:/GitHub/mimic` remains as a registered worktree.
