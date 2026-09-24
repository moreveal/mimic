# Blazor Server Interactive probe

This local .NET 10 app exercises a real SignalR circuit and server-rendered DOM
patches. It covers clicks, `oninput` binding, keyed list changes, delayed event
handlers, select and checkbox binding, form validation, JS interop in both
directions, enhanced navigation, streamed server rendering, and two independent
circuits.

From the repository root, start the app and a fresh Mimic build in separate
terminals:

```powershell
$env:ASPNETCORE_ENVIRONMENT = 'Development'
dotnet run --project tools/compatibility/blazor_server_probe --urls http://127.0.0.1:5187
```

```powershell
go run ./tools/runmimic -listen 127.0.0.1:9222
```

Install the pinned Playwright client with `npm ci` in `examples`, then run:

```powershell
node examples/blazor_server_probe.mjs
```

The runner uses the repository's frozen Chrome 152.0.7977.82 binary on Windows.
Set `CHROME_PATH`, `TARGET_URL`, or `MIMIC_URL` to override its defaults. It
prints each step, the final DOM values, and differences, and exits nonzero when
Mimic differs from Chrome. It is a diagnostic workload, not part of the full Go
test suite.

## 2026-09-24 checkpoint

Chrome 152 completed all 13 steps. A fresh Mimic build completed circuit
startup, counter updates, input binding, keyed list changes, async updates,
checkboxes, both JS interop directions, and navigation through the streamed
page. Two compatibility issues appeared:

- Playwright `selectOption('two')` changes the DOM value, but Mimic emits no
  `input` or `change` event. Blazor's server state remains `one`. Explicitly
  dispatching `change` afterward updates the state to `two`. This points to
  script-created event dispatch across the Playwright isolated world and the
  page's event listeners.
- Form submission after filling the email is intermittent. In a failing run,
  `change` and `blur` fire for the email, and the Save button receives
  `focusin`, but that click has no `mouseup`, `click`, or `submit`. Explicitly
  blurring the email before clicking Save makes the submission succeed. This
  points to input event ordering while the server applies the validation patch.

These are observations from this workload, not site-specific fixes.
