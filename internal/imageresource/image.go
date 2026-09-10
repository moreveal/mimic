// Package imageresource decodes resource bytes and intrinsic metadata. It never
// renders a DOM, SVG scene, or GPU surface.
package imageresource

import (
	"bytes"
	"encoding/xml"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Image struct {
	Width, Height int
	Pixels        []byte
	Vector        bool
}

func Decode(data []byte, contentType string) (*Image, error) {
	if len(data) > 32<<20 {
		return nil, fmt.Errorf("image byte limit")
	}
	if strings.Contains(strings.ToLower(contentType), "svg") || bytes.Contains(data[:min(len(data), 512)], []byte("<svg")) {
		decoder := xml.NewDecoder(bytes.NewReader(data))
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			if start.Name.Local != "svg" || start.Name.Space != "http://www.w3.org/2000/svg" {
				return nil, fmt.Errorf("invalid SVG image root")
			}
			attrs := map[string]string{}
			for _, a := range start.Attr {
				attrs[a.Name.Local] = a.Value
			}
			width, hasWidth := intrinsicLength(attrs["width"])
			height, hasHeight := intrinsicLength(attrs["height"])
			ratio := 0.0
			parts := strings.Fields(strings.ReplaceAll(attrs["viewBox"], ",", " "))
			if len(parts) == 4 {
				vw, e1 := strconv.ParseFloat(parts[2], 64)
				vh, e2 := strconv.ParseFloat(parts[3], 64)
				if e1 == nil && e2 == nil && vw > 0 && vh > 0 && !math.IsInf(vw/vh, 0) {
					ratio = vw / vh
				}
			}
			if !hasWidth {
				width = 300
			}
			if !hasHeight {
				height = 150
			}
			if ratio > 0 {
				if hasWidth && !hasHeight {
					height = width / ratio
				} else if hasHeight && !hasWidth {
					width = height * ratio
				} else if !hasWidth && !hasHeight {
					if ratio > 2 {
						height = width / ratio
					} else {
						width = height * ratio
					}
				}
			}
			if width > 16384 || height > 16384 {
				return nil, fmt.Errorf("image dimension limit")
			}
			w, h := int(math.Round(width)), int(math.Round(height))
			if w > 16384 || h > 16384 {
				return nil, fmt.Errorf("image dimension limit")
			}
			for {
				_, err = decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, err
				}
			}
			return &Image{Width: w, Height: h, Vector: true}, nil
		}
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16777216 {
		return nil, fmt.Errorf("image dimension limit")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	pixels := image.NewRGBA(image.Rect(0, 0, config.Width, config.Height))
	draw.Draw(pixels, pixels.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	return &Image{Width: config.Width, Height: config.Height, Pixels: pixels.Pix}, nil
}

var intrinsicNumber = regexp.MustCompile(`^([+-]?(?:[0-9]*\.[0-9]+|[0-9]+)(?:[eE][+-]?[0-9]+)?)(px|in|cm|mm|q|pt|pc)?$`)

func intrinsicLength(raw string) (float64, bool) {
	match := intrinsicNumber.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	scale := map[string]float64{"": 1, "px": 1, "in": 96, "cm": 96 / 2.54, "mm": 96 / 25.4, "q": 96 / 101.6, "pt": 96.0 / 72, "pc": 16}[match[2]]
	return math.Max(0, value*scale), true
}
