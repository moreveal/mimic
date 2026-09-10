package quic

import (
	"testing"
	"time"
)

func TestClientRTTCacheIsolationAndEviction(t *testing.T) {
	a, b := NewClientRTTCache(2), NewClientRTTCache(2)
	a.put("one:443", time.Millisecond)
	a.put("two:443", 2*time.Millisecond)
	if b.get("one:443") != 0 || a.get("one:8443") != 0 {
		t.Fatal("RTT leaked across clients or origins")
	}
	if a.get("one:443") != time.Millisecond {
		t.Fatal("measured RTT lost")
	}
	a.put("three:443", 3*time.Millisecond)
	if a.get("two:443") != 0 || a.get("one:443") != time.Millisecond {
		t.Fatal("cache did not evict the least recently used origin")
	}
	a.put("one:443", 0)
	if a.get("one:443") != time.Millisecond {
		t.Fatal("unmeasured RTT replaced a measurement")
	}
}
