# Audit of standard algorithm reuse

Date: 2026-09-09. Basis: HEAD `93656f59f2b9a49a67efe20a399ef6513b63f81a` and working changes from the hydration investigation. This audit did not modify production code, dependencies, or frozen workloads. Upstream links were checked during the audit; library versions must be pinned in a separate lock/manifest.

The most useful replacements are CSS selectors, CSS syntax, URL, encoding, and streams. HTML parsing, cryptographic primitives, and decompression already use mature third-party implementations. Their primary problem is browser integration and Web API semantics, not handwritten algorithms.

Update after integration: Window selectors now use CSS-tree/DOMSelector preprocessing/css-select; Streams now use web-streams-polyfill 4.3.0. The LOC table below preserves the original audit snapshot; it does not claim that these handwritten algorithms remain the main production path. In the final code, the restricted native selector leaf helper occupies 42 lines, the canonical selector adapter 198, and the AST translator 34; vendor algorithms are excluded. The 130-line Fetch/body integration uses mature Streams. The old Worker streams layer remains a separate unfinished consumer. These migrations and their measured limitations are documented in the [selectors domain](selectors-domain-20260909/report.md), [Fetch/Streams](fetch-domain-20260909/report.md), and [performance report](../performance/report.md). Other audit recommendations have not yet been implemented.

## Measurement method

LOC below counts physical lines in selected regions, including blank lines, not logical statements. `surface.js` contains long lines, so UTF-8 size after CRLF-to-LF normalization is also listed. Regions are explicitly identified: these are not estimates of all transitive subsystem code. Generated bindings and library code are excluded. Line numbers refer to the audit snapshot. Do not sum overlapping regions. Machine measurements are saved separately in `.build/standards-audit-loc.json`.

| Subsystem | Measured handwritten region | LOC / bytes |
|---|---|---:|
| HTML | `internal/dom/dom.go:32–82`, Parse→canonical import | 51 / 1344 |
| HTML fragments | `internal/dom/dom.go:627–683`, SetInnerHTML region including the start of the next function | 57 / 1778 |
| Selectors | `internal/dom/selectors.go`, entire new working implementation | 372 / 7475 |
| CSS syntax/cascade | `internal/webapi/surface.js:90–103`, including JS selector helper | 14 / 8274 |
| URLSearchParams | `surface.js:248–251` | 4 / 2996 |
| URL object | `surface.js:267–270` | 4 / 2526 |
| URL host parsing/setters | `internal/browser/realm.go:1110–1173` | 64 / 1946 |
| UTF-8 encoder | `surface.js:59–61` | 3 / 1453 |
| bytes→string helper | `surface.js:253` | 1 / 181 |
| TextDecoder | `internal/webapi/dom_compatibility.js:157–176`, including the following matches registration | 20 / 1943 |
| Window streams | `surface.js:254–264` | 11 / 8308 |
| Worker streams | `internal/webapi/worker.js:32–35` | 4 / 2127 |
| WebCrypto classes | `surface.js:278–281`, excluding separate Crypto/getRandomValues | 4 / 3930 |
| WebCrypto host area | `realm.go:756–876`, including random helpers | 121 / 3987 |
| Worker crypto host area | `internal/browser/worker.go:193–240` | 48 / 1837 |
| Compression dispatch | `internal/network/loader.go:328–360` | 33 / 1050 |
| Structured-clone-adjacent messaging | Entire `internal/browser/message_port.go`; delivery/ownership, not a clone algorithm | 84 / 2455 |

Searching production sources found no separate complete handwritten structured-clone algorithm: messages pass through runtime value export/import, and some JS channels pass the original value. Therefore “0 LOC of a complete algorithm” does not mean support. The audit does not attribute the general cost of `engine.Value.Export/Value` conversion to structured cloning.

## Decision matrix

Complexity: L — local adapter; M — multiple consumers/test layers; H — engine/FFI/lifetime/scheduler changes. Gains and performance are forecasts, not benchmark results. No WPT percentages are invented: a full domain baseline was not run in this audit.

| Domain | Candidate / language / license | Canonical-state adapter | Complexity | Expected gain | Expected performance |
|---|---|---|---|---|---|
| HTML | Retain x/net/html, Go BSD-3-Clause; fallback parse5, TS/JS MIT | Currently temporary tree→canonical IDs. parse5 has TreeAdapter | L for import fixes; H for builder replacement | High from import/script integration fixes; uncertain from parser replacement | Retaining Go core is neutral; a JS adapter with a host call per token may regress |
| Selectors | css-select, TS/JS BSD-2-Clause; Servo selectors, Rust MPL-2.0 | css-select explicit Adapter; Servo Element trait; Cascadia has no direct adapter | M JS / H Rust FFI | High: grammar, combinators, functional pseudos | Compile cache useful; per-node host crossings risky; measure |
| CSS syntax | tdewolff/parse, Go MIT; rust-cssparser, Rust MPL-2.0 | Yes: tokens/AST are not a second DOM | M Go / H Rust FFI | High for syntax; cascade/layout separate | More work than split, less repeated parsing with caching |
| URL | nlnwa/whatwg-url, Go Apache-2.0; Ada, C++ MIT OR Apache-2.0 | Yes: one URL record, net/url only a transport representation | M Go / H C++ | High: WHATWG parsing/setters/host normalization | Unknown in Mimic; batched getters/records matter more than advertised upstream nanoseconds |
| Encoding | x/text/encoding + htmlindex, Go BSD-3-Clause | Yes, streaming decoder state does not duplicate DOM | M | High for labels/legacy encodings | Go bulk decoding likely better than JS arrays; calls on tiny chunks may cost more |
| Structured clone | V8 ValueSerializer/Deserializer, C++ BSD-3-Clause | Yes: host-object delegate and transfer-ownership table | H | High for JS graphs; platform objects need adapters | Fewer JSON-like conversions expected; real buffer/graph profiles needed |
| Streams | web-streams-polyfill, TS/JS MIT | Yes: realm-local JS state + canonical network source | M–H | Very high relative to simplified queues | More bootstrap/Promise overhead; bounded queues may improve memory |
| Crypto | Retain Go crypto/*, Go BSD-3-Clause; x/crypto for required algorithms | Yes: opaque key handles, semantic JS wrapper | M | High on completing the WebCrypto contract; core already mature | Bulk bytes and parsed-key caching more promising than replacing primitives |
| Compression | Retain Go compress/* BSD-3-Clause, andybalholm/brotli MIT, klauspost/compress BSD-3-Clause with separate license notices | Yes: io.Reader→canonical response body | L–M | Gains from content-coding chains and streaming wrappers | Streaming reduces peak RAM; no codec replacement without profiling |

## HTML tokenizer / tree builder

`dom.Parse` already calls `golang.org/x/net/html.Parse`, fragments use `ParseFragment`, and serialization uses `html.Render`. There is no custom tokenizer or adoption-agency algorithm here. The [x/net/html documentation](https://pkg.go.dev/golang.org/x/net/html) describes the HTML5 parser and its limitations, including the distinction between a complete tree and tokenizer output.

Observed gaps are in the adapter: main Parse converts comments/doctype to `other` and loses data; attributes are stored by Key alone without namespace; foreign tag names are uppercased. Fragment context does not pass namespace, and fragment-element import does not record Namespace. Full HTML is parsed before scripts execute, so replacing the parser alone does not fix the parser-blocking script/document.write insertion point. Template content needs a separate canonical fragment, not a second independent tree.

Retain x/net/html as the first option. If WPT exposes actual tree-builder gaps, compare parse5: MIT, widely used; its [TreeAdapter](https://parse5.js.org/interfaces/parse5.TreeAdapter.html) can direct insert/adopt/template operations into the canonical store. [Upstream parse5](https://github.com/inikulin/parse5) is a standalone core, not all of jsdom. Replacing it with jsdom would introduce a second DOM and is unacceptable here. Introduce a JS parser only after comparing host crossings and teardown.

## CSS selector parser / matcher

The working tree introduced `selectorParts`, `matchSelectorChain`, and `matchCompound`; the old JS `cssSelectorMatch` remains in the computed-style path. These are already two diverging matchers. Code inspection shows incomplete CSS escapes and gaps in broad grammar validation/SyntaxError, namespaces, :scope/:has/:nth-*, and dynamic-state coverage. The exact list changes with fixes; support for a few GitHub selectors does not complete the domain.

[css-select](https://github.com/fb55/css-select) provides an Adapter through getParent/getChildren/getAttributeValue/isTag and compilation. This is the most direct path without a second DOM. Do not enable jQuery-only extensions in browser APIs. Disable result caches or invalidate them by mutation generation; AST caching is acceptable. Dynamic pseudos must read actual Page state. A JS adapter over canonical proxies may generate thousands of host calls; compare it with a compact read-only traversal bridge that does not store a second mutable DOM.

[Cascadia](https://github.com/andybalholm/cascadia), Go BSD-2-Clause, accepts `*html.Node`. It is not a drop-in canonical adapter: it needs a fork of matcher node access or a temporary snapshot. A snapshot per query is expensive and risks staleness; a persistent mirror violates the single authoritative state model. [Servo selectors in Stylo](https://github.com/servo/stylo), Rust MPL-2.0, fit architecturally through the Element trait, but FFI/ownership/build are more complex. Prioritize a limited css-select adapter spike followed by tests, rather than expanding the custom parser.

## CSS syntax

`parseCSS` splits text on `;` and the first `:`; stylesheet rules are extracted with a `{}` regex. Strings, data URLs, escapes, nested functions/blocks, @media/@supports/@layer, and nested rules break this model; specificity is also regex-based. Replacing the syntax parser does not provide a complete CSSOM, cascade, property validation, or layout.

[tdewolff/parse/css](https://github.com/tdewolff/parse) is a Go MIT CSS Syntax Level 3 lexer/parser with streaming grammar units. It is the preferred first option because of the existing Go host and lack of FFI. [rust-cssparser](https://github.com/servo/rust-cssparser) is MPL-2.0; it intentionally omits the final layer of property-specific grammar and selectors. It is interesting alongside Servo selectors, but introducing Rust solely for tokenization costs more. Parse immutable text→tokens/AST; keep stylesheet identity/owner/rule mutations canonical. Cache by source/version instead of reparsing every style element for each computed property.

## URL

The implementation uses mature `net/url`, but that is not a browser WHATWG URL parser. `urlParts` defaults to documentURL as base even when the JS URL constructor received no base. `setURLPart` directly changes net/url fields; default-port removal, special schemes/backslashes, opaque paths, legacy IPv4 forms, IDNA, and setter validation need separate semantics. `URLSearchParams` uses decodeURIComponent, which throws on malformed percent/UTF-8 instead of applying replacement behavior; the iterator currently uses snapshots.

[nlnwa/whatwg-url](https://github.com/nlnwa/whatwg-url), Apache-2.0, provides Go APIs for WHATWG records and setters. Its README claims relevant WPT passes, but the snapshot is dated May 24, 2023: this does not prove Chrome 152 compatibility. [Ada](https://github.com/ada-url/ada), C++ MIT/Apache-2.0, is used in major runtimes and claims a complete specification test suite; consider it if the Go candidate fails current tests or profiling. Migrate JS URL, navigation resolution, workers, history, fetch, and origin calculation coherently. Convert the serialized canonical record to net/url at the transport boundary; do not maintain two independent URL states.

## Encoding

The UTF-8 encoder is handwritten; bytesString reduces any decode error to a single U+FFFD, losing the remaining text. The working TextDecoder implements only UTF-8 and a few aliases. Broad label tables, legacy/stateful encodings, and a full BufferSource/stream error contract are absent. Check UTF-16 input→USVString before Go conversion to preserve lone-surrogate semantics.

Dependencies already include `golang.org/x/text`. [htmlindex](https://pkg.go.dev/golang.org/x/text/encoding/htmlindex) maps web encoding labels; Decoder/Transformer provide incremental state. A thin fatal/ignoreBOM/end-of-stream layer and checks for replacement-behavior differences are needed. A codec's presence alone does not provide a complete TextDecoder. HTML charset sniffing, CSS byte decoding, and Fetch body decoding should use coherent rules, but their encoding-selection algorithms differ. Prioritize connecting existing codecs and stop expanding handwritten byte tables.

## Structured cloning

`message_port.go` handles ownership/delivery; values arrive through engine export and return as runtime.Value. This does not establish preservation of cycles, shared identity, Map/Set, Error, BigInt, typed-array offsets, or detachment. Some legacy JS MessagePort/BroadcastChannel paths capture the original object. The inspected production sources contain no explicit complete `structuredClone` implementation.

[V8 ValueSerializer](https://v8.github.io/api/head/classv8_1_1ValueSerializer.html) is the natural core for the V8 engine: graph serialization plus a host-object delegate and ArrayBuffer transfers. It is not all of HTML structured clone: Blob/File/MessagePort/CryptoKey, cross-realm ownership, DOM-node rejection, security, and transfer transactions remain Mimic's responsibility. gov8 bindings and an engine interface are needed, with a separate strategy for goja/QuickJS. Do not silently use one wire format across incompatible engine versions.

[@ungap/structured-clone](https://github.com/ungap/structured-clone), JS ISC, is a possible limited fallback for ordinary JS graphs, but upstream explicitly states that the transfer option is ignored and many platform objects are unsupported. It does not replace the complete browser contract. Do not write another JSON clone.

## Streams

Window and Worker contain different small handwritten implementations. In Window, strategy is largely ignored, BYOB is absent, desiredSize is fixed relative to one, pipeTo does not implement the options/abort contract, and writable writes lack complete serialization/backpressure machinery. The Worker implementation is even simpler. This is a separate domain, not just a set of methods for fetch.

[web-streams-polyfill](https://github.com/MattiasBuelens/web-streams-polyfill), MIT, has a browser WPT test suite and documents snapshot/spec exceptions. Integrate one pinned realm-local build for Window/Worker, preserving Promise/microtask scheduling on the current Page event loop. Network sources and Blob/Response must create its streams instead of mixing classes from different implementations. Transferable streams and host cancellation do not appear automatically. Measure startup cost, large-body peak memory, and cancel/teardown retention alongside throughput.

## Crypto

Algorithms are already delegated to Go `crypto/rand`, SHA, `crypto/rsa`, and `crypto/x509`; dependencies also include x/crypto. The handwritten layer covers WebCrypto normalization, key metadata, and conversion. `SubtleCrypto` implements digest and public SPKI RSA-OAEP import/encrypt; other main operations return NotSupportedError. In `cryptoBytes`, the ArrayBuffer.isView branch uses `Uint8Array.from(data)`, which is not equivalent to reading underlying bytes for every view type.

Retain [Go crypto](https://pkg.go.dev/crypto) and complete coherent algorithm families: key formats/usages/extractability/error order/BufferSource, then encrypt/decrypt or sign/verify together. An opaque host key handle is preferable to repeated DER parsing and JS-visible key storage. Do not add OpenSSL/BoringSSL solely for already working SHA/RSA: gains are unmeasured, while build/lifetime costs are real. A new backend requires a specific missing algorithm or profiling evidence.

## Compression

There is no handwritten deflate/brotli/zstd core. `decodeContent` dispatch uses standard Go gzip/zlib/flate, [andybalholm/brotli](https://github.com/andybalholm/brotli), and [klauspost/compress/zstd](https://github.com/klauspost/compress). MIT/BSD dependencies are already pinned in go.mod. Preserve their separate notices when vendoring.

Adapter gaps: only a single Content-Encoding value is supported; chains such as `gzip, br` are not parsed; entire bodies are materialized; LimitReader may report a truncated body at the limit as success; limits differ between zstd and other codecs. The inspected files contain no explicit algorithm implementation for the CompressionStream/DecompressionStream surface. Use the same codecs through stream sources/sinks after completing the streams domain; test flush, truncated data, checksums, and cancellation. Do not replace codecs without a fresh profile.

## Coverage matrix and criteria for returning to production workloads

This is a state inventory, not an invented WPT score. Generated `chrome/152/generated/surface-catalog.json` reflects API shape, not semantics. For each domain, retain the pinned WPT revision, selected tests, Chrome 152 binary SHA, Mimic build SHA, separate PASS/FAIL/TIMEOUT/SKIP counts, and known exclusions. A missing count is marked “not measured,” not zero.

| Domain | Existing evidence | Required WPT subset / surface | WPT Chrome↔Mimic baseline |
|---|---|---|---|
| HTML | Go DOM tests, individual hydration fixtures | [html/syntax/parsing](https://github.com/web-platform-tests/wpt/tree/master/html/syntax/parsing), DOMParser/innerHTML/templates, Document/Element/HTMLTemplateElement IDL | Not measured in audit |
| Selectors | Basic Go queries; independent working selector probes | dom/nodes selectors + css/selectors; ParentNode.querySelector(All), Element.matches/closest | Not measured |
| CSS syntax | CSS/layout smoke paths | css/css-syntax + css/cssom; CSSStyleDeclaration, CSSStyleSheet, CSSRule | Not measured |
| URL | Browser URL tests / generated catalog | [url](https://github.com/web-platform-tests/wpt/tree/master/url): constructor, setters, urltestdata, URLSearchParams, IDNA | Not measured |
| Encoding | TextEncoder tests, working UTF-8 streaming probe | encoding: textdecoder/textencoder/labels/streams IDL | Not measured |
| Structured clone | MessagePort/frame/worker delivery tests | [structured-clone](https://github.com/web-platform-tests/wpt/tree/master/html/webappapis/structured-clone) + messaging transfer cases | Not measured |
| Streams | Blob/pipeTo/TransformStream smoke | [streams](https://github.com/web-platform-tests/wpt/tree/master/streams): readable, writable, transform, BYOB, piping, transfer | Not measured |
| Crypto | Random/digest/RSA-OAEP regressions | WebCryptoAPI algorithm families + IDL | Not measured |
| Compression | Loader/transport tests | compression + fetch/content-encoding tests | Not measured |

For every run, compare API shape with the frozen Chrome 152 catalog and corresponding Blink/WebIDL files at the pinned Chromium revision, not moving main. This audit did not download a complete Blink checkout or claim to check every IDL member. Do so for the selected domain before migration. API exposure counts and behavioral coverage must remain separate columns.

A reasonable minimum for domain completion: all selected standard positive/negative cases pass; unsupported features are explicitly listed as exclusions; no known loss of canonical identity/ownership; mutation/cancel/error paths checked; no unexplained regression in the frozen fast gate or memory/teardown. A production site then checks integration, but does not define grammar or exceptions.

Priority: (1) finish measuring the current DOM/hydration boundary; (2) unify selector consumers behind a standard parser/matcher adapter; (3) CSS syntax and URL; (4) streams+Fetch body/cancel; (5) encoding; (6) a structured-clone engine bridge. Fix HTML import and crypto/compression wrappers according to confirmed domain failures, retaining the mature cores already in use.
