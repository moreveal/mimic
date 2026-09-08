package browser

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

type messagePortState struct {
	peer   string
	owner  *Realm
	closed bool
}

func (p *Page) newMessageChannel(owner *Realm) []string {
	first, second := uuid.NewString(), uuid.NewString()
	p.mu.Lock()
	p.messagePorts[first] = &messagePortState{peer: second, owner: owner}
	p.messagePorts[second] = &messagePortState{peer: first, owner: owner}
	p.mu.Unlock()
	return []string{first, second}
}

func (p *Page) transferMessagePorts(portIDs []string, owner *Realm) {
	if owner == nil || len(portIDs) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, portID := range portIDs {
		if port := p.messagePorts[portID]; port != nil && !port.closed {
			port.owner = owner
		}
	}
}

func (p *Page) closeMessagePort(portID string) {
	p.mu.Lock()
	if port := p.messagePorts[portID]; port != nil {
		port.closed = true
	}
	p.mu.Unlock()
}

func (p *Page) postMessagePort(source *Realm, portID string, data any, transferIDs []string) error {
	p.mu.Lock()
	port := p.messagePorts[portID]
	if port == nil || port.closed || port.owner != source {
		p.mu.Unlock()
		return nil
	}
	peer := p.messagePorts[port.peer]
	if peer == nil || peer.closed || peer.owner == nil {
		p.mu.Unlock()
		return nil
	}
	peerID, target := port.peer, peer.owner
	for _, transferID := range transferIDs {
		if transferred := p.messagePorts[transferID]; transferred != nil && !transferred.closed {
			transferred.owner = target
		}
	}
	p.mu.Unlock()

	p.trace.Add(trace.Lifecycle, "messagePortMessagePosted", map[string]any{"sourceRealm": source.ID, "targetRealm": target.ID, "portCount": len(transferIDs)})
	eventLoop := target.browserEventLoop()
	eventLoop.Post(scheduler.PostedMessage, 0, func(ctx context.Context) error {
		if target.messagePortReceiver == nil {
			return fmt.Errorf("message port receiver is unavailable")
		}
		if receiverType := target.runtime.TypeOf(target.messagePortReceiver); receiverType != "function" {
			return fmt.Errorf("message port receiver in realm %s has type %s", target.ID, receiverType)
		}
		_, err := target.runtime.Call(ctx, target.messagePortReceiver, target.runtime.Get("window"), target.runtime.Value(peerID), target.runtime.Value(data), target.runtime.Value(transferIDs))
		if err != nil {
			return err
		}
		return target.runtime.MicrotaskCheckpoint()
	})
	return nil
}
