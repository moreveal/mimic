// Command runmimic builds changed native inputs before running Mimic from source.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/moreveal/mimic/internal/nativebuild"
)

func main() {
	root, err := nativebuild.Ensure()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mimic:", err)
		os.Exit(1)
	}
	args := append([]string{"run", "./cmd/mimic"}, os.Args[1:]...)
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "mimic:", err)
		os.Exit(1)
	}
}
