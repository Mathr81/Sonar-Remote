package sonar

import (
	"net/http"
	"net/http/httputil"
)

// retryTransport re-discovers Sonar's (possibly rotated) port on a failed
// request and retries once, so the proxy self-heals after a Sonar restart.
type retryTransport struct {
	c    *Client
	next http.RoundTripper
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if err == nil {
		return resp, nil
	}
	if derr := t.c.Discover(); derr != nil {
		return resp, err
	}
	if base := t.c.Base(); base != nil {
		req.URL.Scheme = base.Scheme
		req.URL.Host = base.Host
		req.Host = base.Host
	}
	return t.next.RoundTrip(req)
}

// Handler returns an http.Handler that proxies everything under /api to Sonar.
func (c *Client) Handler() http.Handler {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if base := c.Base(); base != nil {
				req.URL.Scheme = base.Scheme
				req.URL.Host = base.Host
				req.Host = base.Host
			}
			req.Header.Del("Origin")
			req.Header.Del("Referer")
		},
		Transport: &retryTransport{c: c, next: http.DefaultTransport},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, `{"error":"sonar unavailable"}`, http.StatusBadGateway)
		},
	}
	return http.StripPrefix("/api", proxy)
}
