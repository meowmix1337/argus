package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

type StocksHandler struct {
	service  *service.StocksService
	validate *validator.Validate
}

func NewStocksHandler(svc *service.StocksService, v *validator.Validate) *StocksHandler {
	return &StocksHandler{service: svc, validate: v}
}

func (h *StocksHandler) AddRoutes(r chi.Router) {
	r.Get("/api/stocks", h.Get)
	r.Get("/api/stocks/watchlist", h.GetWatchlist)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Post("/api/stocks/watchlist", h.AddSymbol)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Delete("/api/stocks/watchlist/{symbol}", h.RemoveSymbol)
	r.With(httprate.LimitByIP(searchRateLimit, rateLimitWindow)).Get("/api/stocks/search", h.SearchSymbols)
}

// Get returns quotes for the current watchlist.
func (h *StocksHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	data, err := h.service.Fetch(r.Context(), userID)
	if err != nil {
		slog.Error("stocks fetch error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, data)
}

// GetWatchlist returns a paginated list of watchlist symbols.
func (h *StocksHandler) GetWatchlist(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := defaultWatchlistLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxWatchlistLimit {
		limit = maxWatchlistLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	syms, total, err := h.service.GetSymbolsPaginated(r.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("stocks get watchlist error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, WatchlistResponse{Symbols: syms, Total: total, Limit: limit, Offset: offset})
}

// AddSymbol adds a symbol to the watchlist.
// Body: {"symbol":"TSLA"}
// Returns 201 with updated list. Idempotent — re-adding an existing symbol succeeds silently.
func (h *StocksHandler) AddSymbol(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096) // 4 KB is generous for these small JSON bodies
	var body AddSymbolRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.AddSymbol(r.Context(), userID, body.Symbol); err != nil {
		slog.Error("stocks add error", "error", err)
		response.WriteError(w, http.StatusBadRequest, "invalid symbol")
		return
	}

	syms, total, err := h.service.GetSymbolsPaginated(r.Context(), userID, 0, 0)
	if err != nil {
		slog.Error("stocks get watchlist error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusCreated, WatchlistResponse{Symbols: syms, Total: total})
}

// RemoveSymbol removes a symbol from the watchlist.
// Returns 200 with updated list, or 404 if not found.
func (h *StocksHandler) RemoveSymbol(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sym := chi.URLParam(r, "symbol")
	if err := h.service.RemoveSymbol(r.Context(), userID, sym); err != nil {
		if errors.Is(err, apperrors.ErrSymbolNotFound) {
			response.WriteError(w, http.StatusNotFound, "symbol not found")
		} else {
			slog.Error("stocks remove error", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	syms, total, err := h.service.GetSymbolsPaginated(r.Context(), userID, 0, 0)
	if err != nil {
		slog.Error("stocks get watchlist error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, WatchlistResponse{Symbols: syms, Total: total})
}

// SearchSymbols proxies Finnhub symbol search to keep the API key server-side.
// Query param: q (required)
func (h *StocksHandler) SearchSymbols(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteError(w, http.StatusBadRequest, "missing query parameter 'q'")
		return
	}
	if len(q) > 50 {
		response.WriteError(w, http.StatusBadRequest, "query too long")
		return
	}

	results, err := h.service.SearchSymbols(r.Context(), q)
	if err != nil {
		slog.Error("stocks search error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	dtos := make([]SymbolSearchResultDTO, len(results))
	for i, r := range results {
		dtos[i] = SymbolSearchResultDTO{
			Symbol:      r.Symbol,
			Description: r.Description,
			Type:        r.Type,
		}
	}

	response.WriteJSON(w, http.StatusOK, SearchResponse{Results: dtos})
}

