package browser

import (
	"encoding/binary"
	"hash/maphash"
)

const textCacheProfileKeys = 8192

// This optional diagnostic retains fingerprints and counters, never author text
// or glyph results. Distinct estimates are hash-based; when the table fills,
// tracked working-set bytes are explicitly a lower bound. Font generations are
// part of the fingerprint because those observations cannot share cache entries.
type textCacheProfile struct {
	seed              maphash.Seed
	keys              map[uint64]textCacheKeySample
	Full              textCacheModeProfile `json:"full"`
	Compact           textCacheModeProfile `json:"compact"`
	FontInvalidations uint64               `json:"fontInvalidations"`
	PeakEntries       int                  `json:"peakEntries"`
	PeakBytes         int                  `json:"peakBytes"`
}

type textCacheModeProfile struct {
	Calls                  uint64 `json:"calls"`
	Hits                   uint64 `json:"hits"`
	Misses                 uint64 `json:"misses"`
	SuccessfulMisses       uint64 `json:"successfulMisses"`
	FailedMisses           uint64 `json:"failedMisses"`
	OversizedMisses        uint64 `json:"oversizedMisses"`
	EvictedEntries         uint64 `json:"evictedEntries"`
	DistinctTrackedKeys    int    `json:"distinctTrackedKeys"`
	DistinctSuccessfulKeys int    `json:"distinctSuccessfulKeys"`
	TrackedWorkingSetBytes int    `json:"trackedWorkingSetBytes"`
	UntrackedObservations  uint64 `json:"untrackedObservations"`
}

type textCacheKeySample struct{ successful bool }

func newTextCacheProfile() *textCacheProfile {
	return &textCacheProfile{seed: maphash.MakeSeed(), keys: make(map[uint64]textCacheKeySample)}
}

func (p *textCacheProfile) fingerprint(key textShapeKey) uint64 {
	var hash maphash.Hash
	hash.SetSeed(p.seed)
	var bits [41]byte
	binary.LittleEndian.PutUint64(bits[0:8], key.size)
	binary.LittleEndian.PutUint64(bits[8:16], key.weight)
	binary.LittleEndian.PutUint64(bits[16:24], uint64(len(key.text)))
	binary.LittleEndian.PutUint64(bits[24:32], uint64(len(key.families)))
	binary.LittleEndian.PutUint64(bits[32:40], p.FontInvalidations)
	bits[40] = key.flags
	hash.Write(bits[:])
	hash.WriteString(key.text)
	hash.WriteString(key.families)
	return hash.Sum64()
}

func (p *textCacheProfile) mode(key textShapeKey) *textCacheModeProfile {
	if key.flags&(1<<4) != 0 {
		return &p.Compact
	}
	return &p.Full
}

func (p *textCacheProfile) observe(key textShapeKey, hit bool) {
	mode := p.mode(key)
	mode.Calls++
	if hit {
		mode.Hits++
	} else {
		mode.Misses++
	}
	id := p.fingerprint(key)
	if _, exists := p.keys[id]; !exists {
		if len(p.keys) < textCacheProfileKeys {
			p.keys[id] = textCacheKeySample{}
			mode.DistinctTrackedKeys++
		} else {
			mode.UntrackedObservations++
		}
	}
}

func (p *textCacheProfile) success(key textShapeKey, value string, beforeEntries int, cache *textShapeCache) {
	mode := p.mode(key)
	mode.SuccessfulMisses++
	cost := textShapeEntryBytes(key, value)
	if cost > textShapeCacheBytes {
		mode.OversizedMisses++
	} else {
		mode.EvictedEntries += uint64(beforeEntries + 1 - len(cache.values))
	}
	id := p.fingerprint(key)
	if sample, ok := p.keys[id]; ok && !sample.successful {
		p.keys[id] = textCacheKeySample{successful: true}
		mode.DistinctSuccessfulKeys++
		mode.TrackedWorkingSetBytes += cost
	}
	p.PeakEntries = max(p.PeakEntries, len(cache.values))
	p.PeakBytes = max(p.PeakBytes, cache.bytes)
}

func (p *textCacheProfile) snapshot(cache *textShapeCache) any {
	entries, bytes := 0, 0
	if cache != nil {
		entries, bytes = len(cache.values), cache.bytes
	}
	return map[string]any{
		"counts": p, "currentEntries": entries, "currentBytes": bytes,
		"entryLimit": textShapeCacheEntries, "byteLimit": textShapeCacheBytes,
		"distinctKeyLimit": textCacheProfileKeys, "distinctTrackingSaturated": len(p.keys) == textCacheProfileKeys,
		"distinctMethod": "seeded 64-bit fingerprints; no text retained; working-set bytes across font generations; lower bound if saturated",
	}
}
