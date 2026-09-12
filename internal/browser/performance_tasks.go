package browser

import "time"

// Long tasks include microtasks. Transport waiting and future timer delays are
// not execution. This projects the scheduler's existing turn boundaries.
func (r *Realm) recordPerformanceTask(start, end time.Time) {
	r.performance.finishInputEvents()
	if end.Sub(start) < 50*time.Millisecond {
		return
	}
	p := r.agent.Page()
	duration := float64(end.Sub(start) / time.Millisecond)
	frame, ok := r.agent.(*Frame)
	if !ok {
		return
	}
	containerType, src, id, name := "window", "", "", ""
	if frame.parent != nil && frame.parent.Realm != nil {
		containerType = "iframe"
		d := frame.parent.Realm.document
		src, _ = d.GetAttribute(frame.elementID, "src")
		id, _ = d.GetAttribute(frame.elementID, "id")
		name, _ = d.GetAttribute(frame.elementID, "name")
	}
	for owner := frame; owner != nil; owner = owner.parent {
		realm := owner.Realm
		if owner == frame {
			realm = r
		}
		// Cross-origin attribution requires the nearest security boundary, not
		// the executing frame's attributes. Keep that unimplemented boundary
		// explicit until the corresponding multi-origin oracle is covered.
		if realm == nil || realm.performance == nil || realm.origin != r.origin {
			continue
		}
		label := "self"
		if owner != frame {
			label = "same-origin-descendant"
		}
		stamp := p.performanceClamper.now(start, realm.performanceOrigin, realm.securityState().crossOriginIsolated)
		attribution := []map[string]any{{"name": "unknown", "entryType": "taskattribution", "startTime": 0.0, "duration": 0.0, "navigationId": realm.performanceNavigationID(), "containerType": containerType, "containerSrc": src, "containerId": id, "containerName": name}}
		realm.performance.append(realm.performance.create(map[string]any{"name": label, "entryType": "longtask", "startTime": stamp, "duration": duration, "attribution": attribution}, nil))
	}
}
