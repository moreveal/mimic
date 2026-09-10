package textmetrics

import (
	"fmt"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/harfbuzz"
	"strings"
)

func coversCluster(face *loaded, cluster []rune) bool {
	for i, ch := range cluster {
		if i > 0 && (ch == 0xfe0e || ch == 0xfe0f) {
			gid, ok := face.face.VariationGlyph(cluster[i-1], ch)
			if !ok {
				gid, ok = face.face.NominalGlyph(cluster[i-1])
			}
			if !ok {
				return false
			}
			_, color := face.face.GlyphDataColor(tables.GlyphID(gid))
			if (ch == 0xfe0f) != color {
				return false
			}
			continue
		}
		// Joiners and selectors participate in shaping but need no visible glyph.
		if harfbuzz.IsDefaultIgnorable(ch) {
			continue
		}
		if _, ok := face.face.NominalGlyph(ch); !ok {
			return false
		}
	}
	return true
}

func (e *Engine) fallbackFace(primary *loaded, cluster []rune, families string, weight float64, italic bool) (*loaded, error) {
	if coversCluster(primary, cluster) {
		return primary, nil
	}
	// Try the author's remaining families before Windows fallback families.
	// This is a profile-level preference, not a codepoint-specific substitution.
	names := strings.Split(families, ",")
	for _, ch := range cluster {
		if ch == 0xfe0e {
			names = append(names, "serif")
			break
		}
	}
	names = append(names, "Segoe UI Emoji", "Segoe UI Symbol", "Segoe UI", "Arial")
	seen := map[string]bool{}
	try := func(r resource) (*loaded, error) {
		key := fmt.Sprintf("%s#%d", r.path, r.index)
		if seen[key] {
			return nil, nil
		}
		seen[key] = true
		face, err := e.load(r)
		if err != nil {
			return nil, err
		}
		if coversCluster(face, cluster) {
			return face, nil
		}
		return nil, nil
	}
	for _, name := range names {
		r, err := e.selectResource(name, weight, italic)
		if err != nil {
			continue
		}
		face, err := try(r)
		if err != nil {
			return nil, fmt.Errorf("fallback resource %s: %w", r.family, err)
		}
		if face != nil {
			return face, nil
		}
	}
	// Coverage search remains bounded by the Page's face/byte limits. No
	// process-wide cache, synthetic .notdef widths or per-character random data.
	for _, r := range e.catalog {
		if (r.aspect.Style == font.StyleItalic) != italic {
			continue
		}
		face, err := try(r)
		if err != nil {
			return nil, fmt.Errorf("fallback resource %s: %w", r.family, err)
		}
		if face != nil {
			return face, nil
		}
	}
	return nil, fmt.Errorf("no font covers grapheme beginning U+%04X (%d runes)", cluster[0], len(cluster))
}
