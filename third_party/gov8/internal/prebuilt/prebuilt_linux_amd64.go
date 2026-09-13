//go:build linux && amd64

package prebuilt

import _ "embed"

const (
	ABI      = 44
	Size     = int64(57288784)
	SHA256   = "218b113dbf49d0e7b46b6f009bd9924cc83bf8dbf98e2904891babbc2ef7c140"
	fileName = "libgov8_shim.so"
)

//go:embed linux_amd64/libgov8_shim.so.gz
var compressed []byte
