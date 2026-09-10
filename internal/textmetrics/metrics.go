// Package textmetrics reads local OpenType resources and shapes text without
// rasterizing glyphs. An Engine belongs to one Page and is not shared between
// event loops. No native font, windowing or graphics backend is used.
package textmetrics

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/segmenter"
)

type resource struct {
	path   string
	index  int
	family string
	aspect font.Aspect
}

type loaded struct {
	face            *font.Face
	shaper          *harfbuzz.Font
	ascent, descent float64
}

type Engine struct {
	dirs    []string
	catalog []resource
	scanned bool
	faces   map[string]*loaded
	bytes   int
}

func New() *Engine {
	var dirs []string
	if configured := os.Getenv("MIMIC_FONT_DIR"); configured != "" {
		dirs = filepath.SplitList(configured)
	} else if runtime.GOOS == "windows" {
		dirs = append(dirs, filepath.Join(os.Getenv("WINDIR"), "Fonts"))
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
	}
	return NewDirectories(dirs)
}

// NewDirectories permits explicit font resources on a non-Windows host. The
// installed Windows reference fonts themselves are not distributed by Mimic.
func NewDirectories(dirs []string) *Engine {
	return &Engine{dirs: append([]string(nil), dirs...), faces: map[string]*loaded{}}
}

func (e *Engine) scan() {
	if e.scanned {
		return
	}
	e.scanned = true
	var scratch []byte
	for _, dir := range e.dirs {
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if len(e.catalog) >= 4096 {
				return
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if entry.IsDir() || (ext != ".ttf" && ext != ".otf" && ext != ".ttc") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			loaders, err := ot.NewLoaders(f)
			if err == nil {
				for index, loader := range loaders {
					var d font.Description
					d, scratch = font.Describe(loader, scratch)
					d.Aspect.SetDefaults()
					if d.Family != "" {
						e.catalog = append(e.catalog, resource{path, index, strings.ToLower(d.Family), d.Aspect})
					}
				}
			}
			f.Close()
		}
	}
}

func (e *Engine) selectResource(families string, weight float64, italic bool) (resource, error) {
	e.scan()
	names := strings.Split(families, ",")
	names = append(names, "serif")
	for _, name := range names {
		name = strings.TrimSpace(name)
		quoted := strings.HasPrefix(name, "\"") || strings.HasPrefix(name, "'")
		name = strings.ToLower(strings.Trim(name, "\"'"))
		if !quoted {
			switch name {
			case "serif":
				name = "times new roman"
			case "sans-serif":
				name = "arial"
			case "monospace":
				name = "consolas"
			case "system-ui":
				name = "segoe ui"
			}
		}
		best := -1
		score := math.Inf(1)
		for i, r := range e.catalog {
			if r.family != name {
				continue
			}
			s := math.Abs(float64(r.aspect.Weight)-weight) + 1000*math.Abs(float64(r.aspect.Stretch)-1)
			if (r.aspect.Style == font.StyleItalic) != italic {
				s += 10000
			}
			if s < score {
				score = s
				best = i
			}
		}
		if best >= 0 {
			r := e.catalog[best]
			if (r.aspect.Style == font.StyleItalic) != italic {
				return resource{}, fmt.Errorf("synthetic font style is unsupported")
			}
			return r, nil
		}
	}
	return resource{}, fmt.Errorf("no usable reference font resource")
}

func (e *Engine) load(r resource) (*loaded, error) {
	key := fmt.Sprintf("%s#%d", r.path, r.index)
	if f := e.faces[key]; f != nil {
		return f, nil
	}
	if len(e.faces) >= 64 {
		return nil, fmt.Errorf("font face limit")
	}
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > 32<<20 || int64(e.bytes)+stat.Size() > 64<<20 {
		return nil, fmt.Errorf("font byte limit")
	}
	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || r.index >= len(loaders) {
		return nil, fmt.Errorf("invalid font container")
	}
	loader := loaders[r.index]
	ft, err := font.NewFont(loader)
	if err != nil {
		return nil, err
	}
	face := font.NewFace(ft)
	metrics, ok := face.FontHExtents()
	if !ok {
		return nil, fmt.Errorf("missing horizontal font metrics")
	}
	value := &loaded{face: face, shaper: harfbuzz.NewFont(face), ascent: float64(metrics.Ascender), descent: -float64(metrics.Descender)}
	// The Windows Chrome profile uses Windows ascender/descender rather than
	// the optional typographic line metrics (notably different in Consolas).
	if os2, err := loader.RawTable(ot.MustNewTag("OS/2")); err == nil && len(os2) >= 78 {
		value.ascent = float64(binary.BigEndian.Uint16(os2[74:76]))
		value.descent = float64(binary.BigEndian.Uint16(os2[76:78]))
	}
	e.faces[key] = value
	e.bytes += len(data)
	return value, nil
}

type Glyph struct {
	Cluster int     `json:"cluster"`
	Advance float64 `json:"advance"`
	XOffset float64 `json:"xOffset"`
	YOffset float64 `json:"yOffset"`
	Left    float64 `json:"left"`
	Right   float64 `json:"right"`
	Ink     bool    `json:"ink"`
}

type Result struct {
	Glyphs  []Glyph `json:"glyphs"`
	Ascent  float64 `json:"ascent"`
	Descent float64 `json:"descent"`
	XHeight float64 `json:"xHeight"`
	Family  string  `json:"family"`
}

func (e *Engine) Shape(text, families string, size, weight float64, italic, noKern, noLigatures bool) (Result, error) {
	if !finite(size) || size <= 0 || size > 4096 || !finite(weight) {
		return Result{}, fmt.Errorf("unsupported font size or weight")
	}
	runes := []rune(text)
	if len(runes) > 16384 {
		return Result{}, fmt.Errorf("text complexity limit")
	}
	r, err := e.selectResource(families, weight, italic)
	if err != nil {
		return Result{}, err
	}
	f, err := e.load(r)
	if err != nil {
		return Result{}, err
	}
	// Keep grapheme clusters intact (combining marks, selectors, emoji ZWJ).
	type run struct {
		start, end int
		face       *loaded
	}
	runs := []run{}
	var segments segmenter.Segmenter
	segments.Init(runes)
	iterator := segments.GraphemeIterator()
	fallbackCache := map[string]*loaded{}
	for iterator.Next() {
		cluster := iterator.Grapheme()
		key := string(cluster.Text)
		selected := fallbackCache[key]
		if selected == nil {
			selected, err = e.fallbackFace(f, cluster.Text, families, weight, italic)
			if err == nil {
				fallbackCache[key] = selected
			}
		}
		if err != nil {
			return Result{}, err
		}
		end := cluster.Offset + len(cluster.Text)
		if len(runs) > 0 && runs[len(runs)-1].face == selected {
			runs[len(runs)-1].end = end
		} else {
			runs = append(runs, run{cluster.Offset, end, selected})
		}
	}
	var features []harfbuzz.Feature
	for name, disabled := range map[string]bool{"kern": noKern, "liga": noLigatures, "clig": noLigatures} {
		if disabled {
			feature, _ := harfbuzz.ParseFeature(name + "=0")
			features = append(features, feature)
		}
	}
	primaryScale := size / float64(f.face.Upem())
	result := Result{Glyphs: []Glyph{}, Ascent: math.Round(f.ascent * primaryScale), Descent: math.Round(f.descent * primaryScale), XHeight: float64(f.face.LineMetric(font.XHeight)) * primaryScale, Family: r.family}
	for _, run := range runs {
		buffer := harfbuzz.NewBuffer()
		buffer.AddRunes(runes, run.start, run.end-run.start)
		buffer.GuessSegmentProperties()
		if buffer.Props.Direction != harfbuzz.LeftToRight {
			return Result{}, fmt.Errorf("non-LTR shaping is unsupported")
		}
		buffer.Shape(run.face.shaper, features)
		scale := size / float64(run.face.face.Upem())
		for i, info := range buffer.Info {
			p := buffer.Pos[i]
			g := Glyph{Cluster: info.Cluster, Advance: float64(p.XAdvance) * scale, XOffset: float64(p.XOffset) * scale, YOffset: -float64(p.YOffset) * scale}
			if bounds, ok := run.face.face.GlyphExtents(info.Glyph); ok && bounds.Width != 0 && bounds.Height != 0 {
				g.Ink = true
				g.Left = math.Floor(float64(bounds.XBearing) * scale)
				g.Right = math.Ceil(float64(bounds.XBearing+bounds.Width) * scale)
			}
			result.Glyphs = append(result.Glyphs, g)
		}
	}
	return result, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
