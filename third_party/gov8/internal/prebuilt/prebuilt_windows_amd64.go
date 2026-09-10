//go:build windows && amd64

// Package prebuilt materializes gov8's pinned native shim.
package prebuilt

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	ABI      = 44
	Size     = int64(45_930_496)
	SHA256   = "919522b4d4ed80671586a1b7a0efc144a533e91e08319715e76ed27962cee0ff"
	fileName = "gov8_shim.dll"
)

//go:embed windows_amd64/gov8_shim.dll.gz
var compressed []byte

// Path verifies and materializes the embedded DLL in the user's cache.
func Path() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate the user cache: %w", err)
	}
	return Materialize(cacheRoot)
}

// Materialize verifies and extracts the embedded DLL below cacheRoot.
func Materialize(cacheRoot string) (string, error) {
	dir := filepath.Join(cacheRoot, "gov8", "shim", fmt.Sprintf("abi-%d", ABI), SHA256)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create shim cache %s: %w", dir, err)
	}
	dst := filepath.Join(dir, fileName)
	if err := Verify(dst); err == nil {
		return dst, nil
	}

	tmp, err := os.CreateTemp(dir, ".gov8-shim-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary shim in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("open embedded shim: %w", err)
	}
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, h), zr)
	zipErr := zr.Close()
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if copyErr != nil {
		return "", fmt.Errorf("extract embedded shim: %w", copyErr)
	}
	if zipErr != nil {
		return "", fmt.Errorf("verify embedded shim stream: %w", zipErr)
	}
	if syncErr != nil {
		return "", fmt.Errorf("flush embedded shim: %w", syncErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close embedded shim: %w", closeErr)
	}
	if written != Size || hex.EncodeToString(h.Sum(nil)) != SHA256 {
		return "", fmt.Errorf("embedded shim digest mismatch: got %d bytes sha256 %s", written, hex.EncodeToString(h.Sum(nil)))
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		// Another process may have won the same content-addressed publish.
		if verifyErr := Verify(dst); verifyErr == nil {
			return dst, nil
		}
		// A stale or truncated cache entry is safe to replace: dst is the
		// exact content-addressed file owned by this package.
		if removeErr := os.Remove(dst); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", fmt.Errorf("replace invalid cached shim %s: %w", dst, removeErr)
		}
		if err := os.Rename(tmpPath, dst); err != nil {
			return "", fmt.Errorf("publish embedded shim %s: %w", dst, err)
		}
	}
	if err := Verify(dst); err != nil {
		return "", fmt.Errorf("verify published shim: %w", err)
	}
	return dst, nil
}

// Verify checks that path contains the exact packaged DLL.
func Verify(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if info.Size() != Size {
		return fmt.Errorf("size is %d, want %d", info.Size(), Size)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != SHA256 {
		return fmt.Errorf("sha256 is %s, want %s", got, SHA256)
	}
	return nil
}
