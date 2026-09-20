// Command buildnative prepares Mimic's native style/layout archive.
package main

import (
	"fmt"
	"os"

	"github.com/moreveal/mimic/internal/nativebuild"
)

func main() {
	if _, err := nativebuild.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
