// Package chrome is the registry for immutable, versioned Chrome
// compatibility bundles.
package chrome

import (
	"fmt"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/state"
)

func Get(milestone int) (compatibility.Bundle, error) {
	return GetForMode(milestone, state.BrowserModeHeadful)
}

// GetForMode selects an explicit, coherent presentation environment. The
// ordinary registry path always selects the authoritative headful profile.
func GetForMode(milestone int, mode state.BrowserMode) (compatibility.Bundle, error) {
	switch milestone {
	case chrome152.Milestone:
		return chrome152.NewForMode(mode)
	default:
		return nil, fmt.Errorf("Chrome milestone %d is not installed", milestone)
	}
}
