package cdp

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Diagnostic clock amplification probe: multiple debugger connections must
// never become multiple observable clocks for a single Page.
func TestMultipleDebuggerConnectionsDoNotMultiplyPageClock(t *testing.T) {
	s, addr := runningServer(t)
	var connections []*websocket.Conn
	defer func() {
		for _, c := range connections {
			c.Close()
		}
	}()
	for i := 0; i < 16; i++ {
		c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, c)
		c.WriteJSON(map[string]any{"id": 1, "method": "Target.getTargets"})
		readReply(t, c, 1)
	}
	clock := s.Page.ClockNow()
	start := time.Now()
	time.Sleep(150 * time.Millisecond)
	elapsed := time.Since(start)
	advanced := s.Page.ClockNow().Sub(clock)
	if advanced > 2*elapsed {
		t.Fatalf("multiple clocks: %v advanced in %v", advanced, elapsed)
	}
	t.Log(fmt.Sprintf("connections=16 wall_ms=%.3f page_clock_ms=%.3f amplification=%.2f", float64(elapsed)/1e6, float64(advanced)/1e6, float64(advanced)/float64(elapsed)))
}
