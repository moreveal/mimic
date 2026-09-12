# RTC offer/session projection

Frozen headful Chrome 152.0.7977.82 A/B controls and current Mimic have zero
differences on the normalized session oracle in `.build/residual-rtc/direction-after`.
The committed probe/passport retains the full comparison. Random credentials,
fingerprints and origin session IDs are excluded from equality, not replaced
with captured values.

Offers now derive media sections, direction, transceivers, receivers and sender
identity from peer-owned state. Codec and feedback lines come from the shared
Go RTP catalog; bundled header-extension identifiers are allocated consistently
across media sections. Offers are ordinary description dictionaries, while
applied local descriptions remain RTCSessionDescription objects. Repeated offers
allocate tentative mids; applying a description assigns the corresponding mids.

Local signaling state changes occur asynchronously, dispatch the signaling event
before resolving the operation, then gather ICE for the offered media sections.
An empty offer does not fabricate ICE work. Candidate read-only properties parse
the same stored candidate string used by events and SDP; candidate counts and
addresses continue to derive from Environment. No native media/socket backend
or capture-specific payload is introduced.

Focused ordinary/restored frozen session and existing RTC state, entropy,
candidate profile and accessor regressions passed. This package covers the
confirmed initial-offer residual, not a new complete peer-to-peer transport.
