package cdp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/network"
)

type interceptAnswer struct {
	action  string
	url     string
	method  string
	headers http.Header
	body    []byte
	status  int
}
type ControlInterceptor struct {
	mu             sync.Mutex
	fetchEnabled   bool
	networkEnabled bool
	emit           func(string, any)
	pending        map[string]chan interceptAnswer
}

func NewControlInterceptor(emit func(string, any)) *ControlInterceptor {
	return &ControlInterceptor{emit: emit, pending: map[string]chan interceptAnswer{}}
}
func (i *ControlInterceptor) Enable(v bool)        { i.mu.Lock(); i.fetchEnabled = v; i.mu.Unlock() }
func (i *ControlInterceptor) EnableNetwork(v bool) { i.mu.Lock(); i.networkEnabled = v; i.mu.Unlock() }
func (i *ControlInterceptor) Close() {
	i.mu.Lock()
	i.fetchEnabled = false
	i.networkEnabled = false
	pending := i.pending
	i.pending = map[string]chan interceptAnswer{}
	i.mu.Unlock()
	for _, ch := range pending {
		ch <- interceptAnswer{action: "fail"}
	}
}
func (i *ControlInterceptor) pause(ctx context.Context, stage string, req network.Request, res *network.Response) (interceptAnswer, error) {
	i.mu.Lock()
	fetchEnabled, networkEnabled := i.fetchEnabled, i.networkEnabled
	if !fetchEnabled && !networkEnabled {
		i.mu.Unlock()
		return interceptAnswer{action: "continue"}, nil
	}
	id := uuid.NewString()
	ch := make(chan interceptAnswer, 1)
	i.pending[id] = ch
	i.mu.Unlock()
	requestData := map[string]any{"url": req.URL.String(), "method": req.Method, "headers": flatHeaders(req.Headers), "postData": string(req.Body)}
	if networkEnabled && !fetchEnabled {
		i.emit("Network.requestIntercepted", map[string]any{"interceptionId": id, "request": requestData, "frameId": req.ContextID, "resourceType": resourceType(req.Initiator), "isNavigationRequest": req.Initiator == network.Navigation})
		select {
		case a := <-ch:
			return a, nil
		case <-ctx.Done():
			return interceptAnswer{}, ctx.Err()
		}
	}
	params := map[string]any{"requestId": id, "request": requestData, "resourceType": resourceType(req.Initiator), "frameId": req.ContextID}
	if res != nil {
		params["responseStatusCode"] = res.Status
		params["responseHeaders"] = res.Headers
	}
	i.emit("Fetch.requestPaused", params)
	select {
	case a := <-ch:
		return a, nil
	case <-ctx.Done():
		return interceptAnswer{}, ctx.Err()
	}
}
func (i *ControlInterceptor) Before(ctx context.Context, req network.Request) (network.Decision, error) {
	a, err := i.pause(ctx, "request", req, nil)
	if err != nil {
		return network.Decision{}, err
	}
	switch a.action {
	case "fail":
		return network.Decision{Block: fmt.Errorf("request blocked by controller")}, nil
	case "fulfill":
		return network.Decision{Response: &network.Response{Status: a.status, Headers: a.headers, Body: a.body, URL: req.URL, Synthetic: true}}, nil
	}
	copy := req
	if a.url != "" {
		u, err := url.Parse(a.url)
		if err != nil {
			return network.Decision{}, err
		}
		copy.URL = u
	}
	if a.method != "" {
		copy.Method = a.method
	}
	if a.headers != nil {
		copy.Headers = a.headers
	}
	if a.body != nil {
		copy.Body = a.body
	}
	return network.Decision{Request: &copy}, nil
}
func (i *ControlInterceptor) After(ctx context.Context, req network.Request, res network.Response) (network.Response, error) {
	i.mu.Lock()
	enabled := i.fetchEnabled
	i.mu.Unlock()
	if !enabled {
		return res, nil
	}
	a, err := i.pause(ctx, "response", req, &res)
	if err != nil {
		return res, err
	}
	if a.action == "fail" {
		return res, fmt.Errorf("response blocked by controller")
	}
	if a.action == "fulfill" {
		if a.status != 0 {
			res.Status = a.status
		}
		if a.headers != nil {
			res.Headers = a.headers
		}
		if a.body != nil {
			res.Body = a.body
		}
		res.Synthetic = true
	}
	return res, nil
}
func flatHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		out[k] = strings.Join(v, ", ")
	}
	return out
}
func resourceType(i network.Initiator) string {
	switch i {
	case network.Navigation, network.Iframe:
		return "Document"
	case network.Script:
		return "Script"
	case network.Stylesheet:
		return "Stylesheet"
	case network.Fetch:
		return "Fetch"
	case network.XHR:
		return "XHR"
	case network.Image:
		return "Image"
	}
	return "Other"
}
func (i *ControlInterceptor) Resolve(id string, a interceptAnswer) error {
	i.mu.Lock()
	ch := i.pending[id]
	delete(i.pending, id)
	i.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("unknown interception %q", id)
	}
	ch <- a
	return nil
}
func decodeBody(s string) []byte {
	if s == "" {
		return nil
	}
	b, _ := base64.StdEncoding.DecodeString(s)
	return b
}
