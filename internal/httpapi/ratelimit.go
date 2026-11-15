package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const forwardedForHeader = "X-Forwarded-For"

// rateLimited rejects requests once the client has used up its budget for
// the credential endpoints. It slows down password guessing and mass account
// creation; bcrypt alone makes each guess slow but not scarce.
func (a *api) rateLimited(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAllowed, retryAfter := a.authLimiter.Allow(clientIP(r, a.trustProxyHeaders))
		if !isAllowed {
			// Rounded up so a client that waits exactly this long gets through.
			seconds := int(math.Ceil(retryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeErrorBody(w, r, http.StatusTooManyRequests, "rate_limited", "too many attempts; try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the address the request is attributed to. The forwarded
// header is client-controlled, so it is only honoured when the deployment
// declares a trusted reverse proxy in front of the API. Then the right-most
// entry is used: it is the one appended by that proxy, while everything to
// its left was supplied by the client and can be forged.
func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if forwarded := r.Header.Get(forwardedForHeader); forwarded != "" {
			hops := strings.Split(forwarded, ",")
			return strings.TrimSpace(hops[len(hops)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
