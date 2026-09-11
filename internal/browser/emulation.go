package browser

import (
	"fmt"
	"math"
	"strings"

	"github.com/moreveal/mimic/internal/state"
)

func (p *Page) SetDeviceMetrics(width, height int, scale float64, screenWidth, screenHeight int) error {
	if width < 0 || height < 0 || width > 10000000 || height > 10000000 || scale < 0 || math.IsInf(scale, 0) || math.IsNaN(scale) {
		return fmt.Errorf("Invalid device metrics")
	}
	base := p.ctx.browser.Environment()
	if width == 0 {
		width = base.Window.ViewportWidth
	}
	if height == 0 {
		height = base.Window.ViewportHeight
	}
	if scale == 0 {
		scale = base.Display.DeviceScaleFactor
	}
	p.viewportObservationChange(true)
	defer p.viewportObservationChange(false)
	p.mu.Lock()
	defer p.mu.Unlock()
	// Screen dimensions are CSS pixels in CDP and do not shrink when only
	// devicePixelRatio changes. Physical dimensions are derived consistently.
	screen := base.Screen()
	if screenWidth == 0 {
		screenWidth = screen.Width
	}
	if screenHeight == 0 {
		screenHeight = screen.Height
	}
	p.env.Display.DeviceScaleFactor = scale
	p.env.Display.PhysicalWidth = int(math.Round(float64(screenWidth) * scale))
	p.env.Display.PhysicalHeight = int(math.Round(float64(screenHeight) * scale))
	p.env.Display.AvailableWidth = int(math.Round(float64(screen.AvailWidth) * scale))
	p.env.Display.AvailableHeight = int(math.Round(float64(screen.AvailHeight) * scale))
	p.env.Window.ViewportWidth = width
	p.env.Window.ViewportHeight = height
	return nil
}
func (p *Page) ClearDeviceMetrics() {
	p.viewportObservationChange(true)
	defer p.viewportObservationChange(false)
	base := p.ctx.browser.Environment()
	p.mu.Lock()
	p.env.Window.ViewportWidth = base.Window.ViewportWidth
	p.env.Window.ViewportHeight = base.Window.ViewportHeight
	p.env.Display = base.Display
	p.env.ScreenOrientation = nil
	p.mu.Unlock()
}
func (p *Page) SetUserAgentOverride(o *state.UserAgentOverride) {
	p.mu.Lock()
	p.env.UserAgentOverride = o
	p.mu.Unlock()
}

func (p *Page) SetScreenOrientation(kind string, angle int) error {
	switch kind {
	case "portraitPrimary", "portraitSecondary", "landscapePrimary", "landscapeSecondary":
	default:
		return fmt.Errorf("Invalid screen orientation type")
	}
	if angle < 0 || angle > 360 {
		return fmt.Errorf("Invalid screen orientation angle")
	}
	p.viewportObservationChange(true)
	defer p.viewportObservationChange(false)
	split := "portrait"
	suffix := "primary"
	if strings.HasPrefix(kind, "landscape") {
		split = "landscape"
	}
	if strings.HasSuffix(kind, "Secondary") {
		suffix = "secondary"
	}
	p.mu.Lock()
	p.env.ScreenOrientation = &state.ScreenOrientation{Type: split + "-" + suffix, Angle: angle}
	p.mu.Unlock()
	return nil
}
func (p *Page) SetMediaPreferences(scheme string, reducedMotion *bool) {
	p.viewportObservationChange(true)
	defer p.viewportObservationChange(false)
	p.mu.Lock()
	defer p.mu.Unlock()
	if scheme != "" {
		p.env.Preferences.ColorScheme = scheme
	}
	if reducedMotion != nil {
		p.env.Preferences.ReducedMotion = *reducedMotion
	}
}
