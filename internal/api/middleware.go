package api

import (
	"net/http"

	"github.com/selfstack/selfstack/internal/auth"
)

func AuthMiddleware(a *auth.Auth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/status" || r.URL.Path == "/api/devices/pair" {
				next.ServeHTTP(w, r)
				return
			}

			token := ""
			if cookie, err := r.Cookie("selfstack_token"); err == nil {
				token = cookie.Value
			}
			if token == "" {
				token = r.Header.Get("Authorization")
			}

			if token == "" {
				jsonError(w, 401, "device not trusted — visit dashboard from a trusted device to approve")
				return
			}

			_, ok := a.VerifyToken(token)
			if !ok {
				jsonError(w, 401, "invalid or expired device token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
