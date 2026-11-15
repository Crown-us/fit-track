package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name              string
		remoteAddr        string
		forwardedFor      string
		trustProxyHeaders bool
		want              string
	}{
		{name: "remote address without port", remoteAddr: "203.0.113.7:51234", want: "203.0.113.7"},
		{name: "ipv6 remote address", remoteAddr: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "unparseable remote address is used as is", remoteAddr: "pipe", want: "pipe"},
		{
			name:       "forwarded header is ignored when not trusted",
			remoteAddr: "203.0.113.7:51234", forwardedFor: "198.51.100.9",
			want: "203.0.113.7",
		},
		{
			name:       "trusted proxy: single hop",
			remoteAddr: "10.0.0.2:40000", forwardedFor: "198.51.100.9", trustProxyHeaders: true,
			want: "198.51.100.9",
		},
		{
			// The left entry is what the client claimed; only the right-most
			// one was written by the trusted proxy.
			name:       "trusted proxy: forged left entry is not used",
			remoteAddr: "10.0.0.2:40000", forwardedFor: "1.2.3.4, 198.51.100.9", trustProxyHeaders: true,
			want: "198.51.100.9",
		},
		{
			name:       "trusted proxy without the header falls back",
			remoteAddr: "10.0.0.2:40000", trustProxyHeaders: true,
			want: "10.0.0.2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/login", http.NoBody)
			request.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				request.Header.Set(forwardedForHeader, tt.forwardedFor)
			}
			if got := clientIP(request, tt.trustProxyHeaders); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
