// Package chrome is the registry for immutable, versioned Chrome
// compatibility bundles.
package chrome

import (
	"fmt"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/compatibility"
)

func Get(milestone int) (compatibility.Bundle, error) {
	switch milestone {
	case chrome152.Milestone:
		return chrome152.New(), nil
	default:
		return nil, fmt.Errorf("Chrome milestone %d is not installed", milestone)
	}
}
