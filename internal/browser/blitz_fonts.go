package browser

import (
	"fmt"
	"github.com/moreveal/mimic/internal/layoutblitz"
)

// blitzFontInputs executes only on the Page owner. Resources are admitted and
// normalized by the existing font loader; this adapter never fetches a URL.
// The reason result requests explicit whole-document fallback where Parley's
// font selection cannot yet represent canonical FontFace descriptors.
func (r *Realm) blitzFontInputs() ([]layoutblitz.Font, string, error) {
	var fonts []layoutblitz.Font
	var total int
	for _, choice := range r.fontChoices {
		if choice.Unsupported != "" || (choice.UnicodeRange != "" && choice.UnicodeRange != "U+0-10FFFF") {
			return nil, "native FontFace descriptor coverage pending", nil
		}
		if choice.Style != "" && choice.Style != "normal" && choice.Style != "italic" {
			return nil, "native oblique FontFace descriptor pending", nil
		}
		if choice.ID == "" {
			continue
		}
		data, index, err := r.agent.Page().textMetricsEngine().FontResourceBytes(choice.ID)
		if err != nil {
			return nil, "", fmt.Errorf("blitz canonical font: %w", err)
		}
		if index != 0 {
			return nil, "native indexed font collection face pending", nil
		}
		total += len(data)
		if total > 64<<20 || len(fonts) >= 4096 {
			return nil, "native font collection bound", nil
		}
		weight := float32(choice.Weight)
		if weight == 0 {
			weight = 400
		}
		fonts = append(fonts, layoutblitz.Font{Family: choice.Family, Weight: weight, Italic: choice.Style == "italic", Bytes: data})
	}
	return fonts, "", nil
}
