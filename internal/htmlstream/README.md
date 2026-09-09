# Incremental HTML tree construction

The tree-construction rules in `parse.go`, `foreign.go`, `const.go`,
`doctype.go`, and `stacks.go` derive from `golang.org/x/net/html` v0.51.0.
Their BSD license is retained in `LICENSE`. Keep changes to those rules small
and compare future updates against that upstream version.

The upstream parser keeps its state private and only exposes batch parsing.
Mimic needs to stop at script boundaries, insert document.write input ahead of
the unparsed suffix, and let script mutations affect the same tree that the
parser is constructing. The adapted tree builder therefore uses canonical DOM
handles: every parent/child/attribute/text operation delegates to the owning
Document. There is no second html.Node tree to reconcile after JavaScript runs.
The main mechanical differences from upstream are Node accessor methods,
canonical creation/mutation callbacks, and supplying a foreign element's
namespace at creation rather than changing it immediately afterward.

The tokenizer remains the upstream x/net/html implementation. A stream-local
goroutine preserves its stack across incomplete input. It never runs JavaScript
or a browser event loop. The caller waits for an input boundary or script and
executes callbacks on its own thread. At a completed script boundary the lexer
is in data mode; its buffered suffix is saved at an insertion point and a fresh
tokenizer consumes nested writes before that suffix. This resets lexical input
buffering only, never the persistent HTML5 tree-construction state.

Because the upstream tokenizer coalesces character tokens, starvation exposes
the current token's unambiguous text prefix through the same tree builder.
Incomplete tags, comment/declaration contents, closing-tag candidates, and
character references remain buffered. When the complete token arrives, the
already-emitted prefix is removed once. No accumulated document markup is
reparsed. Tests compare byte-at-a-time streaming against the batch tree builder
and exercise canonical mutations at script pauses.

The owner serializes Write/Close/Resume and must Abort a superseded stream.
Callbacks may reenter Write and Close. Returning ErrPaused retains the script
insertion point; Resume invokes that script callback again, allowing a loaded
external script to run before the pending suffix. Writes while externally
paused append to the queued input tail. Nested Close requests EOF after the
outer parsing operation and script continuations finish.
