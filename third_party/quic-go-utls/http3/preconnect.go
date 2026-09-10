package http3

import "context"

// Preconnect establishes a pooled HTTP/3 connection without sending an HTTP
// request. It waits for the full handshake, so a caller can race this connection
// against TCP without submitting its application request on both protocols.
func (t *Transport) Preconnect(ctx context.Context, addr string) error {
	t.initOnce.Do(func() { t.initErr = t.init() })
	if t.initErr != nil {
		return t.initErr
	}
	cl, _, err := t.getClient(ctx, authorityAddr(addr), false)
	if err != nil {
		return err
	}
	defer cl.useCount.Add(-1)
	select {
	case <-cl.dialing:
	case <-ctx.Done():
		return ctx.Err()
	}
	if cl.dialErr != nil {
		return cl.dialErr
	}
	select {
	case <-cl.conn.HandshakeComplete():
		return nil
	case <-cl.conn.Context().Done():
		return context.Cause(cl.conn.Context())
	case <-ctx.Done():
		return ctx.Err()
	}
}
