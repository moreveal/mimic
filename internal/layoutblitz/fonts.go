package layoutblitz

/*
#include <stdint.h>
#include <stddef.h>
typedef struct MimicBlitzHandle MimicBlitzHandle;
int32_t mimic_blitz_fonts(MimicBlitzHandle*, const uint8_t*, size_t);
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"unsafe"
)

type Font struct {
	Family string
	Weight float32
	Italic bool
	Bytes  []byte
}

// ReplaceFonts copies a complete authoritative collection into native memory.
// Removing a face requires calling this with the remaining collection, including
// an empty collection after clear. No caller pointers survive the transaction.
func (o *Owner) ReplaceFonts(fonts []Font) error {
	if len(fonts) > 4096 {
		return fmt.Errorf("blitz font collection exceeds face bound")
	}
	size := 4
	for _, font := range fonts {
		if len(font.Bytes) > 64<<20 || len(font.Family) > 1<<20 {
			return fmt.Errorf("blitz font resource exceeds byte bound")
		}
		size += 16 + len(font.Family) + len(font.Bytes)
		if size > 64<<20 {
			return fmt.Errorf("blitz font collection exceeds byte bound")
		}
	}
	data := make([]byte, 4, size)
	binary.LittleEndian.PutUint32(data, uint32(len(fonts)))
	for _, font := range fonts {
		data = binary.LittleEndian.AppendUint32(data, uint32(len(font.Family)))
		data = binary.LittleEndian.AppendUint32(data, uint32(len(font.Bytes)))
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(font.Weight))
		var italic uint32
		if font.Italic {
			italic = 1
		}
		data = binary.LittleEndian.AppendUint32(data, italic)
		data = append(data, font.Family...)
		data = append(data, font.Bytes...)
	}
	err := check(C.mimic_blitz_fonts(o.handle, (*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data))))
	runtime.KeepAlive(data)
	return err
}
