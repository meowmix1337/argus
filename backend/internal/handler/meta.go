package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

type MetaHandler struct {
	sunrise *service.SunriseService
	quotes  *service.QuotesService
}

func NewMetaHandler(sunrise *service.SunriseService, quotes *service.QuotesService) *MetaHandler {
	return &MetaHandler{sunrise: sunrise, quotes: quotes}
}

func (h *MetaHandler) AddRoutes(r chi.Router) {
	r.Get("/api/meta", h.Get)
}

func (h *MetaHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sunriseTime, sunsetTime, daylight, sunriseErr := h.sunrise.Fetch(ctx)
	quote, quoteErr := h.quotes.Fetch(ctx)

	if sunriseErr != nil && quoteErr != nil {
		response.WriteError(w, http.StatusServiceUnavailable, "meta services unavailable")
		return
	}

	meta := model.MetaData{
		Sunrise:  sunriseTime,
		Sunset:   sunsetTime,
		Daylight: daylight,
		Quote:    quote,
	}

	response.WriteJSON(w, http.StatusOK, meta)
}
