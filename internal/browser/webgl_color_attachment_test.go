//go:build (windows || linux) && amd64

package browser

import "testing"

func TestWebGLColorAttachmentMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "webgl_color_attachment")
}
