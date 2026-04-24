package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	quotessvc "github.com/meowmix1337/argus/backend/internal/domain/external/quotes/service"
	sunrisesvc "github.com/meowmix1337/argus/backend/internal/domain/external/sunrise/service"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

type MetaHandler struct {
	sunrise *sunrisesvc.SunriseService
	quotes  *quotessvc.QuotesService
}

func NewMetaHandler(sunrise *sunrisesvc.SunriseService, quotes *quotessvc.QuotesService) *MetaHandler {
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
