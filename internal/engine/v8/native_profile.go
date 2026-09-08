//go:build windows && amd64

package v8

import (
	"encoding/json"
	"fmt"

	gov8 "github.com/maclof/gov8"
)

type nativeProfileChannel struct{ responses map[int32]json.RawMessage }

func (c *nativeProfileChannel) SendResponse(id int32, message *gov8.InspectorStringBuffer) {
	c.responses[id] = json.RawMessage(message.StringView().String())
}
func (*nativeProfileChannel) SendNotification(*gov8.InspectorStringBuffer) {}
func (*nativeProfileChannel) FlushProtocolNotifications()                  {}

// The inspector is confined to a diagnostic Eval on the isolate owner thread.
// It is closed before the surrounding context or isolate can be disposed.
func startNativeProfile(isolate *gov8.Isolate, realm *gov8.Context) (func() (json.RawMessage, error), error) {
	inspector, err := gov8.NewInspector(isolate)
	if err != nil {
		return nil, err
	}
	empty := gov8.EmptyInspectorStringView()
	if err = inspector.ContextCreated(realm, 1, empty, empty); err != nil {
		inspector.Close()
		return nil, err
	}
	channel := &nativeProfileChannel{responses: map[int32]json.RawMessage{}}
	session, err := inspector.Connect(1, channel, empty, gov8.InspectorFullyTrusted)
	if err != nil {
		inspector.ContextDestroyed(realm)
		inspector.Close()
		return nil, err
	}
	closeAll := func() { session.Close(); inspector.ContextDestroyed(realm); inspector.Close() }
	commands := []string{`{"id":1,"method":"Profiler.enable"}`, `{"id":2,"method":"Profiler.setSamplingInterval","params":{"interval":100}}`, `{"id":3,"method":"Profiler.start"}`}
	for i, command := range commands {
		if err = session.DispatchProtocolMessage(gov8.NewInspectorStringView8([]byte(command))); err != nil {
			closeAll()
			return nil, err
		}
		var response struct {
			Error any `json:"error"`
		}
		if err = json.Unmarshal(channel.responses[int32(i+1)], &response); err != nil || response.Error != nil {
			closeAll()
			return nil, fmt.Errorf("native profiler response: %v %v", err, response.Error)
		}
	}
	return func() (json.RawMessage, error) {
		defer closeAll()
		err := session.DispatchProtocolMessage(gov8.NewInspectorStringView8([]byte(`{"id":4,"method":"Profiler.stop"}`)))
		return channel.responses[4], err
	}, nil
}
