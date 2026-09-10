package imageresource

import (
	"encoding/base64"
	"testing"
)

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
