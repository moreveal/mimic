package state

func (e Environment) SystemFontPalette() map[string]string {
	result := map[string]string{"caption": "16px Arial", "icon": "16px Arial", "message-box": "16px Arial", "menu": "12px Ubuntu", "small-caption": "12px Ubuntu", "status-bar": "12px Ubuntu"}
	for name, value := range e.Fonts.System {
		result[name] = value
	}
	return result
}

func (e Environment) SystemColorPalette() map[string]map[string]string {
	result := DefaultSystemColors()
	for scheme, colors := range e.Preferences.SystemColors {
		if result[scheme] == nil {
			result[scheme] = map[string]string{}
		}
		for name, value := range colors {
			result[scheme][name] = value
		}
	}
	return result
}

// DefaultSystemColors is the measured Windows Chrome 152 palette. Profiles may
// override individual colors; callers receive fresh maps, never shared mutable state.
func DefaultSystemColors() map[string]map[string]string {
	return map[string]map[string]string{
		"light": {
			"activetext":          "rgb(0, 102, 204)",
			"buttonborder":        "rgb(0, 0, 0)",
			"buttonface":          "rgb(240, 240, 240)",
			"buttontext":          "rgb(0, 0, 0)",
			"canvas":              "rgb(255, 255, 255)",
			"canvastext":          "rgb(0, 0, 0)",
			"field":               "rgb(255, 255, 255)",
			"fieldtext":           "rgb(0, 0, 0)",
			"graytext":            "rgb(109, 109, 109)",
			"highlight":           "rgb(0, 120, 212)",
			"highlighttext":       "rgb(255, 255, 255)",
			"linktext":            "rgb(0, 102, 204)",
			"mark":                "rgb(255, 255, 0)",
			"marktext":            "rgb(0, 0, 0)",
			"selecteditem":        "rgb(25, 103, 210)",
			"selecteditemtext":    "rgb(255, 255, 255)",
			"visitedtext":         "rgb(0, 102, 204)",
			"accentcolor":         "rgb(0, 117, 255)",
			"accentcolortext":     "rgb(255, 255, 255)",
			"activeborder":        "rgb(0, 0, 0)",
			"activecaption":       "rgb(255, 255, 255)",
			"appworkspace":        "rgb(255, 255, 255)",
			"background":          "rgb(255, 255, 255)",
			"buttonhighlight":     "rgb(240, 240, 240)",
			"buttonshadow":        "rgb(240, 240, 240)",
			"captiontext":         "rgb(0, 0, 0)",
			"inactiveborder":      "rgb(0, 0, 0)",
			"inactivecaption":     "rgb(255, 255, 255)",
			"inactivecaptiontext": "rgb(128, 128, 128)",
			"infobackground":      "rgb(255, 255, 255)",
			"infotext":            "rgb(0, 0, 0)",
			"menu":                "rgb(255, 255, 255)",
			"menutext":            "rgb(0, 0, 0)",
			"scrollbar":           "rgb(255, 255, 255)",
			"threeddarkshadow":    "rgb(0, 0, 0)",
			"threedface":          "rgb(240, 240, 240)",
			"threedhighlight":     "rgb(0, 0, 0)",
			"threedlightshadow":   "rgb(0, 0, 0)",
			"threedshadow":        "rgb(0, 0, 0)",
			"window":              "rgb(255, 255, 255)",
			"windowframe":         "rgb(0, 0, 0)",
			"windowtext":          "rgb(0, 0, 0)",
		},
		"dark": {
			"activetext":          "rgb(255, 0, 0)",
			"buttonborder":        "rgb(255, 255, 255)",
			"buttonface":          "rgb(107, 107, 107)",
			"buttontext":          "rgb(255, 255, 255)",
			"canvas":              "rgb(18, 18, 18)",
			"canvastext":          "rgb(255, 255, 255)",
			"field":               "rgb(59, 59, 59)",
			"fieldtext":           "rgb(255, 255, 255)",
			"graytext":            "rgb(128, 128, 128)",
			"highlight":           "rgb(25, 103, 210)",
			"highlighttext":       "rgb(255, 255, 255)",
			"linktext":            "rgb(158, 158, 255)",
			"mark":                "rgb(255, 255, 0)",
			"marktext":            "rgb(0, 0, 0)",
			"selecteditem":        "rgb(153, 200, 255)",
			"selecteditemtext":    "rgb(59, 59, 59)",
			"visitedtext":         "rgb(208, 173, 240)",
			"accentcolor":         "rgb(0, 117, 255)",
			"accentcolortext":     "rgb(255, 255, 255)",
			"activeborder":        "rgb(255, 255, 255)",
			"activecaption":       "rgb(18, 18, 18)",
			"appworkspace":        "rgb(18, 18, 18)",
			"background":          "rgb(18, 18, 18)",
			"buttonhighlight":     "rgb(107, 107, 107)",
			"buttonshadow":        "rgb(107, 107, 107)",
			"captiontext":         "rgb(255, 255, 255)",
			"inactiveborder":      "rgb(255, 255, 255)",
			"inactivecaption":     "rgb(18, 18, 18)",
			"inactivecaptiontext": "rgb(128, 128, 128)",
			"infobackground":      "rgb(18, 18, 18)",
			"infotext":            "rgb(255, 255, 255)",
			"menu":                "rgb(18, 18, 18)",
			"menutext":            "rgb(255, 255, 255)",
			"scrollbar":           "rgb(18, 18, 18)",
			"threeddarkshadow":    "rgb(255, 255, 255)",
			"threedface":          "rgb(107, 107, 107)",
			"threedhighlight":     "rgb(255, 255, 255)",
			"threedlightshadow":   "rgb(255, 255, 255)",
			"threedshadow":        "rgb(255, 255, 255)",
			"window":              "rgb(18, 18, 18)",
			"windowframe":         "rgb(255, 255, 255)",
			"windowtext":          "rgb(255, 255, 255)",
		},
	}
}
