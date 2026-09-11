package browser

import (
	"context"

	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
)

func (p *Page) NetworkPolicy() *network.RequestPolicy { return p.loader.Policy() }

func (p *Page) networkOnline() bool {
	return p.Environment().Navigator().Online && !p.NetworkPolicy().Offline()
}

// SetNetworkOffline applies to this Page's requests, Window realms and workers.
// Native connectivity events are queued on the existing Page event loop. A
// realm created afterwards simply observes the current state, without a stale
// transition event. Callers hold the Page command boundary.
func (p *Page) SetNetworkOffline(offline bool) {
	wasOnline := p.networkOnline()
	p.NetworkPolicy().SetOffline(offline)
	online := p.networkOnline()
	if wasOnline == online {
		return
	}
	eventType := "offline"
	if online {
		eventType = "online"
	}
	for _, realm := range p.evaluationRealms(nil) {
		if realm == nil || realm.closed || realm.inactive || realm.networkStateEvent == nil {
			continue
		}
		realm.scheduler.Post(scheduler.Network, 0, func(ctx context.Context) error {
			if realm.closed || realm.inactive {
				return nil
			}
			return realm.runOnOwner(ctx, func(ctx context.Context) error {
				typeValue := realm.val(eventType)
				defer releaseDebuggerValue(realm, typeValue)
				value, err := realm.runtime.Call(ctx, realm.networkStateEvent, nil, typeValue)
				releaseDebuggerValue(realm, value)
				return err
			})
		})
	}
}
