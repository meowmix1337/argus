package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	calendarsvc "github.com/meowmix1337/argus/backend/internal/domain/external/calendar/service"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

type CalendarHandler struct {
	service *calendarsvc.CalendarService
}

func NewCalendarHandler(svc *calendarsvc.CalendarService) *CalendarHandler {
	return &CalendarHandler{service: svc}
}

func (h *CalendarHandler) AddRoutes(r chi.Router) {
	r.Get("/api/calendar", h.Get)
}

func (h *CalendarHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := h.service.Fetch(r.Context(), userID)
	if err != nil {
		slog.Error("calendar fetch error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, data)
}
