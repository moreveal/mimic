package cdp

import (
	"context"
	"errors"
)

func navigationReplyError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "net::ERR_ABORTED"
	case errors.Is(err, context.DeadlineExceeded):
		return "net::ERR_TIMED_OUT"
	default:
		return err.Error()
	}
}
