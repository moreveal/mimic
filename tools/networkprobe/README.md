# Chrome 152 direct network probe

This probe measures the frozen Windows Chrome `152.0.7977.82` against a local
TLS / HTTP/3 server. It records received QUIC Initial datagrams, reassembles and
decrypts their public Initial CRYPTO data, and captures HTTP/3 control streams
at the server. TCP ClientHellos come from Chrome NetLog. It does not infer a
wire profile from a browser user-agent string.

Requirements: Go, Python with `cryptography` and `websockets`, Windows
`certutil`, and the pinned Chrome for Testing binary. Run from the repository
root. Each output directory must be new.

```powershell
go build -o .build/networkprobe.exe ./tools/networkprobe
python tools/networkprobe/capture.py --chrome compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe --server .build/networkprobe.exe --output .build/wire-h3 --force-quic --raw-h3 --mimic
python tools/networkprobe/compare.py .build/wire-h3
python tools/networkprobe/capture.py --chrome compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe --server .build/networkprobe.exe --output .build/wire-tcp --tcp-only
python tools/networkprobe/freeze.py --quic-dir .build/wire-h3 --tcp-dir .build/wire-tcp --chrome compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe --output internal/network/testdata/chrome152_network_wire.json
```

The capture starts a fresh headful profile. It temporarily trusts an ephemeral
loopback certificate in **CurrentUser Root**, then removes it in `finally` and
closes its browser and server. QUIC requires `--origin-to-force-quic-on` because
ordinary Chrome QUIC rejects locally trusted roots. Metadata retains the exact
flags, browser version/hash, probe hash, viewport and security state. The
unforced cold ClientHello was separately checked against the forced capture.

The raw server closes completed connections to obtain a fresh resumed handshake.
The Mimic client performs cold, reused, then resumed requests with certificate
verification enabled. `compare.py` checks both ClientHellos, JA4 ingredients,
transport parameters, SETTINGS order/values and control-frame ordering. Request
priorities are reported separately: Chrome navigation/fetch and the plain Go
GET workload have different priorities. Local regressions check propagation of
the actual request priority and stream ID.

Normalization removes random key material, PSK identities/binders, GREASE
values, extension/transport-parameter permutations, trust-anchor permutation,
and the numeric value of the measured positive RTT. It preserves extension
membership, ordered ciphers/groups/signatures, key-share sizes, transport
parameters and ordered SETTINGS. GREASE frame length must agree with the random
frame type, not just fall within a permissive range.

Raw NetLogs can include unrelated Chrome background traffic and session secrets;
keep them in ignored `.build` directories. The committed fixture contains only
normalized loopback evidence and capture metadata. Do not replace frozen
expectations merely to make implementation tests pass.

See `docs/compatibility/chrome152-network-profile-2026-09-11.md` for the verified
boundary and remaining limitations.
