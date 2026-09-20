package layoutblitz

/*
#include <stdint.h>
#include <stddef.h>
typedef struct MimicBlitzHandle MimicBlitzHandle;
int32_t mimic_blitz_control_value(MimicBlitzHandle*,uint64_t,const char*,size_t,uint32_t);
*/
import "C"
import "unsafe"

func (o *Owner) ControlValue(id uint64, value string, multiline bool) error {
	var multi C.uint32_t
	if multiline {
		multi = 1
	}
	return check(C.mimic_blitz_control_value(o.handle, C.uint64_t(id), (*C.char)(unsafe.Pointer(unsafe.StringData(value))), C.size_t(len(value)), multi))
}
