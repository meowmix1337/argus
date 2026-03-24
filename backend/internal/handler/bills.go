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

// BillsHandler handles CRUD for user-scoped bills.
type BillsHandler struct {
	service  *service.BillsService
	validate *validator.Validate
}

// NewBillsHandler creates a new BillsHandler.
func NewBillsHandler(svc *service.BillsService, v *validator.Validate) *BillsHandler {
	return &BillsHandler{service: svc, validate: v}
}

// AddRoutes registers bill routes on the given router.
func (h *BillsHandler) AddRoutes(r chi.Router) {
	r.Get("/api/bills", h.List)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Post("/api/bills", h.Create)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Patch("/api/bills/{id}", h.Update)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Delete("/api/bills/{id}", h.Delete)
}

func (h *BillsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := defaultBillLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxBillLimit {
		limit = maxBillLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	bills, total, err := h.service.List(r.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("failed to list bills", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, BillListResponse{
		Bills:  billsToResponse(bills),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *BillsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req CreateBillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	bill, err := h.service.Create(r.Context(), userID, reqToBillUpdate(req))
	if err != nil {
		if errors.Is(err, apperrors.ErrBillValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to create bill", "error", err, "user_id", userID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, billToResponse(bill))
}

func (h *BillsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req UpdateBillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	bill, err := h.service.Update(r.Context(), id, userID, updateReqToBillUpdate(req))
	if err != nil {
		if errors.Is(err, apperrors.ErrBillNotFound) {
			response.WriteError(w, http.StatusNotFound, "bill not found")
		} else if errors.Is(err, apperrors.ErrBillValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to update bill", "error", err, "user_id", userID, "bill_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, billToResponse(bill))
}

func (h *BillsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), id, userID); err != nil {
		if errors.Is(err, apperrors.ErrBillNotFound) {
			response.WriteError(w, http.StatusNotFound, "bill not found")
		} else {
			slog.Error("failed to delete bill", "error", err, "user_id", userID, "bill_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
