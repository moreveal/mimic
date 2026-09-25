# Context profiles

Mimic exposes one current contract. Use `Mimic.getVersion` to identify the Mimic
release/build, separately from `chromeVersion`. There is no contractVersion.
The generator uses one coherent installed Chrome 152 device recipe. It is not a
catalog of all physical machines.

```javascript
const { browserContextId, profile, warnings } = await cdp.send("Mimic.createContext", {
  profile: {
    generate: { browser: "chrome", version: 152, platform: "windows", seed: "account-1842" },
  },
  proxy: { server: "socks5://host:1080", username: "user", password: "pass" },
  resourcePolicy: { presets: ["noVisualAssets", "noSpeculativeLoads"] },
  disposeOnDetach: true,
});
```

`profile` in the response is a portable string. Save it and pass it as the
`profile` parameter of another createContext call. It contains no proxy credentials,
context ID, cookies or storage. No server-side profile registry is created.
Treat it as opaque; use export/import to inspect or store JSON.
Omitting profile means random generation. Omitting seed uses 256 random bits;
explicit empty seeds, unknown selectors/fields, null and wrong types fail.
Version/platform filter the installed bundle; they do not install a browser.
`Target.createTarget` without `browserContextId` keeps ordinary CDP behavior:
the new Page belongs to the default Context and shares its cookies and storage.
For separate accounts or machines, create one Mimic Context per identity and
pass its `browserContextId` when creating the Page. Closing the Page alone does
not dispose that Context; dispose it explicitly when the task finishes.

```javascript
const generated = await cdp.send("Mimic.generateProfile", { seed: "account-1842" });
const exported = await cdp.send("Mimic.exportProfile", { profile: generated.profile });
const imported = await cdp.send("Mimic.importProfile", { profile: exported.profile });
const context = await cdp.send("Mimic.createContext", { profile: imported.profile });
```

Generation/import/export allocate no browser Context or Page. Restore recomputes
and validates the environment. An unavailable base or changed resolution returns
`incompatibleProfile`; no old generator implementation is selected. A saved seed
alone does not promise the same output across releases. profileId hashes the
resolved environment rather than seed, so collisions in generated configurations
are visible. Tokens and hashes are not signatures or access-control credentials.

## Explicit manual mode

```javascript
const { profile, warnings } = await cdp.send("Mimic.importProfile", {
  mode: "manual",
  profile: {
    hardware: { logicalProcessors: 8, deviceMemoryGB: 8 },
    window: { outerWidth: 1280, outerHeight: 800, viewportWidth: 1280, viewportHeight: 720 },
    locale: { languages: ["en-US", "en"], intlLocale: "en-US", timezone: "America/New_York" },
    audio: { sampleRate: 44100 },
    preferences: { colorScheme: "dark", reducedMotion: false },
  },
});
const context = await cdp.send("Mimic.createContext", { profile });
```

Manual mode explicitly warns against inconsistent surfaces. Known invalid values
and relations fail; the warning is not an override. Only modeled fields work.
Identity, graphics and fonts must retain the installed baseline until complete
alternative recipes can be validated. Arbitrary hardware realism and every
cross-surface relation are not yet certified. Custom timezone/Intl locale requires
native Intl (V8). IANA timezone and language do not infer the proxy's geography.

Manual input has no schemaVersion/baseProfile and cannot contain network.proxy.
It merges named fields into the installed baseline; arrays replace whole arrays.
Exported descriptors can be restored without mode. To deliberately edit one,
pass its environment member through explicit mode: manual; do not rewrite its hash.
Manual export includes normalized environment data and can be substantially larger
than a generated token. Tokens are currently limited to 1 MiB on restoration.

## Lifecycle, emulation and network

Profile, proxy and policy validate before publication. Create returns
browserContextId, profile, profileId, mode and warnings. Use Target.createTarget,
Target.attachToTarget and ordinary page commands, then
Target.disposeBrowserContext in finally. disposeOnDetach refers to the creating
browser connection, not an individual Page attachment.

Generated/imported Context environments are immutable. UA, metrics,
locale, timezone, theme and viewport mutation commands return profileLocked before
modification. Resize currently requires a new Context. This may conflict with
client-library automatic emulation: raw CDP is the tested integration path.
Ordinary Target-created contexts retain standard mutable CDP behavior.
Mimic.getProfile reads effective observations; it is not accepted as create input.
Existing update/reset commands only change ordinary mutable contexts.
This is not a sandbox against user scripts or request interception: these can
still replace script/network observations. Managed-profile CDP mutation paths,
including generic-font overrides, reject changes to the selected environment.

--profile and the browser ProfileJSON option have been removed. Profile JSON is
only a CDP import/export format. Mimic.validateProfile is replaced by importProfile.
ResourcePolicy follows the current Mimic release contract without a version
selector; `scraping` is not a preset.
Blocking images/fonts changes observable behavior and must be workload-tested.
Resource byte budgets do not cap process RSS, DOM, V8 or native allocation.

HTTP, HTTPS and SOCKS5 proxy credentials use separate fields, not URL userinfo.
The origin Chrome transport remains coupled to the installed bundle. Proxy
failure does not fall back to direct requests; proxy mode disables HTTP/3.
HTTP(S) resource-loader paths and Worker fetch use the Context proxy. Do not infer
UDP/WebRTC routing from ICE metadata; broader proxy coverage remains an audit item.
Credentials are omitted from profile reads and errors.

## Current diversity and RAM limits

The generated recipe varies viewport dimensions, window position, theme, reduced
motion and output-device sample rate (44.1 or 48 kHz). Window insets and the
audio latency recipe remain coupled to their installed defaults. Hardware,
browser identity, display, locale, graphics, fonts and wire profile stay at the
captured baseline. On the current 2560×1440 screen, the Cartesian upper bound is
**3,612,980,639,528 configurations (~41.7 bits)**: the sum of 1..1753 legal
horizontal placements times the sum of 1..766 vertical placements times eight
preference/audio choices. The 256-bit random seed makes random generation
unpredictable; it does not imply 2^256 observable fingerprints. Different seeds
can still produce the same profileId, and a site's smaller observation set may
collapse many configurations. Graphics readbacks use canonical modeled state,
without per-seed random noise. Distinct generated profiles do not imply distinct
GPUs, font inventories, public IPs or network identities.

For 100 jobs, limit live contexts (for example to 8), create on admission, stream
results and close in finally. For 100 simultaneously live pages, expect a much
larger memory footprint. Profile-specific bootstrap code is now shared across
contexts whose exposure graph is the same; each Page still owns its realm and
event loop. The first managed Page for a new security/exposure graph waits for
its bounded snapshot to be prepared. This trades one-time startup latency for
predictable live memory when many identities start together. See the
[density report](performance/profile-context-final-20260925.md) and
[implementation history](public-context-contract-plan.md).
