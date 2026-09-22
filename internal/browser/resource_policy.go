package browser

import "github.com/moreveal/mimic/internal/network"

// UpdateResourcePolicy atomically changes decisions for future operations in
// this Context. Already started loads keep their captured generation.
func (c *Context) UpdateResourcePolicy(policy network.ResourcePolicy) (uint64, error) {
	if err := c.lifetime.Err(); err != nil {
		return 0, err
	}
	return c.resourcePolicy.Update(policy)
}

func (c *Context) ResourcePolicy() (network.ResourcePolicy, bool) {
	return c.resourcePolicy.Policy()
}

func (c *Context) ResourcePolicyStats() network.ResourcePolicyStats {
	stats := c.resourcePolicy.Stats()
	stats.RetainedBodyBytes = c.network.BodyStorageStats().ResidentBytes
	return stats
}
