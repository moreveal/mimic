//go:build mimic_profile_cgo

package browser

// Match the production executable's CGO linkage. The normal browser test
// binary does not link QuickJS; Windows native callback costs can differ even
// when V8 is the selected engine. This opt-in import changes no workload.
import _ "github.com/buke/quickjs-go"
