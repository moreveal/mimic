package textmetrics

import (
	"fmt"
	"os"
)

// FontResourceBytes exports the already admitted resource, not its original URL.
// The caller owns the returned bytes. This does not activate any font family.
func (e *Engine) FontResourceBytes(id string) ([]byte, int, error) {
	r, ok := e.resources[id]
	if !ok {
		return nil, 0, fmt.Errorf("unknown font resource %q", id)
	}
	info, err := os.Stat(r.path)
	if err != nil {
		return nil, 0, err
	}
	if info.Size() > 64<<20 {
		return nil, 0, fmt.Errorf("font resource exceeds native collection bound")
	}
	data, err := os.ReadFile(r.path)
	return data, r.index, err
}
