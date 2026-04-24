package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/session"
)

func withSession(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.SessionKey, session.Data{UserID: userID})
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
