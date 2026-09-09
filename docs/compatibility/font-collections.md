# Font collections and resource boundary

FontFace descriptor state and FontFaceSet membership live in realm-local weak
maps. Each Document owns a separate canonical set, including documents created
through DOMImplementation. WorkerGlobalScope owns its own set. The collection
uses insertion order, live iterators, branded methods, and the EventTarget base.

Chrome 152 measurements cover constructor family serialization, common
descriptors and rejected assignments, malformed sources, short invalid binary
sources, promise identity, and collection operations. Eight captures under
`compatibility/captures/semantic-checkpoints/font-{collections,descriptors,constructor,documents}*`
retain separate Window and Worker observations. Run
`go test ./internal/browser -run 'TestFont(Collections|Resource)'` to compare V8
and Goja with these captures.

An empty active document set's ready promise resolves to the set. An empty Worker
or DOMImplementation-created document set's ready promise remains pending in the
measured state, including after a Worker's empty load request. Neither the same status string nor matching prototypes imply the
same lifecycle in the two realms. The native oracle helper accepts `--worker`
to execute the unchanged fixture inside a disposable worker in its own browser
context.

For an empty set, a supported valid font shorthand checks true even when the
requested family is not installed. This API does not test installed-font
availability. An empty load resolves to an empty array. Invalid shorthands
throw SyntaxError from check and reject load's promise. FontFace.load brand
errors also reject a promise rather than throwing synchronously.

## Remaining boundaries

This is not a font loader or a text shaping implementation. Local font lookup,
font fetching, binary decoding, CSS-connected @font-face entries, matching
nonempty sets, initial layout readiness and loading event transitions remain unsupported. Loading a
syntactically accepted source rejects the canonical loaded promise with
NotSupportedError and records a semantic diagnostic. It never reports a usable
font or invents glyph metrics. Short binary data that cannot contain a font
header rejects with SyntaxError. Longer binary data is not treated as decoded.

The descriptor and shorthand parsers implement common literal forms, not the
complete CSS grammar. Escapes, comments, computed expressions, all descriptor
normalizations and all font shorthand combinations remain compatibility gaps.
Source validation currently checks the basic function envelope; it does not
fully validate source lists or format/tech qualifiers. These limitations must
be resolved before claiming general Font Loading API compatibility.

The broader `font-faces-chrome152.json` capture retains the native missing-local
font loading observation (NetworkError). It is diagnostic evidence for the
remaining backend work, not a passing oracle: Mimic deliberately reports
NotSupportedError at that boundary. No live Cloudflare outcome was established
by this change.
