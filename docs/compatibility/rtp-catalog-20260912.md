# RTP capability catalog — frozen Chrome 152, 2026-09-12

The xkNI3 residual had no RTP capability implementation. One profile catalog now
owns codec profiles, sender/receiver order, preferred payload assignments,
retransmission associations, feedback and header extensions. Public capability
objects and SDP media metadata are independent fresh projections of that catalog.
No captured session description, ICE state or VM field value is used in production.

Frozen headful Windows Chrome 152.0.7977.82 A/B controls show eight audio codecs
for both roles, sixteen video sender codecs, and twenty-three video receiver
codecs. The receiver supports additional VP9/H264/AV1 profiles and flexfec.
H264 high-profile sender capability allows level 0x33 while the initial offer
and receiver advertise 0x1f; this distinction is recorded as codec policy.
RTX payloads are generated from each codec's preferred retransmission assignment;
`apt` always points to the associated media payload. Audio RED derives its
payload mapping from the Opus codec entry. Extmap IDs follow catalog order.

`host.rtpMedia(kind)` returns `{payloads, extensions, attributes}` for the parent
RTC negotiation implementation. This commit does not replace RTCPeerConnection
or negotiate sessions. `RTCRtpSender/Receiver.getCapabilities` use the same catalog
and return fresh dictionaries; unknown kinds return null and missing arguments
throw the measured TypeError.

Validation:

- Fresh Chrome A/B capabilities controls: zero differing leaves.
- Fresh production capabilities versus Chrome: zero differing leaves.
- Ordinary and restored-bootstrap browser oracle passes.
- SDP media projection matches frozen audio/video offer codec, feedback and
  extension lines exactly, including payload uniqueness and valid payload range.
- `go test ./internal/browser -run 'TestRTP' -count=1` passes.

Committed oracle metadata identifies Chrome/V8/revision, binary and probe hashes,
fresh profile/contexts and actual window/origin state. Full local raw evidence,
runner and measured binary are preserved at
`E:/GitHub/mimic/.build/residual-media-delegated/rtp-capabilities` and `rtp-sdp`.
The raw SDP capture compares against the branch's pre-existing negotiation
implementation; only the catalog projection is asserted here until the parent
integrates its negotiation work. Codec availability is frozen profile policy,
not a claim that Mimic performs hardware media encoding/decoding. No full suite,
race gate or performance matrix was run.
