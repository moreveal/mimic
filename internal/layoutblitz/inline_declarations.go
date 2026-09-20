package layoutblitz

/*
#include <stdint.h>
#include <stddef.h>
typedef struct MimicBlitzHandle MimicBlitzHandle;
int32_t mimic_blitz_inline_declarations(MimicBlitzHandle*,uint64_t,const char*,size_t);
*/
import "C"

import (
	"encoding/json"
	"github.com/moreveal/mimic/internal/dom"
	"strings"
	"unsafe"
)

// Publish the existing canonical parsed CSSOM input without replacing its
// serialized attribute (which remains observable to attribute selectors).
func (o *Owner) inlineDeclarations(node dom.Node) error {
	css := node.Attributes["style"]
	if node.StyleDeclarationsJSON != "" {
		var entries []struct {
			Name, Value, Priority string
			ParsedValue           *string `json:"parsedValue"`
		}
		if err := json.Unmarshal([]byte(node.StyleDeclarationsJSON), &entries); err != nil {
			return err
		}
		var text strings.Builder
		for _, entry := range entries {
			value := entry.Value
			if entry.ParsedValue != nil {
				value = *entry.ParsedValue
			}
			text.WriteString(entry.Name)
			text.WriteByte(':')
			text.WriteString(value)
			if entry.Priority != "" {
				text.WriteString(" !")
				text.WriteString(entry.Priority)
			}
			text.WriteByte(';')
		}
		css = text.String()
	}
	return check(C.mimic_blitz_inline_declarations(o.handle, C.uint64_t(node.ID), (*C.char)(unsafe.Pointer(unsafe.StringData(css))), C.size_t(len(css))))
}
