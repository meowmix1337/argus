package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	weathersvc "github.com/meowmix1337/argus/backend/internal/domain/external/weather/service"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

type WeatherHandler struct {
	service *weathersvc.WeatherService
}

func NewWeatherHandler(svc *weathersvc.WeatherService) *WeatherHandler {
	return &WeatherHandler{service: svc}
}

func (h *WeatherHandler) AddRoutes(r chi.Router) {
	r.Get("/api/weather", h.Get)
}

func (h *WeatherHandler) Get(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Fetch(r.Context())
	if err != nil {
		slog.Error("weather fetch error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, data)
}
