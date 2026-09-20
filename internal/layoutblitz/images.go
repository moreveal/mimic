package layoutblitz

/*
#include <stdint.h>
typedef struct MimicBlitzHandle MimicBlitzHandle;
int32_t mimic_blitz_image(MimicBlitzHandle*, uint64_t, uint32_t, uint32_t, uint32_t);
*/
import "C"

// ImageIntrinsic shares only authoritative resource metadata, never pixel data.
type ImageIntrinsic struct {
	ID            uint64
	Width, Height uint32
	Complete      bool
}

func (o *Owner) ImageIntrinsic(image ImageIntrinsic) error {
	var complete C.uint32_t
	if image.Complete {
		complete = 1
	}
	return check(C.mimic_blitz_image(o.handle, C.uint64_t(image.ID), C.uint32_t(image.Width), C.uint32_t(image.Height), complete))
}
