package browser

import (
	"math"
	"slices"

	"github.com/moreveal/mimic/internal/engine"
)

var performanceEventNames = []string{"pointerdown", "touchend", "input", "keydown", "mouseleave", "mouseenter", "drop", "beforeinput", "pointerenter", "dragend", "pointercancel", "compositionupdate", "mousedown", "dragleave", "dragover", "mouseup", "pointerover", "lostpointercapture", "mouseover", "gotpointercapture", "dblclick", "keyup", "keypress", "pointerup", "compositionstart", "auxclick", "dragstart", "touchstart", "compositionend", "pointerout", "dragenter", "touchcancel", "click", "contextmenu", "mouseout", "pointerleave"}

func (r *Realm) installPerformanceEvents(host map[string]any) {
	host["performanceEventStart"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		p := r.performance
		name := strarg(a, 0)
		if !slices.Contains(performanceEventNames, name) {
			return r.val(0), nil
		}
		if p.eventCounts == nil {
			p.eventCounts = map[string]uint64{}
		}
		p.eventCounts[name]++
		id := uint64(0)
		switch name {
		case "keydown":
			if p.keyInteraction == 0 {
				p.interactionID += 7
				p.keyInteraction = p.interactionID
				p.interactionCount++
			}
			id = p.keyInteraction
		case "keyup", "keypress", "beforeinput", "input":
			id = p.keyInteraction
		case "pointerdown":
			p.interactionID += 7
			p.pointerInteraction = p.interactionID
			p.interactionCount++
			id = p.pointerInteraction
		case "pointerup", "click":
			id = p.pointerInteraction
		}
		record := p.create(map[string]any{"name": name, "entryType": "event", "startTime": numarg(a, 2), "duration": 0.0, "processingStart": p.now(), "processingEnd": 0.0, "targetNode": numarg(a, 1), "cancelable": arg(a, 3) == true, "interactionId": id}, nil)
		p.pendingEvents = append(p.pendingEvents, record)
		return r.val(record.id), nil
	})
	host["performanceEventEnd"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		p := r.performance
		record := p.records[uint64(numarg(a, 0))]
		if record == nil {
			return nil, nil
		}
		record.data["processingEnd"] = p.now()
		if record.data["name"] == "keyup" {
			p.keyInteraction = 0
		}
		if record.data["name"] == "click" || record.data["name"] == "pointercancel" {
			p.pointerInteraction = 0
		}
		return nil, nil
	})
}

func (p *performanceTimeline) finishInputEvents() {
	if len(p.pendingEvents) == 0 {
		return
	}
	end := p.now()
	for _, record := range p.pendingEvents {
		record.data["duration"] = math.Round(max(0, end-numberValue(record.data["startTime"]))/8) * 8
		// Chrome's default event buffer threshold is 104ms. An observer can
		// lower its own delivery threshold to 16ms without changing that buffer.
		if numberValue(record.data["duration"]) >= 104 {
			count := 0
			for _, e := range p.entries {
				if e.data["entryType"] == "event" {
					count++
				}
			}
			if count < 150 {
				record.buffered = true
				p.entries = append(p.entries, record)
			} else {
				p.droppedEntries["event"]++
			}
		}
		p.publish(record)
		name := record.data["name"]
		if !p.firstInput && (name == "keydown" || name == "pointerdown" || name == "mousedown" || name == "touchstart") {
			p.firstInput = true
			data := map[string]any{}
			for k, v := range record.data {
				data[k] = v
			}
			data["entryType"] = "first-input"
			p.append(p.create(data, nil))
		}
	}
	p.pendingEvents = nil
	p.prune()
}
