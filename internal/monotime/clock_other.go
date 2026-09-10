//go:build !windows

package monotime

import "time"

func Now() time.Time                      { return time.Now() }
func Since(start time.Time) time.Duration { return time.Since(start) }
