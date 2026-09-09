"""Produce an honest matrix from retained observations, without normalizing diffs."""
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
def read(path): return json.loads((ROOT/path).read_text(encoding='utf-8'))

GROUPS = [
 ('Legacy identity','vendorSub productSub appCodeName doNotTrack','semantics implemented'),
 ('Input','userActivation scheduling keyboard virtualKeyboard ink','capability-only; input activation implemented'),
 ('Permissions / credentials','permissions credentials clipboard geolocation','capability-only; shared permission store implemented'),
 ('Network / storage','connection storage storageBuckets locks webkitTemporaryStorage webkitPersistentStorage','semantics implemented for tested operations; see limits'),
 ('Media','mediaDevices mediaCapabilities mediaSession presentation','capability-only'),
 ('Devices','bluetooth hid serial usb devicePosture','capability-only; transport unsupported by design'),
 ('Browser services','serviceWorker wakeLock windowControlsOverlay xr managed login','capability-only; service backends unsupported by design'),
 ('Privacy','protectedAudience deprecatedRunAdAuctionEnforcesKAnonymity','capability-only; auctions unsupported by design'),
]
PREFIXES={
 'userActivation':['activation'], 'scheduling':['activation'], 'connection':['connection'],
 'geolocation':['geolocation.'], 'permissions':['permissions.'], 'clipboard':['clipboard.'],
 'credentials':['credentials.'], 'bluetooth':['bluetooth.'], 'hid':['hid.'], 'serial':['serial.'], 'usb':['usb.'],
 'devicePosture':['posture'], 'storage':['storage.'], 'storageBuckets':['buckets.'], 'locks':['locks.'],
 'webkitTemporaryStorage':['quota.temporary','quota.identity'], 'webkitPersistentStorage':['quota.persistent','quota.identity'],
 'mediaDevices':['media.enumerate','media.getUserMedia','media.invalid','media.constraints'],
 'mediaCapabilities':['media.decoding','media.mp4'], 'mediaSession':['media.session'], 'presentation':['presentation'],
 'serviceWorker':['serviceWorker.'], 'xr':['xr.'], 'wakeLock':['wakeLock.'], 'keyboard':['keyboard.'],
 'virtualKeyboard':['virtualKeyboard'], 'ink':['ink'], 'managed':['managed'], 'login':['login.'],
 'windowControlsOverlay':['overlay'], 'protectedAudience':['protectedAudience'],
}

def main():
    chrome=read('compatibility/captures/navigator-chrome152.json')
    mimic=read('compatibility/captures/navigator-mimic-current.json')
    baseline=read('compatibility/captures/navigator-mimic-baseline.json')
    catalog=read('chrome/152/generated/webapi.json')
    declarations={d['name']:d for d in catalog['declarations']}
    nav={m.get('name'):m for m in declarations['Navigator']['members']}
    contexts=['secure','isolated','opaque']
    operations=chrome['secure']['operations']
    differences={k:{'chrome':v,'mimic':mimic['secure']['operations'].get(k)} for k,v in operations.items() if v!=mimic['secure']['operations'].get(k)}
    opaque_operations=chrome['opaque']['operations']
    opaque_differences={k:{'chrome':v,'mimic':mimic['opaque']['operations'].get(k)} for k,v in opaque_operations.items() if v!=mimic['opaque']['operations'].get(k)}
    names=[n for _,members,_ in GROUPS for n in members.split()]
    shapes=lambda data:sum(chrome[c]['members'][n]==data[c]['members'][n] for c in contexts for n in names)
    rows=[];provenance=[]
    for group,members,scope in GROUPS:
        for name in members.split():
            c=[chrome[ctx]['members'][name] for ctx in contexts];m=[mimic[ctx]['members'][name] for ctx in contexts]
            exposure=lambda values:'/'.join('yes' if row['exposed'] else 'no' for row in values)
            descriptor=all(a.get('descriptor')==b.get('descriptor') and a.get('prototypeDescriptors')==b.get('prototypeDescriptors') for a,b in zip(c,m))
            prototype=all(all(a.get(k)==b.get(k) for k in ['owner','chain','prototype','constructor','constructorIdentity','tag','sameObject','ownKeys','illegalReceiver']) for a,b in zip(c,m))
            failed=['secure: '+key for key in differences if any(key.startswith(prefix) for prefix in PREFIXES.get(name,[]))]
            failed+=['opaque: '+key for key in opaque_differences if any(key.startswith(prefix) for prefix in PREFIXES.get(name,[]))]
            values='matched in probes' if not failed else 'differences: '+', '.join(failed)
            rows.append(f'| `{name}` / {group} | {exposure(c)} | {exposure(m)} | {"yes" if descriptor else "NO"} | {"yes" if prototype else "NO"} | {values} | {scope} |')
            member=nav[name];interface=declarations.get(member.get('type'),{})
            provenance.append({'member':name,'domain':group,'type':member.get('type'),'navigatorExposure':declarations['Navigator']['extended'],
              'declaringSource':member.get('origin'),'memberConditions':member.get('extended'),
              'interfaceSource':interface.get('source'),'interfaceConditions':interface.get('extended'),
              'chromeExposed':dict(zip(contexts,[v['exposed'] for v in c]))})
    report=f'''# Navigator capability domains — Chrome 152

Reference: `{chrome['product']}`, Windows x64, headful, fresh controlled profile, no feature overrides; synthetic local pages. `Runtime.evaluate` uses `userGesture:false`. Network and OS-service availability reflect machine state, not constants of the Chrome version.

Contexts in the exposure columns: **secure / secure+COOP+COEP / opaque about:blank**. Additional insecure Window data was captured on `data:`. This report does not certify other profiles, origin trials, PWAs, managed apps, or iframe permissions.

Full shape matched in **{shapes(mimic)}/{len(names)*len(contexts)}** observations; original HEAD `{baseline.get('sourceRevision','recorded baseline')}`: **{shapes(baseline)}/{len(names)*len(contexts)}**. In the secure context, **{len(operations)-len(differences)}/{len(operations)}** operation results matched; in opaque, **{len(opaque_operations)-len(opaque_differences)}/{len(opaque_operations)}**. For isolated contexts, only shape was checked here, not operations. This is a limited set of observations, not a browser compatibility percentage.

Checks cover owners, descriptors, native Function#toString, name/length, function own keys, non-constructible methods/getters, readonly constructor.prototype, inheritance chains, constructor identity, same-object behavior, absence of leaked own fields, and illegal receivers.

| member / domain | Chrome exposed? | Mimic exposed? | descriptor parity | prototype parity | capability/value parity | semantics / capability-only / unsupported |
|---|---|---|---|---|---|---|
'''+ '\n'.join(rows)+'''

## State and boundaries

- Legacy: fixed Blink rules and the DoNotTrack preference; these are not values taken from a site.
- Input: trusted `Page.DispatchInput`, the UserInteraction queue, and Scheduler time; synthetic dispatchEvent does not activate the page. Supports mousedown/keydown/touchend, expiry, and sticky activation. This is not complete CDP Input, hit-testing, or the full activation propagation/consumption algorithm. KeyboardLayoutMap reads the layout from Environment. No actual keyboard lock, virtual keyboard, or ink renderer is provided.
- Permissions: one origin store in Context, shared by query, location, clipboard, media, and keyboard; PermissionStatus changes are delivered as tasks. The full set of specialized descriptor algorithms (MIDI sysex, requestedOrigin, fullscreen, and others) is not implemented. Credential transport, WebAuthn/FedCM, and ClipboardItem transport are absent. Location does not fabricate a position without a backend.
- Network/storage: connection reads Environment.Network. Quota uses a shared profile; as in Chrome, Web Storage is excluded from StorageManager usage. Bucket metadata/lifecycle, expiry, and persistence are supported. Quota-consuming IDB/Cache/OPFS clients are absent: estimate is not real disk usage. Locks supports queuing, shared/exclusive modes, callback lifetime, ifAvailable, abort, and release on realm closure; steal is explicitly unsupported. The two legacy quota getters return the same object without a public constructor.
- Media: one model of installed device classes, redaction, and capability profiles. No streams are created. Codec negotiation is limited to configured content types and tested configurations; this is not a complete decoder/encoder/EME, media pipeline, or performance test of arbitrary resolutions/bitrates. MediaSession stores playback/action/position state; MediaMetadata/PresentationRequest constructors and Presentation transport are not implemented.
- Devices: one empty transport registry, not independent successful devices. HID/USB/Serial return empty lists; a chooser without activation is rejected. Bluetooth getDevices is absent in the selected Chrome profile. On this machine, Bluetooth availability sometimes does not settle within 1500 ms and sometimes returns false; Mimic reports the absence of an adapter immediately.
- Browser services: no service-worker execution, XR session backend, presentation receiver, managed configuration, or system power backend. Controller/registration queries describe an empty state; ready remains pending. Unsupported operations explicitly reject instead of resolving to null through semanticMissing. Chrome with a power backend returns WakeLockSentinel; Mimic rejects the request. This is an intentional difference, not demonstrated Chrome parity. Window controls overlay is disabled for a regular window. Login status is stored per origin.
- Privacy: observed capability responses are supported; the auctions/interest-group backend is not implemented. Related Navigator operations from the same IDL source explicitly reject their Promise instead of resolving to null through semanticMissing. deprecatedRunAdAuctionEnforcesKAnonymity is false in the selected profile.

Exposure is selected from the pinned snapshot and may then be restricted by explicit Environment.Features. An unknown or enabled override does not add APIs beyond the snapshot. Sources and conditions for partial/mixin declarations and conditional Exposed(Window Feature) are preserved; provenance is not replaced with merged Navigator conditions.

## Verifiable artifacts

- [Headful Chrome capture](../compatibility/captures/navigator-chrome152.json), [mode-specific headless capture](../compatibility/captures/navigator-chrome152-headless.json), [original HEAD](../compatibility/captures/navigator-mimic-baseline.json), [Mimic after changes](../compatibility/captures/navigator-mimic-current.json).
- [Oracle policy](oracle-policy.md) and [headless expectations audit](oracle-headless-audit.md).
- [Exact operation differences](navigator-capability-differences.json), [provenance and conditions for each member](navigator-capability-provenance.json).
- [Shared capture scenario](../compatibility/navigator_capabilities.py), [probes](../compatibility/probes-navigator-capabilities.json), [regression tests](../internal/browser/capabilities_test.go).
- [109 pinned IDL sources](../chrome/152/generated/navigator-idl-sources.json), [reproducible reconstruction](../tools/refresh_navigator_idl.py), [parser tests](../tools/test_generate_compat.py).

The matrix does not imply complete implementation of every method on these interfaces. `capability-only` and `unsupported by design` remain explicit boundaries even when shape matches completely.
'''
    (ROOT/'docs/navigator-capabilities.md').write_text(report,encoding='utf-8')
    for name,value in [('navigator-capability-differences.json',{'secure':differences,'opaque':opaque_differences}),('navigator-capability-provenance.json',provenance)]:
        (ROOT/'docs'/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print(f'Shape {shapes(mimic)}/{len(names)*len(contexts)}; operations {len(operations)-len(differences)}/{len(operations)}; baseline shape {shapes(baseline)}')

if __name__=='__main__':main()
