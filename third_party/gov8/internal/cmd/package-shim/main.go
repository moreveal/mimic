// Command package-shim deterministically compresses a source-built gov8 shim
// for inclusion in the Go module.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	input := flag.String("input", filepath.FromSlash("build/shim/gov8_shim.dll"), "source DLL")
	output := flag.String("output", filepath.FromSlash("internal/prebuilt/windows_amd64/gov8_shim.dll.gz"), "compressed output")
	flag.Parse()
	if err := packageShim(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, "package-shim:", err)
		os.Exit(1)
	}
}

func packageShim(input, output string) error {
	source, err := os.Open(input)
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".gov8-shim-*.gz")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	zw, err := gzip.NewWriterLevel(tmp, gzip.BestCompression)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	// Empty name/time and the fixed OS byte make the gzip header reproducible.
	zw.Name = ""
	zw.OS = 255
	h := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(zw, h), source)
	zipErr := zw.Close()
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if zipErr != nil {
		return zipErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Remove(output); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpPath, output); err != nil {
		return err
	}
	fmt.Printf("packaged %s: %d bytes, sha256 %s\n", output, size, hex.EncodeToString(h.Sum(nil)))
	return nil
}
