package browser

import (
	"context"
	"fmt"
	"math"

	"github.com/moreveal/mimic/internal/scheduler"
)

// GeolocationOverride is Page-owned emulated provider state, not a permission.
// Missing latitude, longitude, or accuracy emulates a provider error.
type GeolocationOverride struct {
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	Accuracy         *float64 `json:"accuracy"`
	Altitude         *float64 `json:"altitude"`
	AltitudeAccuracy *float64 `json:"altitudeAccuracy"`
	Heading          *float64 `json:"heading"`
	Speed            *float64 `json:"speed"`
}

func (p *Page) SetGeolocationOverride(location *GeolocationOverride) error {
	if location != nil {
		copy := *location
		for _, field := range []struct {
			value    **float64
			min, max float64
		}{
			{&copy.Latitude, -90, 90}, {&copy.Longitude, -180, 180}, {&copy.Accuracy, 0, math.Inf(1)},
			{&copy.Altitude, math.Inf(-1), math.Inf(1)}, {&copy.AltitudeAccuracy, 0, math.Inf(1)},
			{&copy.Heading, 0, 360}, {&copy.Speed, 0, math.Inf(1)},
		} {
			if *field.value == nil {
				continue
			}
			value := **field.value
			if math.IsNaN(value) || math.IsInf(value, 0) || value < field.min || value > field.max {
				return fmt.Errorf("Invalid geolocation")
			}
			*field.value = &value
		}
		location = &copy
	}
	p.mu.Lock()
	p.geolocationOverride = location
	p.mu.Unlock()
	// The existing realm registry supplies lifetime ownership; never invoke JS
	// from the protocol goroutine or create a second event loop for the provider.
	p.ctx.mu.Lock()
	defer p.ctx.mu.Unlock()
	for realm := range p.ctx.permissionRealms {
		if realm.agent.Page() != p || realm.geolocationNotifier == nil || realm.resourceContext.Err() != nil {
			continue
		}
		realm.scheduler.Post(scheduler.Control, 0, func(ctx context.Context) error {
			_, err := realm.runtime.Call(ctx, realm.geolocationNotifier, nil)
			return err
		})
	}
	return nil
}
