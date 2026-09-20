package browser

import "github.com/moreveal/mimic/internal/layoutblitz"

// blitzImageInputs runs on the canonical Page owner. In-flight replacements
// retain the previously decoded image exactly as image DOM accessors do.
// Existing resourceRevision makes completion/error invalidate native products.
func (r *Realm) blitzImageInputs() []layoutblitz.ImageIntrinsic {
	nodes := r.document.FindAllByTagName("img")
	images := make([]layoutblitz.ImageIntrinsic, 0, len(nodes))
	for _, node := range nodes {
		image := layoutblitz.ImageIntrinsic{ID: uint64(node.ID), Complete: true}
		if load := r.imageLoads[node.ID]; load != nil {
			image.Complete = load.complete
			if load.decoded != nil {
				image.Width, image.Height = uint32(load.decoded.Width), uint32(load.decoded.Height)
			}
		}
		images = append(images, image)
	}
	return images
}
