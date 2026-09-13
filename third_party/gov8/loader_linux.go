//go:build linux && amd64

package gov8

import "github.com/maclof/gov8/internal/native"

func loadShimDLL(path string) (*native.DLL, error) { return native.LoadDLL(path) }
