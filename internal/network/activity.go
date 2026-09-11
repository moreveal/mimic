package network

import "time"

// Activity covers the complete logical load, including cache/interception and
// failures before a transport is opened. Keeping it at Load's ownership boundary
// prevents missing cancellation/error events from leaking an in-flight count.
func (l *Loader) beginActivity() {
	l.activityMu.Lock()
	defer l.activityMu.Unlock()
	l.activeLoads++
	for i, threshold := range []int{0, 2} {
		if l.activeLoads > threshold {
			l.idleSince[i] = time.Time{}
		} else if l.idleSince[i].IsZero() {
			l.idleSince[i] = time.Now()
		}
	}
}
func (l *Loader) endActivity() {
	l.activityMu.Lock()
	defer l.activityMu.Unlock()
	l.activeLoads--
	for i, threshold := range []int{0, 2} {
		if l.activeLoads <= threshold && l.idleSince[i].IsZero() {
			l.idleSince[i] = time.Now()
		}
	}
}
func (l *Loader) Activity() (active int, idleSince [2]time.Time) {
	l.activityMu.Lock()
	defer l.activityMu.Unlock()
	return l.activeLoads, l.idleSince
}
