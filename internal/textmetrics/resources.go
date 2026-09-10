package textmetrics

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/harfbuzz"
	fontcontainer "github.com/tdewolff/font"
)

// FontReference is an active face in the owning Document/Worker font set.
// Opaque resources alone do not make a family available to another realm.
type FontReference struct {
	Unsupported  string  `json:"unsupported"`
	UnicodeRange string  `json:"unicodeRange"`
	ID           string  `json:"id"`
	Family       string  `json:"family"`
	Weight       float64 `json:"weight"`
	Style        string  `json:"style"`
}

func (e *Engine) LocalFont(name string) (string, error) {
	e.scan()
	r, ok := e.localNames[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", nil
	}
	if _, err := e.load(r); err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s#%d", r.path, r.index)
	id := fmt.Sprintf("local-%x", sha256.Sum256([]byte(key)))
	e.resources[id] = r
	return id, nil
}

func (e *Engine) RegisterFont(data []byte) (string, error) {
	if len(data) < 12 || len(data) > 32<<20 {
		return "", fmt.Errorf("invalid font resource size")
	}
	id := fmt.Sprintf("data-%x", sha256.Sum256(data))
	if _, ok := e.resources[id]; ok {
		return id, nil
	}
	if len(e.faces) >= 64 {
		return "", fmt.Errorf("font face limit")
	}
	if string(data[:4]) == "wOF2" || string(data[:4]) == "wOFF" {
		if len(data) < 48 || binary.BigEndian.Uint32(data[16:20]) > 32<<20 {
			return "", fmt.Errorf("font expansion limit")
		}
		if err := preflightWebFont(data); err != nil {
			return "", err
		}
		var err error
		data, err = fontcontainer.ToSFNT(data)
		if err != nil {
			return "", err
		}
	}
	if len(data) > 32<<20 || len(data)+e.bytes > 64<<20 {
		return "", fmt.Errorf("font byte limit")
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || len(loaders) == 0 {
		return "", fmt.Errorf("invalid font container")
	}
	loader := loaders[0]
	ft, err := font.NewFont(loader)
	if err != nil {
		return "", err
	}
	face := font.NewFace(ft)
	metrics, ok := face.FontHExtents()
	if !ok {
		return "", fmt.Errorf("missing horizontal font metrics")
	}
	value := &loaded{face: face, shaper: harfbuzz.NewFont(face), ascent: float64(metrics.Ascender), descent: -float64(metrics.Descender)}
	if os2, err := loader.RawTable(ot.MustNewTag("OS/2")); err == nil && len(os2) >= 78 {
		value.ascent = float64(binary.BigEndian.Uint16(os2[74:76]))
		value.descent = float64(binary.BigEndian.Uint16(os2[76:78]))
	}
	description, _ := font.Describe(loader, nil)
	description.Aspect.SetDefaults()
	r := resource{path: "memory:" + id, index: 0, family: description.Family, aspect: description.Aspect}
	e.resources[id] = r
	e.faces[r.path+"#0"] = value
	e.bytes += len(data)
	return id, nil
}

func (f FontReference) covers(cluster []rune) bool {
	if f.UnicodeRange == "" {
		return true
	}
	for _, ch := range cluster {
		if harfbuzz.IsDefaultIgnorable(ch) {
			continue
		}
		covered := false
		for _, raw := range strings.Split(f.UnicodeRange, ",") {
			bounds := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "U+"), "-")
			start, err := strconv.ParseInt(bounds[0], 16, 32)
			if err != nil {
				continue
			}
			end := start
			if len(bounds) == 2 {
				end, err = strconv.ParseInt(bounds[1], 16, 32)
				if err != nil {
					continue
				}
			}
			if int64(ch) >= start && int64(ch) <= end {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}
