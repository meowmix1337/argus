package handler

import (
	"context"
	"net/http"

	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/session"
)

func withSession(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.SessionKey, session.Data{UserID: userID})
	return r.WithContext(ctx)
}
