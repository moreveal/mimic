package browser

import (
	"encoding/binary"
	"fmt"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/woozymasta/bcn"
	"math"
)

// Texture blocks are decoded on observation, without a native graphics device.
// A single worker preserves Page ownership and avoids hidden decoding pools.
func installTextureDecoder(host map[string]any, runtime engine.Runtime) {
	host["decodeTextureBlocks"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		format, w, h := int(numarg(args, 0)), int(numarg(args, 1)), int(numarg(args, 2))
		data := byteSlice(arg(args, 3))
		pixels, err := decodeTextureBlocks(format, w, h, data)
		if err != nil {
			return nil, err
		}
		return runtime.Value(pixels), nil
	})
}
func decodeTextureBlocks(format, w, h int, data []byte) ([]float32, error) {
	if w <= 0 || h <= 0 || w > 16384 || h > 16384 || w*h > 16777216 {
		return nil, fmt.Errorf("texture decode dimensions exceed supported bounds")
	}
	opts := &bcn.DecodeOptions{Workers: 1}
	var decoded []byte
	var err error
	if format == 36494 || format == 36495 {
		rgb, err := bcn.DecodeBC6HFloat32WithOptions(data, w, h, format == 36494, opts)
		if err != nil {
			return nil, err
		}
		rgba := make([]float32, w*h*4)
		for i := 0; i < w*h; i++ {
			copy(rgba[i*4:i*4+3], rgb[i*3:i*3+3])
			rgba[i*4+3] = 1
		}
		return rgba, nil
	}
	switch format {
	case 33776, 33777, 35916, 35917:
		decoded, err = bcn.DecodeBC1WithOptions(data, w, h, opts)
	case 33778, 35918:
		decoded, err = bcn.DecodeBC2WithOptions(data, w, h, opts)
	case 33779, 35919:
		decoded, err = bcn.DecodeBC3WithOptions(data, w, h, opts)
	case 36492, 36493:
		decoded, err = bcn.DecodeBC7WithOptions(data, w, h, opts)
	case 36283, 36284, 36285, 36286:
		return decodeRGTC(format, w, h, data)
	default:
		return nil, fmt.Errorf("unsupported compressed texture format %d", format)
	}
	if err != nil {
		return nil, err
	}
	// The frozen ANGLE device rounds the BC1 three-color midpoint upward.
	// bcn truncates this midpoint; correct the palette before sRGB conversion.
	if format == 33776 || format == 33777 || format == 35916 || format == 35917 {
		bw := (w + 3) / 4
		expand := func(c uint16) [3]int {
			return [3]int{int((c>>11)<<3 | (c>>11)>>2), int(((c>>5)&63)<<2 | ((c>>5)&63)>>4), int((c&31)<<3 | (c&31)>>2)}
		}
		for by := 0; by < (h+3)/4; by++ {
			for bx := 0; bx < bw; bx++ {
				block := data[(by*bw+bx)*8:]
				a, b := binary.LittleEndian.Uint16(block), binary.LittleEndian.Uint16(block[2:])
				if a > b {
					continue
				}
				pa, pb := expand(a), expand(b)
				bits := binary.LittleEndian.Uint32(block[4:])
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						if bx*4+x >= w || by*4+y >= h || bits>>uint(2*(y*4+x))&3 != 2 {
							continue
						}
						i := ((by*4+y)*w + bx*4 + x) * 4
						for c := 0; c < 3; c++ {
							decoded[i+c] = byte((pa[c] + pb[c] + 1) / 2)
						}
					}
				}
			}
		}
	}
	out := make([]float32, len(decoded))
	srgb := format >= 35916 && format <= 35919 || format == 36493
	for i, v := range decoded {
		f := float64(v) / 255
		if i%4 != 3 && srgb {
			if f <= .04045 {
				f /= 12.92
			} else {
				f = math.Pow((f+.055)/1.055, 2.4)
			}
		}
		out[i] = float32(f)
	}
	return out, nil
}

// RGTC signed values retain their normalized precision instead of making an
// intermediate signed-to-byte image conversion.
func decodeRGTC(format, w, h int, data []byte) ([]float32, error) {
	channels := 1
	if format >= 36285 {
		channels = 2
	}
	stride := 8 * channels
	bw := (w + 3) / 4
	if len(data) != bw*((h+3)/4)*stride {
		return nil, fmt.Errorf("invalid RGTC block length")
	}
	out := make([]float32, w*h*4)
	for i := 3; i < len(out); i += 4 {
		out[i] = 1
	}
	signed := format == 36284 || format == 36286
	for by := 0; by < (h+3)/4; by++ {
		for bx := 0; bx < bw; bx++ {
			for c := 0; c < channels; c++ {
				block := data[(by*bw+bx)*stride+c*8:]
				var p [8]float32
				endpoint := func(b byte) float32 {
					if !signed {
						return float32(b) / 255
					}
					v := int(int8(b))
					if v < -127 {
						v = -127
					}
					return float32(v) / 127
				}
				p[0], p[1] = endpoint(block[0]), endpoint(block[1])
				if p[0] > p[1] {
					for j := 2; j < 8; j++ {
						p[j] = (float32(8-j)*p[0] + float32(j-1)*p[1]) / 7
					}
				} else {
					for j := 2; j < 6; j++ {
						p[j] = (float32(6-j)*p[0] + float32(j-1)*p[1]) / 5
					}
					if signed {
						p[6] = -1
					}
					p[7] = 1
				}
				var bits uint64
				for j := 0; j < 6; j++ {
					bits |= uint64(block[j+2]) << uint(8*j)
				}
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						px, py := bx*4+x, by*4+y
						if px < w && py < h {
							out[(py*w+px)*4+c] = p[(bits>>uint(3*(y*4+x)))&7]
						}
					}
				}
			}
		}
	}
	return out, nil
}
