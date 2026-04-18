package middleware

import (
	"context"
	"net/http"

	"github.com/meowmix1337/argus/backend/internal/platform/session"
)

type contextKey string

const SessionKey contextKey = "session"

// RequireAuth returns a middleware that validates the session cookie.
// On success it stores session.Data in the request context under SessionKey.
// On failure it returns 401 JSON: {"error":"unauthorized"}
func RequireAuth(sessionSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(session.CookieName)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			data, err := session.Decode(sessionSecret, cookie.Value)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			ctx := context.WithValue(r.Context(), SessionKey, data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionFromContext retrieves the session data stored by RequireAuth.
func SessionFromContext(ctx context.Context) (session.Data, bool) {
	d, ok := ctx.Value(SessionKey).(session.Data)
	return d, ok
}

// UserIDFromRequest extracts the authenticated user ID from the request context.
func UserIDFromRequest(r *http.Request) (string, bool) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		return "", false
	}
	return sess.UserID, true
}
