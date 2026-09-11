package imageresource

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// A generated 2x2 lossless grayscale AVIF, independently decoded in Chrome 152
// through Image + canvas readback (0, 85, 170, 255; alpha 255 throughout).
const avifGrayFixture = "AAAAIGZ0eXBhdmlmAAAAAGF2aWZtaWYxbWlhZk1BMUIAAADvbWV0YQAAAAAAAAAhaGRscgAAAAAAAAAAcGljdAAAAAAAAAAAAAAAAAAAAAAOcGl0bQAAAAAAAQAAAB5pbG9jAAAAAEQAAAEAAQAAAAEAAAEXAAAAMQAAAChpaW5mAAAAAAABAAAAGmluZmUCAAAAAAEAAGF2MDFDb2xvcgAAAABuaXBycAAAAE9pcGNvAAAAFGlzcGUAAAAAAAAAAgAAAAIAAAAOcGl4aQAAAAABCAAAABJhdjFDgQAcAAoEGAAyFQAAABNjb2xybmNseAABAA0ABoAAAAAXaXBtYQAAAAAAAAABAAEEAQKDBAAAADltZGF0EgAKBBgAMhUyJxAAAzd4Rf0JyNT1/Q844zkL9N/Q8e/DgZhZJXkld9eIJtJduXbnBg=="

func TestAVIFIntrinsicPixelsAndConcurrentDecode(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(avifGrayFixture)
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0, 0, 0, 255, 85, 85, 85, 255, 170, 170, 170, 255, 255, 255, 255, 255}
	for i := 0; i < 8; i++ {
		t.Run("independent decode", func(t *testing.T) {
			t.Parallel()
			got, err := Decode(data, "image/avif")
			if err != nil || got.Width != 2 || got.Height != 2 || !bytes.Equal(got.Pixels, expected) {
				t.Fatalf("AVIF pixels: %+v, %v", got, err)
			}
		})
	}
}

func TestAVIFRejectsTruncatedData(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString(avifGrayFixture)
	if _, err := Decode(data[:len(data)-12], "image/avif"); err == nil {
		t.Fatal("accepted truncated AVIF")
	}
}

func TestResourceDecodeBoundsAndPixels(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAABCAYAAAD0In+KAAAADklEQVR4nGP4z8DwHwQBEPgD/U6VwW8AAAAASUVORK5CYII=")
	decoded, err := Decode(data, "image/png")
	if err != nil || decoded.Width != 2 || decoded.Height != 1 || string(decoded.Pixels) != string([]byte{255, 0, 0, 255, 0, 255, 0, 255}) {
		t.Fatalf("pixels: %v %v", decoded, err)
	}
	for _, data := range []string{`<svg xmlns="http://www.w3.org/2000/svg" width="1e100"/>`, `<svg xmlns="http://www.w3.org/2000/svg">`, "broken"} {
		if _, err := Decode([]byte(data), "image/svg+xml"); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
}
