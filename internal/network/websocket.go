package network

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	ws "github.com/bogdanfinn/websocket"
)

type WebSocketConn interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	WriteControl(int, []byte, time.Time) error
	Subprotocol() string
	Close() error
}

func websocketHeaders(headers http.Header) fhttp.Header {
	out := make(fhttp.Header, len(headers))
	for name, values := range headers {
		out[name] = append([]string(nil), values...)
	}
	return out
}

func (l *Loader) DialWebSocket(ctx context.Context, rawURL string, headers http.Header) (WebSocketConn, error) {
	if target, err := url.Parse(rawURL); err == nil {
		cookies := l.cookies.ForURL(target)
		if len(cookies) > 0 {
			request := &http.Request{Header: headers}
			for _, cookie := range cookies {
				request.AddCookie(cookie)
			}
			headers.Set("Cookie", strings.Join(request.Header.Values("Cookie"), "; "))
		}
	}
	if transport, ok := l.transport.(*TLSClientTransport); ok {
		return transport.dialWebSocket(ctx, rawURL, websocketHeaders(headers))
	}
	dialer := *ws.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, rawURL, websocketHeaders(headers))
	return conn, err
}

func (t *TLSClientTransport) dialWebSocket(ctx context.Context, rawURL string, headers fhttp.Header) (WebSocketConn, error) {
	options := append([]tlsclient.HttpClientOption(nil), t.clientOptions...)
	options = append(options, tlsclient.WithDisableProtocolRacing(), tlsclient.WithForceHttp1())
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
	if err != nil {
		return nil, err
	}
	socket, err := tlsclient.NewWebsocket(tlsclient.NewNoopLogger(), tlsclient.WithTlsClient(client), tlsclient.WithUrl(rawURL), tlsclient.WithHeaders(headers), tlsclient.WithHandshakeTimeoutMilliseconds(30000))
	if err != nil {
		return nil, err
	}
	return socket.Connect(ctx)
}
