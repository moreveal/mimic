# Secure Window installation order

`window-secure-order.json` records `Object.getOwnPropertyNames(window)` order
in frozen headful Chrome 152.0.7977.82 on a loopback secure-context page.
The source is `compatibility/private-captures/window-sweep-phases-20260911/chrome.json`,
first `foreign` trial, `value.keys`, excluding the two explicitly added test
properties `sweepHits` and `sweepGetter`. All five foreign trials agreed.

The existing descriptor captures alphabetize properties; they remain unchanged.
This additional capture controls initial publication only. It does not sort
reflection results, add API capabilities, or affect script-defined properties.

The follow-up `compatibility/private-captures/window-global-orders-20260911`
capture records three fresh targets per exposure with the same owned headful
Chrome binary. Its secure ordering exactly reconfirms the original 1232 keys.
`window-insecure-order.json` contains 981 keys from HTTP `order.invalid`, mapped
to the owned loopback server using `--host-resolver-rules`; the recorded
`isSecureContext` and `crossOriginIsolated` are both false.
`window-secure-isolated-order.json` contains 1233 keys from the loopback page
served with COOP `same-origin` and COEP `require-corp`; both flags are true.
The three trials agree for each exposure, and their key sets exactly match
the corresponding frozen descriptor captures. Process arguments, SHA256,
probe expression and raw results are preserved in that capture directory.
