//go:build windows && amd64

package prebuilt

import _ "embed"

const (
	ABI      = 44
	Size     = int64(45_930_496)
	SHA256   = "919522b4d4ed80671586a1b7a0efc144a533e91e08319715e76ed27962cee0ff"
	fileName = "gov8_shim.dll"
)

//go:embed windows_amd64/gov8_shim.dll.gz
var compressed []byte
