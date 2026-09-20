package layoutblitz

/*
#include <stdint.h>
typedef struct MimicBlitzHandle MimicBlitzHandle;
int32_t mimic_blitz_transform(MimicBlitzHandle*,uint64_t,double*,uint32_t*);
*/
import "C"

func (o *Owner) GeometryTransform(id uint64) ([]float64, error) {
	var values [19]C.double
	var present C.uint32_t
	if err := check(C.mimic_blitz_transform(o.handle, C.uint64_t(id), &values[0], &present)); err != nil {
		return nil, err
	}
	if present == 0 {
		return nil, nil
	}
	result := make([]float64, len(values))
	for i, value := range values {
		result[i] = float64(value)
	}
	return result, nil
}
