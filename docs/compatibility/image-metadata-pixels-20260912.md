# Image metadata and pixel availability

The saved image failure was a PNG with valid intrinsic metadata and an invalid
zlib stream. Independent synthetic PNG controls reproduce the same general
Chrome contract: Image load and decode resolve, natural dimensions remain
available, drawImage makes no pixel change, and createImageBitmap rejects an
unavailable pixel stream. A complete pixel stream with only a bad Adler trailer
is accepted and remains readable.

The image resource now records intrinsic metadata separately from pixel
availability. PNG repair is limited to a verified Adler checksum failure after
successful complete decompression; it neither fabricates pixels nor accepts an
invalid header. Canvas and ImageBitmap consume that shared availability state.
Dimension and allocation bounds remain enforced.

Frozen Chrome 152.0.7977.82 headful A/B controls and Mimic have zero differences
for valid, raw-deflate, invalid-header, invalid-checksum and truncated streams,
including load/error events, decode, dimensions, Canvas and bitmap observations.
Raw evidence is `.build/residual-images/pixels-after`; the synthetic probe and
passport are committed with the focused ordinary/restored regression. No saved
capture image or its dimensions are embedded in production or the test fixture.
