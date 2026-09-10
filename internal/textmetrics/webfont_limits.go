package textmetrics

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"github.com/andybalholm/brotli"
	"io"
)

// Check compressed output before the converter allocates it. Container length
// fields alone cannot enforce a resource budget against malformed streams.
func preflightWebFont(data []byte) error {
	const limit = 32 << 20
	check := func(reader io.Reader) error {
		n, err := io.Copy(io.Discard, io.LimitReader(reader, limit+1))
		if err != nil {
			return err
		}
		if n > limit {
			return fmt.Errorf("font expansion limit")
		}
		return nil
	}
	if len(data) < 4 {
		return fmt.Errorf("invalid font header")
	}
	if string(data[:4]) == "wOFF" {
		if len(data) < 44 {
			return fmt.Errorf("invalid WOFF header")
		}
		count := int(binary.BigEndian.Uint16(data[12:14]))
		if count > 4096 || 44+count*20 > len(data) {
			return fmt.Errorf("invalid WOFF directory")
		}
		for i := 0; i < count; i++ {
			p := 44 + i*20
			offset := uint64(binary.BigEndian.Uint32(data[p+4 : p+8]))
			size := uint64(binary.BigEndian.Uint32(data[p+8 : p+12]))
			original := uint64(binary.BigEndian.Uint32(data[p+12 : p+16]))
			if size > original || original > limit || offset+size > uint64(len(data)) {
				return fmt.Errorf("invalid WOFF table")
			}
			if size < original {
				r, err := zlib.NewReader(bytes.NewReader(data[offset : offset+size]))
				if err != nil {
					return err
				}
				err = check(r)
				r.Close()
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	if string(data[:4]) != "wOF2" {
		return nil
	}
	if len(data) < 48 {
		return fmt.Errorf("invalid WOFF2 header")
	}
	count := int(binary.BigEndian.Uint16(data[12:14]))
	if count > 4096 {
		return fmt.Errorf("font table limit")
	}
	at := 48
	base128 := func() error {
		for i := 0; i < 5; i++ {
			if at >= len(data) {
				return io.ErrUnexpectedEOF
			}
			b := data[at]
			at++
			if b&128 == 0 {
				return nil
			}
		}
		return fmt.Errorf("invalid Base128 length")
	}
	for i := 0; i < count; i++ {
		if at >= len(data) {
			return io.ErrUnexpectedEOF
		}
		flag := data[at]
		at++
		tag := flag & 63
		glyf := tag == 10 || tag == 11
		if tag == 63 {
			if at+4 > len(data) {
				return io.ErrUnexpectedEOF
			}
			name := string(data[at : at+4])
			glyf = name == "glyf" || name == "loca"
			at += 4
		}
		if err := base128(); err != nil {
			return err
		}
		version := flag >> 6
		if glyf && version != 3 || !glyf && version != 0 {
			if err := base128(); err != nil {
				return err
			}
		}
	}
	size := uint64(binary.BigEndian.Uint32(data[20:24]))
	if uint64(at)+size > uint64(len(data)) {
		return fmt.Errorf("invalid WOFF2 compressed range")
	}
	return check(brotli.NewReader(bytes.NewReader(data[at : uint64(at)+size])))
}
