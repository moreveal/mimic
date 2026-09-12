# Saved VM payload review, 2026-09-12

This is a compatibility investigation of the saved `manual-20260912-045102`
programs against frozen **headful Chrome 152.0.7977.82**, using Mimic built at
`da4f93e`. It changes no browser implementation. The independently assigned
structured-clone repair is outside this report's implementation scope.

The review covers all ten captured pre-packer input/output pairs, including
root packet fields, not only the large child result objects. The accompanying
[field registry](manual-045102-field-registry.csv) contains **1,499 rows**, of
which **434 differ**, representing **76 distinct field names**. **100 differing
rows are repeat-stable in both runtimes**. Each differing field has an explicit
[review note](manual-045102-field-notes.json). This is field-level coverage:
nested graphics arrays are compared in full but not every component has a
minimal causal repro. `reviewed-…-unresolved` means inspected, not solved.

## Evidence boundaries

The saved responses were supplied to both runtimes, preserving original origins
and normalizing only the recorded dynamic path segment. The Chrome replay used a
deny-all proxy and intercepted fixture responses; successful runs recorded zero
proxy attempts and zero replay errors. Neither regenerated answers nor a later
server decision were sent to the live target. Replaying a saved continuation does
not prove what the server would return for a different answer.

Controls `chrome-a`/`chrome-b` and `mimic-probe`/`mimic-repeat` each have ten
complete packer pairs, all matched to POST bodies. The later broad `scan3` and
delegated `full` runs also have ten complete, request-matched pairs. Additional
instrumentation records host reads/calls and selected VM register changes; it
changes body sizes, timings, heap use, stacks and potentially task ordering.
The CSV retains baseline-versus-repeat and baseline-versus-scan equality flags.
It must not be used as a performance measurement or a pristine browser profile.

Physical VM register indices vary between instances. Instruction offsets below
refer to the first saved child or parent program, not general browser addresses.
Similar offsets in the second program are not automatically the same operation.
Earlier broken hooks (`branch`, first `reads`, first `scan`) are excluded from
causal evidence. Local synthetic oracles use fresh contexts, exact binary/version
checks, retained launch metadata and Chrome A/B observations.

## Individually localized observations

| Field/group | Generator and explanation |
| --- | --- |
| `IGBuA2` | Five extra indices correspond to **missing `console.dir`, `dirxml`, `table`, `trace`, and `Summarizer.availability`**. The previously inferred adjacent `debug`/`log`/`digest` mapping was wrong. Reads immediately before the pushes and local probes establish the corrected mapping. Common indices 159/163 do not distinguish the runtimes. |
| `MlWrD5` | The same missing `Summarizer.availability` changes the API-presence branch. Chrome resolves availability to `unavailable`; Mimic takes the missing-member path. This is not a second independent subsystem failure. |
| `oHIQ6[652]` | `Object.getPrototypeOf(fetch)` differs from `Function.prototype` in Mimic. The extra index is a prototype observation, even though the displayed native source string agrees. |
| `oHIQ6[690]` | `Object.setPrototypeOf(Function.prototype.toString, itself)` immediately throws `TypeError: Cyclic __proto__ value` in Chrome. Mimic succeeds, then the following `.toString()` lookup overflows with `RangeError`. The generator tests the exception-name length (9 versus 10). See the causal repro below. |
| `oHIQ6[699]` | A deliberately invalid receiver call captures an error stack, then searches for `at Object.toString (<anonymous>)` at IP106148. Mimic includes that frame through its source shim and takes the extra-index path. This is distinct from the cyclic-prototype predicate; `.caller` alone throws TypeError in both runtimes and does not explain index690. |
| Remaining 20 `oHIQ6` indices | Five checks for each of Window `crypto`, `isSecureContext`, `performance`, `speechSynthesis`: two source regex checks, two native-source substring checks, and getter name. Mimic exposes ordinary getter source and names such as `get`; Chrome exposes native getter source and `get <property>`. They are repeated checks of four getters, not twenty separate defects. |
| `SbVZ3` | The 90/62 token-length difference decomposes exactly: four missing console methods replace eight accessor-observation tokens with two catch tokens each (**−24**); console-log custom-`toString` observations add four Chrome-only tokens (**−4**). Other Error name/message accessor observations agree. Local probes confirm function/RegExp/Date preview coercion in Chrome and none in Mimic, while plain objects are not coerced in either. These observations were taken with `Runtime.enable`; do not generalize inspector preview effects to an unattached page. |
| `OjmeV1[75]` | Math.random-derived count; varies in repeats. It is not a stable independent defect. |
| `OjmeV1[82]` | `getComputedStyle(div).display` following construction of a styled div returns empty versus `flex`, changing a boolean. A detached-element local probe confirms a related empty-versus-default-display difference; the exact detached/hidden-frame fixture still needs reduction before a general CSS fix. |
| `OjmeV1[118/119]` | `structuredClone` returns objects for boxed BigInt/RegExp in Chrome and `undefined` in this Mimic build; subsequent reads take the error path. Do not describe this as necessarily a throw from `structuredClone` itself. The independent structured-clone task owns the repair and consumer regression suite. |
| `BMnw0`, `xWWV8` | `performance.memory` presence selects heap collection versus null. Heap amounts themselves vary. |
| `UYbIv2`, `knVv1` | Trusted Types CSP enforcement at eval and DOM/script sinks differs. The default-policy callback accumulator is 7 versus 0. Prior local iframe/worker probes distinguish raw strings and TrustedScript and confirm the enforcement/callback cause. |
| `ZkTjK2` | `CSSStyleSheet.cssRules[].cssText` feeds the hash. Keyframe whitespace and `scale3d` comma spacing differ before hashing; the local CSS oracle confirms serialization differences. |
| `NNZHC4` | `JSON.stringify(getComputedStyle(iframe.document.body))` feeds the hash. Indexed property-name order starts with `accent-color` versus `color`; the raw serialized inputs already differ. |
| `DdIVt1`, `uUOw3` | `StorageManager.getDirectory` rejects with backend unavailable in Mimic. Chrome continues through worker file creation, sync access handle, write/flush/close. These are one unsupported storage path plus its timing output. |
| `thhSb9`, `ppMls5`, `cHIoX8` | Image decode/size result: 46×23 and naturalWidth46 in Chrome versus fallback/0 in Mimic. These three fields share an image outcome. |

The five-index getter groups are `2736,2737,2746,2749,2751` (crypto),
`2752,2753,2762,2765,2767` (isSecureContext),
`2768,2769,2778,2781,2783` (performance), and
`2784,2785,2794,2797,2799` (speechSynthesis).

### Cyclic native-function prototype: concrete cause, not an error-name patch

The narrow VM trace captures `getPrototypeOf` at IP104340,
`setPrototypeOf(f,f)` at IP104351, and restoration at IP104380. Chrome skips
directly to the catch handler; Mimic executes another call before catching the
stack overflow. A synthetic oracle without target code reproduces it twice in
Chrome with zero A/B differences. Ordinary functions and Object.prototype.toString
correctly reject self-prototypes in both runtimes.

```js
const f = Function.prototype.toString;
const set = Object.setPrototypeOf;
const original = Object.getPrototypeOf(f);
try {
  set(f, f);       // Chrome: TypeError; this Mimic: succeeds
  f.toString();    // this Mimic: RangeError during cyclic lookup
} finally {
  set(f, original);
}
```

`internal/webapi/native_functions.js` replaces the engine function with a Proxy.
That proxy changes the observable prototype-cycle behavior as well as source and
stack observations. This localizes a structural source-wrapper problem; changing
the reported exception string would not repair it. No such change is made here.

## Aggregates, profiles and remaining causal limits

| Fields | Reviewed generator / remaining limit |
| --- | --- |
| `gsLi5` | Inverting the buckets gives 1,666 names on each side and ten differing values in the first large packet: two dynamic URLs, lastModified, webdriver, four visibility projections and outer dimensions. Bucket order does not imply missing APIs. |
| `XCvwf5`, `xVwa3` | Notification permission plus navigation sizes/serverTiming, secure-context/touch and Permissions query observations. PermissionStatus.name includes Chrome `video_capture`/`audio_capture` versus `camera`/`microphone`. Permission decisions require the selected profile; instrumentation also alters body sizes. |
| `zYUn5`, `CYsxg7`, `MfOHt6`, `RlMmu2`, `RPKTR7` | Connection estimates, visibility and storage quota. Repeat/environment differences are retained rather than called universal browser defects. |
| `TPpkV4`, `Vtvy6`, `hCfV6` | Per-stage finish, start and elapsed timestamps. All 79 stage triples per baseline runtime satisfy `finish - start = elapsed`; full trace also shows Date.now and saved start immediately before elapsed assignment. |
| `qIEN4`, `hCKCP8`, `jBVrk8`, `qdVjJ1`, `lIEA1`, `uZjT5` | Timing around callbacks/work/ICE. Replay and hook overhead prevent performance conclusions. |
| `Jpzg5`, `qydV6` | WebGL renderer/profile and extension projection (35 versus 4 extensions); several basic size/precision values agree. Individual nested parameter differences are not all reduced. |
| `etmnR7` | WebGL2 parameter fields0–33 agree in the first packet; context attributes differ in antialias and powerPreference (true/false, low-power/default). This is not a general numeric-limit discrepancy. |
| `OYbs6`, `KMUh5` | Context/parameter/readback observations and nested capability flags were inspected. A complete operation-to-subindex mapping is still missing. |
| `lDUiR4` | WebGPU adapter info agrees in the first packet; limits have 38/33 entries and differing values. The same twenty feature names have different iteration order; WGSL language features are populated/empty. Preferred format agrees. |
| `XYvy9` | After a 2×2 Canvas fill/arc/fill sequence, Chrome exposes intermediate edge pixels and Mimic hard255/0 values. TextMetrics widths/ascent/descent differ, with distinct metric counts8/2. Thumbnail readback hash also differs. Underlying shaping/raster formulas are not fully isolated. |
| `gAtz0`, `auHG6`, `zfym0` | Canvas/readback-derived results; the first two duplicate the same tokenized hash. Exact individual aggregate producers remain incomplete. No bad hash algorithm is established. |
| `ZwhIC5` | Both run shader draws, readPixels16×16 and4×1, TextDecoder.decode→TextEncoder.encode→SHA256 and register then/catch. Chrome executes success callback IP190033 and stores first32 hex digits. The targeted Mimic trace never enters that callback and retains the initial sentinel. This narrows the gap to continuation/result delivery; promise scheduling or another specific root is **not yet established**. It is not sufficient evidence to label WebGL unsupported. |
| `RKUE0`, `JRzmw6`, `oSIr8` | Prior operation traces identify SVG group bbox component sum, first-character extent sum, and distinct text-length sum/100000 before SHA256. Geometry/shaping inputs differ; all font/layout causes are not isolated. |
| `HPcn5` | Ten DOMRect observations of flow/table/control/extreme-transform fixtures; not ten independent root causes. |
| `MnIr8`, `CzUP6` | OfflineAudio startRendering/getChannelData aggregates and hashes. Duplicate first-cycle hash; the exact DSP source of the sample differences remains unresolved. |
| `HDEX5`, `Aqcaj8`, `XqEQ3`, `xkNI3` | canPlayType, MediaSource, mediaCapabilities and RTP capability results. RTP audio/video codec lists are populated8/23 versus empty. All codec support flags are not independently minimized. |
| `uHNG9`, `KnOhl5` | ICE/event/SDP observations; three m-lines versus one. Session IDs, ports and timing are variable. Chrome-only nonproxy-UDP restrictions make candidate counts unsuitable as a like-for-like network proof. |
| `maNnU6`, `rPXg2`, `Pdbt7` | Selector-access reentry and navigation/resource/mark entries. Earlier traces identify extra collection-selector reentry and root-document resource duplication; complete entry-source reconciliation remains open. |
| `lgWCE7`, `gPuVs8`, `PWGF4` | Serialized internal observation counters and stacks/timing. Not every originating public operation is isolated; native wrapper frames and instrumentation affect stacks. |
| `KDQCx4` | The child packet is0 in both. Parent copies `_cf_chl_opt.KDQCx4` at IP27188/27217, already0/1 before packing. Its earlier writer remains unlocalized. This is not an observed server verdict. |
| Root numeric metadata | `NnqX6`, `TzZRB1`, `ZMSOw0`, `twvE0`, `Blsob5`, `WHTpH6`, `eaaP6`, `poqG1`, `tZwbF3`, `uGyjw9`, `wOvYJ5`, `myWtu3` were compared across every packet/repeat. Exact expressions are still unlocalized; their variability alone is not proof they are only timings. |
| `jyDXx2` | A recorded response-status observation; equal in the first cycle. A cycle-specific difference is not a uniform API defect. |

## Reproduction and retained private evidence

Private evidence remains under `.build/compare-045102/` in the original checkout;
none of the response bodies, VM programs, target URLs, session tokens or raw
payloads are checked in. The supplemental `delegated-localization/` directory
holds narrow call/graphics steps, the full field scan and the cycle oracle,
including scripts, launch passports and raw output. Removing the analyst worktree
does not remove this evidence.

`tools/compatibility/payload_field_registry.py` regenerates the CSV from the six
`payload-probes.json` inputs (`chrome-a`, `mimic-probe`, `chrome-b`, `mimic-repeat`,
`chrome-scan3`, `mimic-scan3`) and the checked-in field notes. Pass them with
`--chrome`, `--mimic`, `--chrome-repeat`, `--mimic-repeat`, `--chrome-scan`,
`--mimic-scan`, then `--notes` and `--out`. It rejects mismatched packet counts,
distinguishes absent values from null and exports no payload values.

Validation for this documentation/tooling change: regenerated all1,499 rows;
confirmed ten request-matched pairs in both additional full scans; confirmed the
synthetic cyclic-prototype oracle has no harness errors and zero Chrome A/B
differences; inspected all differing field classifications. There is no claim
that all remaining root causes have been solved or that a live challenge now
accepts Mimic.
