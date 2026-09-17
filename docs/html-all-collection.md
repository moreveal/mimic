# HTMLAllCollection compatibility

`Document.all` is a lazy, identity-stable native HTMLDDA object. V8 supplies
its callable behavior, `typeof`, truthiness and abstract equality; wrapping it
in an ordinary JavaScript Proxy would lose those operator semantics. The
gov8 extension and its reproducible native build are described in
`third_party/gov8/README.mimic.md`.

The collection reads the owning Document's canonical DOM on each operation.
It does not cache a second element tree. Named duplicates use a cached live
HTMLCollection; indices, supported names, descriptor flags, expando precedence
and method shape are checked against frozen Chrome 152. Set/define on supported
indices and names deliberately succeed without replacing their values, as
measured in Chrome. Deleting a supported entry fails.

Bootstrap creates only prototypes, methods and empty private maps. The native
object and its callback state are allocated on first access after host binding,
including in a restored snapshot. Required Chrome-oracle tests exercise both
ordinary and restored bootstrap.

Same-origin frame references retain their owner/handle identity and import as
native undetectable objects. Calls and property access still use the existing
frame bridge and origin checks. Assigning a reference to another frame's global
now writes the actual destination global instead of a local WindowProxy copy.

Saved Documents and their collections retain their immutable owner across
iframe navigation. Page lifetime tracking separates imported object owners
from WindowProxy references and collects unreachable inactive realm graphs.
The unchanged Chrome navigation oracle now runs by default in both ordinary
and restored bootstrap modes. Bridge-cache and DOM-arena retention boundaries
are described in
`compatibility/compat-fuzz-fixes/navigation-ownership.md`.

## Existing boundaries

- The imported native object uses the original remote prototype at import.
  Subsequent remote `Object.setPrototypeOf` is not reflected in its local
  `Object.getPrototypeOf`. Native V8 interceptors cannot trap that internal
  operation, and introducing a Proxy would break HTMLDDA. Existing unsupported
  cross-frame define/delete/prototype mutation operations remain unsupported.
- gov8 roots callback data until isolate disposal, following its existing
  callback registry ownership. Repeated detached Documents can retain their
  collection handlers until Page teardown even if JS drops its references.
  Page teardown releases the registry and native handles.
- Engines without the native undetectable-object capability report an explicit
  unsupported error when reading `document.all`.

The ordinary corpus and its reference provenance live under
`internal/browser/testdata/document_all*`. Validation commands:

```powershell
go test ./internal/browser -run 'TestDocumentAll.*FrozenChrome|TestFrame|TestBootstrapSnapshot' -count=1
go test -race ./internal/engine/v8 -count=1
```

No protected-site challenge result is inferred from these tests.
