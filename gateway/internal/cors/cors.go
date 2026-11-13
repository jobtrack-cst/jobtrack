// Package cors lets one configured browser origin call the gateway (AGENTS.md Section 15, JT-53).
package cors

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrInvalidOrigin is returned for any value that is not exactly one http or https origin.
var ErrInvalidOrigin = errors.New(
	"CORS_ALLOWED_ORIGIN must be one lowercase http or https origin without a path, for example http://localhost:5173")

// Methods used by the HTTP annotations in proto/. A preflight for any other method is refused.
var allowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// Headers the shared frontend API client sends. Browsers list them in lowercase.
var allowedHeaders = map[string]bool{"authorization": true, "content-type": true}

// Browsers cache an accepted preflight for this many seconds.
const maxAgeSeconds = "600"

// Policy holds the one allowed origin. The zero value means CORS is off.
type Policy struct {
	origin string
}

// Parse checks CORS_ALLOWED_ORIGIN. An empty value turns CORS off.
func Parse(value string) (Policy, error) {
	if value == "" {
		return Policy{}, nil
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return Policy{}, ErrInvalidOrigin
	}
	// Browsers send Origin as lowercase scheme://host[:port] only, and the match is exact,
	// so anything else (path, query, user info, uppercase, spaces) could never match.
	if value != strings.ToLower(value) || value != u.Scheme+"://"+u.Host || strings.HasSuffix(u.Host, ":") {
		return Policy{}, ErrInvalidOrigin
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return Policy{}, ErrInvalidOrigin
		}
	}
	return Policy{origin: value}, nil
}

// Origin returns the allowed origin, or "" when CORS is off.
func (p Policy) Origin() string {
	return p.origin
}

// Wrap must sit outside the JWT middleware. It answers CORS preflight requests itself,
// so they never reach authentication or a backend service, and adds the allow-origin
// header to every other response for the configured origin. All other requests,
// including OPTIONS without Access-Control-Request-Method, go to next unchanged.
func (p Policy) Wrap(next http.Handler) http.Handler {
	if p.origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Vary", "Origin")
		origins := r.Header.Values("Origin")
		allowed := len(origins) == 1 && origins[0] == p.origin

		if r.Method == http.MethodOptions && len(r.Header.Values("Access-Control-Request-Method")) > 0 {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
			if !allowed || !preflightAllowed(r) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			h.Set("Access-Control-Allow-Origin", p.origin)
			h.Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", maxAgeSeconds)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if allowed {
			h.Set("Access-Control-Allow-Origin", p.origin)
		}
		next.ServeHTTP(w, r)
	})
}

func preflightAllowed(r *http.Request) bool {
	methods := r.Header.Values("Access-Control-Request-Method")
	if len(methods) != 1 || !methodAllowed(methods[0]) {
		return false
	}
	for _, line := range r.Header.Values("Access-Control-Request-Headers") {
		for _, name := range strings.Split(line, ",") {
			name = strings.ToLower(strings.TrimSpace(name))
			if name != "" && !allowedHeaders[name] {
				return false
			}
		}
	}
	return true
}

func methodAllowed(method string) bool {
	for _, m := range allowedMethods {
		if method == m {
			return true
		}
	}
	return false
}
