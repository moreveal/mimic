//go:build windows && amd64

package v8

import "testing"

// Bootstrap instrumentation changes executable source. Snapshot admission
// must agree with the adapter's instrumentation policy before source capture.
func TestBootstrapSnapshotDiagnosticsPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, diagnostics, hosts string
		eligible                 bool
	}{
		{"ordinary", "", "", true},
		{"instrumented", "1", "", false},
		{"host-only", "", "1", true},
		{"host-only-overrides-diagnostics", "1", "1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MIMIC_DIAGNOSTICS", tc.diagnostics)
			t.Setenv("MIMIC_PROFILE_HOSTS", tc.hosts)
			eligible := (Factory{}).BootstrapSnapshotsEnabled()
			if eligible != tc.eligible {
				t.Fatalf("snapshot eligibility = %v, want %v", eligible, tc.eligible)
			}
			runtime := &adapter{profile: newDiagnostics()}
			if eligible == runtime.ProfileEnabled() {
				t.Fatal("snapshot eligibility disagrees with bootstrap instrumentation")
			}
		})
	}
}
