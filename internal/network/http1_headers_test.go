package network

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCleartextHTTP1BrowserHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("Connection")+"|"+r.Header.Get("Priority"))
	}))
	defer server.Close()
	transport := &TLSClientTransport{fallback: http.DefaultTransport.(*http.Transport).Clone()}
	for _, tc := range []struct {
		name        string
		authorOrder []string
		want        string
	}{
		{name: "browser priority", want: "keep-alive|"},
		{name: "author priority", authorOrder: []string{"Priority"}, want: "keep-alive|u=4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequestWithContext(withBrowserHeaderLayout(t.Context(), Fetch, tc.authorOrder), http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Priority", "u=4")
			response, err := transport.RoundTrip(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != tc.want {
				t.Fatalf("wire headers = %q, want %q", body, tc.want)
			}
			if request.Header.Get("Priority") != "u=4" || request.Header.Get("Connection") != "" {
				t.Fatal("round trip mutated the original request")
			}
		})
	}
}
