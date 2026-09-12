# Environment profiles (`Mimic.*`, schema version 1)

One JSON contract configures a new browser Context through CDP or the `--profile`
command-line option. A Context owns its initial environment and connection pool;
its pages, frames and workers inherit that environment. Full replacement requires
a new Context. Existing cookies, documents and JavaScript are not migrated.

## Example

Save as `profile.json`:

```json
{
  "schemaVersion": 1,
  "baseProfile": "chrome-152-windows-x64-headful-controlled-v1",
  "display": {
    "width": 1920,
    "height": 1080,
    "availableWidth": 1920,
    "availableHeight": 1040,
    "deviceScaleFactor": 1,
    "orientation": {"type": "landscape-primary", "angle": 0}
  },
  "window": {
    "outerWidth": 1280,
    "outerHeight": 800,
    "viewportWidth": 1280,
    "viewportHeight": 720
  },
  "hardware": {"logicalProcessors": 8, "deviceMemoryGB": 8},
  "locale": {
    "languages": ["en-US", "en"],
    "reduceAcceptLanguage": false,
    "timezone": "America/New_York",
    "intlLocale": "en-US"
  },
  "preferences": {"colorScheme": "dark", "reducedMotion": false}
}
```

```powershell
./.build/mimic.exe --profile profile.json --listen 127.0.0.1:9222
```

The file is validated before opening the listener. It supplies the default for
the initial Context and subsequent ordinary `Target.createBrowserContext` calls.
`Mimic.createContext` supplies its own profile independently of that default.
`baseProfile` must be the profile of the loaded Chrome bundle/mode; obtain its
exact name from `Mimic.getProfileSchema`. No other Chrome implementation is
installed by changing identity strings.

For CDP clients, `send` below sends a command on the browser WebSocket:

```js
const { schema, baseProfiles, limitations } =
  await send("Mimic.getProfileSchema", {});
const validated = await send("Mimic.validateProfile", { profile });
const { browserContextId } = await send("Mimic.createContext", {
  profile, disposeOnDetach: true
});
const { targetId } = await send("Target.createTarget", {
  browserContextId, url: "about:blank"
});
await send("Mimic.updateProfile", {
  targetId, patch: { window: { viewportWidth: 900, viewportHeight: 600 } }
});
const effective = await send("Mimic.getProfile", { targetId });
await send("Mimic.resetProfileOverrides", { targetId });
await send("Target.disposeBrowserContext", { browserContextId });
```

Attach with `Target.attachToTarget` to navigate or evaluate scripts using standard
page commands. Set the profile before creating/navigating the target so the first
document request and bootstrap see the intended values.

## Contract and ownership

`getProfileSchema` returns JSON Schema and `x-mimic-mutability` annotations.
Objects merge named members into the base; arrays replace their entire value.
Unknown fields, nulls and wrong types are errors. Fields use case-sensitive
lower-camel names, including `cpuPerformance`, `deviceMemoryGB`, `rttMillis`,
`graphics.webGPU`, and `graphics.webGLCapabilitiesJSON`.

`validateProfile` returns `profile` and `diagnostics` without creating a Context.
`getProfile` accepts exactly one of `browserContextId` and `targetId`; the latter
includes Page overrides. Responses are defensive projections, not mutable
references into the runtime. Internal custom profile IDs are content-derived and
are not represented as the frozen reference profile's identity.

`updateProfile` accepts only dynamic fields. A forbidden member returns a CDP
error with `data.path`, `data.reason: "requiresNewContext"` and `data.message`;
no members of that patch take effect. Reset restores the Page's own Context
baseline. Already sent requests retain their original headers.

Existing Workers retain their startup navigator identity and languages, as in
the controlled Chrome probe; newly constructed Workers inherit the updated Page.
UA overrides do not synthesize a `languagechange` event. Media-query changes
notify registered `MediaQueryList` listeners through the Page task queue.

| Group | Supported changes | Boundary |
| --- | --- | --- |
| `display`, `window` | Dynamic screen/available area, DPR, orientation, position, outer size and viewport; color depth at creation | Positive coherent dimensions; no mobile layout |
| `identity` | Dynamic `userAgent`, `platform`, `metadata` Client Hints | Full profiles retain the loaded desktop Chrome identity; standard CDP remains permissive |
| `hardware` | CPU count, memory bucket, CPU performance class at creation | Observations, not allocation of physical CPU/RAM |
| `locale` | Languages dynamically; language reduction, IANA timezone and Intl locale at creation | Custom timezone/Intl locale requires the native Intl (V8) backend; experimental engines reject rather than simulate formatting |
| `graphics` | Vendor, renderer, maximum texture size at creation | Custom capability JSON and WebGPU adapter overrides reject; graph/pixel approximations remain documented |
| `fonts` | Read baseline selection | Custom selections reject until resource validation is implemented |
| `preferences` | Theme/reduced motion dynamically; DNT at creation | Existing modeled preferences only |
| `network` | Existing connection observations, cookies, ICE and proxy at creation | Wire profile stays coupled to the loaded bundle; metadata does not provide a real device or public IP |
| `permissions` | Initial decisions for existing baseline permission names | Live permission state stays in the origin capability store |
| `capabilities` | Existing quota and keyboard-layout settings | Custom device/media backends reject |
| `features` | Restrict existing exposure flags | Cannot enable an uncaptured feature |
| `timing` | Existing execution/navigation/network scale factors at creation | No arbitrary wall-clock or monotonic epoch injection |

Unsupported fields may be read or round-tripped unchanged; their custom values
return `unsupported` with a reason. The schema describes their shape without
claiming that every value is executable. Browser security still controls exposure:
for example, `navigator.deviceMemory` is not exposed in an insecure document.

Date local construction, parsing, getters/setters and formatting use the Context
zone, including DST gaps/repeats. UTC methods and stored timestamps remain native
engine values. Window, frames and workers share the same implementation; neither
the host timezone nor process-global ICU defaults are changed. Eight cached
transition intervals bound realm-local lookup overhead. Default Intl formatting
and Date/Number/BigInt locale methods use `intlLocale`, independently of the
language list. Explicit formatter arguments still take precedence. These two
fields require a new Context, not an in-place update of existing documents.

The US example changes language/time observations, not network geolocation. A
US public IP requires a real US proxy. Other unsupported resource/device/graphics
fields remain explicit boundaries; accepting arbitrary JSON is not compatibility.

## Proxy

Add this group to the input profile:

```json
{
  "network": {
    "proxy": {
      "server": "socks5://127.0.0.1:1080",
      "username": "user",
      "password": "password"
    }
  }
}
```

HTTP and HTTPS proxy URLs are also supported. Keep credentials in their separate
fields, not in the URL. Empty `server` means no explicit proxy. Default ports are
80/443/1080. Bypass lists are not supported.

Each Context uses its own pool. HTTP(S) proxies use CONNECT; SOCKS5 sends the
destination hostname to the proxy. The origin still uses the selected Chrome
TLS profile. HTTPS proxy certificates use system trust independently of any
explicit origin certificate override. Proxy mode disables HTTP/3/QUIC. Connection
or authentication failure never falls back to a direct request.

Credentials are omitted from validation/read/update responses and diagnostics;
retain the original input privately if recreating an authenticated profile.
Proxy settings route resource-loader HTTP(S) requests, including Worker fetch;
they do not configure UDP/WebRTC routes. ICE settings describe modeled
observations and must not be interpreted as a public-IP change.

## Standard CDP compatibility

`Emulation.*` is a standard Chrome CDP domain. Existing commands and
`Mimic.setViewport` remain available and operate on the same Page environment.
They do not gain extra custom parameters. Standard metrics overrides preserve
Chrome's outer-window semantics and expose the whole emulated screen as available;
the full Mimic profile instead lets callers specify these dimensions explicitly.

The focused native probe is `tools/compatibility/profile_contract_oracle.py`.
It launches an owned frozen headful Chrome, creates a disposable context and
captures baseline, metrics, identity/media and reset observations. It never
modifies the frozen performance harness or reference expectations.
