package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// BearerAuth returns middleware that requires `Authorization: Bearer <token>`
// where the token matches one of the configured tokens (constant-time compared).
// Empty tokens slice rejects everything; callers must validate config first.
func BearerAuth(tokens []string) func(http.Handler) http.Handler {
	allowed := make([][]byte, len(tokens))
	for i, t := range tokens {
		allowed[i] = []byte(t)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const prefix = "Bearer "
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, prefix) {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}
			provided := []byte(strings.TrimSpace(h[len(prefix):]))
			if len(provided) == 0 {
				writeUnauthorized(w, "empty bearer token")
				return
			}
			for _, t := range allowed {
				if subtle.ConstantTimeCompare(provided, t) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeUnauthorized(w, "invalid token")
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="activity"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(Error{Error: msg})
}
