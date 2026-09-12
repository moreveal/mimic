package textmetrics

import "github.com/go-text/typesetting/harfbuzz"

const shapeScratchRunes = 1024
const shapeScratchPlans = 16

type shapePlanKey struct {
	face                *loaded
	properties          harfbuzz.SegmentProperties
	noKern, noLigatures bool
}

// HarfBuzz stores reusable shaping plans in Buffer, not Font. Recreating a
// Buffer for every observation discards those plans and all glyph workspace.
// One Page-owned buffer is retained for short runs, with at most 16 distinct
// immutable font/property/feature combinations. Font variations are fixed on
// loaded resources. Large runs use temporary storage; input length also bounds
// HarfBuzz's internal expansion limit (64*length, minimum 16384 glyphs).
func (e *Engine) shapingBuffer(runes []rune, start, end int, face *loaded, noKern, noLigatures bool) *harfbuzz.Buffer {
	reusable := end-start <= shapeScratchRunes
	buffer := e.shapeScratch
	if !reusable || buffer == nil {
		buffer = harfbuzz.NewBuffer()
	} else {
		buffer.Clear()
	}
	buffer.AddRunes(runes, start, end-start)
	buffer.GuessSegmentProperties()
	if !reusable {
		return buffer
	}
	key := shapePlanKey{face, buffer.Props, noKern, noLigatures}
	if _, exists := e.shapePlans[key]; !exists {
		if len(e.shapePlans) == shapeScratchPlans {
			e.shapePlans = nil
			buffer = harfbuzz.NewBuffer()
			buffer.AddRunes(runes, start, end-start)
			buffer.GuessSegmentProperties()
		}
		if e.shapePlans == nil {
			e.shapePlans = make(map[shapePlanKey]struct{})
		}
		e.shapePlans[key] = struct{}{}
	}
	e.shapeScratch = buffer
	return buffer
}
