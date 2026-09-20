# Native Blitz producer

The canonical style/layout producer uses the maintained
[`moreveal/blitz`](https://github.com/moreveal/blitz) fork. Cargo pins an exact
commit from its `mimic` branch; the dependency is not a moving branch and is not
a Git submodule.

The fork preserves upstream history from
[`DioxusLabs/blitz`](https://github.com/DioxusLabs/blitz), so upstream updates
and semantic conflicts are handled in the fork. To update:

1. Merge a reviewed upstream revision into `moreveal/blitz:mimic` and run its
   focused checks.
2. Change both `blitz-dom` and `blitz-traits` revisions in `Cargo.toml` to the
   resulting fork commit.
3. Regenerate `Cargo.lock` and run `python tools/build_native_layout.py`.
4. Run Mimic's native binding, browser correctness, Wikipedia E2E and memory
   gates before committing the new pin.

Mimic's modified Parley remains in `vendor/parley` and is applied through
Cargo's `[patch.crates-io]`, because it belongs to a separate upstream project.
