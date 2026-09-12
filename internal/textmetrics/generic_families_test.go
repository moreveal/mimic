package textmetrics

import (
	"reflect"
	"testing"
)

func TestConfiguredGenericFamilyUsesNamedResource(t *testing.T) {
	engine := New()
	named, err := engine.Shape("Alpha", "Ubuntu", 14, 400, false, false, false)
	if err != nil || named.Family != "ubuntu" {
		t.Skip("selected frozen Ubuntu resource unavailable on this host")
	}
	engine.SetGenericFamily("system-ui", "Ubuntu")
	actual, err := engine.Shape("Alpha", "system-ui", 14, 400, false, false, false)
	if err != nil || !reflect.DeepEqual(actual, named) {
		t.Fatalf("generic resource selection: %#v, %v", actual, err)
	}
	other := NewDirectories(nil)
	other.SetGenericFamily("system-ui", "missing selected resource")
	if _, err := other.Shape("Alpha", "system-ui,Ubuntu", 14, 400, false, false, false); err == nil {
		t.Fatal("missing selected generic resource must report an unsupported boundary")
	}
}
