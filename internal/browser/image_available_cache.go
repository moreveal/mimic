package browser

import "container/list"

const availableImageCacheBytes = 32 << 20
const availableImageCacheEntries = 1024

// HTML permits removing entries from the document's available-image list to
// save memory: https://html.spec.whatwg.org/multipage/images.html#the-list-of-available-images
// These are Mimic resource limits, not a claim about Chrome's eviction threshold.
// FIFO eviction drops only this additional reference; active image requests
// still own their decoded pixels. Both small/vector entries and pixel storage
// are bounded. All access belongs to the document's Page event loop.
type availableImageCache struct {
	entries map[preloadKey]*list.Element
	order   list.List
	bytes   int
}

type availableImageEntry struct {
	key   preloadKey
	image availableImage
	bytes int
}

func (c *availableImageCache) get(key preloadKey) (availableImage, bool) {
	if c != nil {
		if element := c.entries[key]; element != nil {
			return element.Value.(availableImageEntry).image, true
		}
	}
	return availableImage{}, false
}

func (c *availableImageCache) remove(element *list.Element) {
	entry := element.Value.(availableImageEntry)
	delete(c.entries, entry.key)
	c.bytes -= entry.bytes
	c.order.Remove(element)
}

func (c *availableImageCache) put(key preloadKey, image availableImage) {
	if image.resource == nil {
		return
	}
	if old := c.entries[key]; old != nil {
		c.remove(old)
	}
	// Count backing capacity, not just the exposed pixel length. Overhead covers
	// the decoded image, map/list entry and key; aliased images are conservatively
	// counted once per entry so sharing can never bypass the resource limit.
	cost := image.resource.RetainedBytes() + len(key.url) + len(key.destination) + len(key.mode) + len(key.credentials) + 256
	if cost > availableImageCacheBytes {
		return
	}
	if c.entries == nil {
		c.entries = make(map[preloadKey]*list.Element)
	}
	for len(c.entries) >= availableImageCacheEntries || c.bytes+cost > availableImageCacheBytes {
		c.remove(c.order.Front())
	}
	c.entries[key] = c.order.PushBack(availableImageEntry{key, image, cost})
	c.bytes += cost
}
