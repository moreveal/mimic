# Geolocation emulation

Mimic supports the navigator Geolocation API with a Page-owned emulated provider.
It does not connect to an operating-system location service. Without an override,
a permitted request reports `POSITION_UNAVAILABLE`; a headful permission prompt
remains pending until a permission decision is supplied.

## Protocol commands

- `Emulation.setGeolocationOverride` sets latitude, longitude and accuracy.
  Optional altitude, altitudeAccuracy, heading and speed are supported.
- Missing any of latitude, longitude or accuracy emulates an unavailable position.
- `Emulation.clearGeolocationOverride` removes the emulated provider.
- The deprecated `Page.setGeolocationOverride` and
  `Page.clearGeolocationOverride` share the same underlying state.

Overrides are isolated between Pages and persist across document navigation.
Invalid coordinates are rejected before replacing the current override.

Geolocation permission is independent of provider state. CDP
`Browser.setPermission` and `Browser.grantPermissions` support geolocation grants;
grants of other permission types are explicitly unsupported. A grant applies to
the selected browser context and optional origin, and denies other permissions.
`Browser.resetPermissions` restores the context's environment defaults.

## JavaScript behavior

`getCurrentPosition`, `watchPosition` and `clearWatch` use the Page event loop.
Active watches receive provider changes. Secure-context requirements, the
self-default Permissions Policy and permission denial are enforced. Clearing a
watch cancels callbacks that have been queued but not delivered.

Position and coordinate wrappers expose the native interface prototypes,
nullable optional coordinate fields, timestamps and `toJSON`. A fresh cached
position may satisfy `maximumAge`, including a request with `timeout: 0`;
otherwise a zero-timeout request reports `TIMEOUT`. `enableHighAccuracy` is
accepted but does not alter explicitly supplied provider coordinates.

## Example with Playwright

```javascript
const context = await browser.newContext();
await context.grantPermissions(['geolocation'], { origin: 'https://example.com' });
const page = await context.newPage();
const cdp = await context.newCDPSession(page);
await cdp.send('Emulation.setGeolocationOverride', {
  latitude: 41.7151,
  longitude: 44.8271,
  accuracy: 10,
});
await page.goto('https://example.com');
const position = await page.evaluate(() => new Promise((resolve, reject) => {
  navigator.geolocation.getCurrentPosition(p => resolve(p.toJSON()), reject);
}));
```

The focused oracle was measured on a local HTTPS document using frozen Chrome
152.0.7977.82. It covers successful coordinates, nullable fields, empty override,
invalid latitude, heading 360, cache identity and zero timeout. Regression tests
also cover watches, cancellation, Page isolation, navigation, permission reset,
Permissions Policy, and the CDP commands.
