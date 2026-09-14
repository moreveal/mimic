# Mimic v0.1.2

Changes since v0.1.1:

- Playwright can now click actionable controls inside iframes and observe
  asynchronously opened popup pages through the normal `popup` event.
- Network request events expose POST bodies in both legacy `postData` and modern
  `postDataEntries` forms for compatibility across Playwright versions.
- DOM/CDP integration now resolves iframe owner nodes, rendered `innerText`,
  customized built-in elements, and `ReportingObserver` lifecycle behavior.
- Element resource loading, preload cancellation, CSS-connected font loading,
  and nested-frame focus now share their canonical page state.
- Observable Canvas, Web Audio, WebGPU, CSS, and HTML attribute behavior has
  been expanded while preserving realm isolation and coherent readbacks.
- Child-process shutdown now stops the Mimic server, and the development preview
  projects child frames by their canonical owner identity.
