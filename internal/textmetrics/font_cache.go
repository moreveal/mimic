package textmetrics

import "fmt"

// Local font faces are reloadable cache entries, not an ever-growing quota.
// Keep author-registered memory resources alive: their handles remain valid.
// Eviction never changes the selected family or substitutes fabricated metrics.
func (e *Engine) makeFontRoom(bytes int) error {
	for len(e.faces) >= 64 || e.bytes+bytes > 64<<20 {
		victim := ""
		var oldest uint64 = ^uint64(0)
		for key, face := range e.faces {
			if victim == "" || face.lastUse < oldest {
				victim, oldest = key, face.lastUse
			}
		}
		if victim == "" {
			return fmt.Errorf("font cache budget exhausted")
		}
		e.bytes -= e.faces[victim].resourceBytes
		delete(e.faces, victim)
		// Shaping scratch may retain plans for the evicted face.
		e.shapeScratch = nil
		e.shapePlans = nil
	}
	return nil
}
