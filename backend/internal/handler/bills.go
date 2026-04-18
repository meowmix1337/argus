package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
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
	r.Get("/api/bills/due", h.ListDue)
	r.Get("/api/bills/due/year", h.ListDueYear)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/bills", h.Create)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/bills/{id}/pay", h.MarkPaid)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/bills/payments/{paymentId}", h.UnmarkPaid)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Patch("/api/bills/{id}", h.Update)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/bills/{id}", h.Delete)
}

func (h *BillsHandler) ListDue(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	now := time.Now().UTC()
	year := now.Year()
	month := int(now.Month())

	if v := r.URL.Query().Get("year"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			year = n
		}
	}
	if v := r.URL.Query().Get("month"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 12 {
			month = n
		}
	}

	bills, err := h.service.ListForMonth(r.Context(), userID, year, month)
	if err != nil {
		slog.Error("failed to list bills due", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, BillsDueResponse{
		Bills: bills,
		Year:  year,
		Month: month,
	})
}

func (h *BillsHandler) ListDueYear(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	year := time.Now().UTC().Year()
	if v := r.URL.Query().Get("year"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			year = n
		}
	}

	byMonth, err := h.service.ListYear(r.Context(), userID, year)
	if err != nil {
		slog.Error("failed to list bills due for year", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	months := make([]BillsMonthEntry, 12)
	for i := 1; i <= 12; i++ {
		months[i-1] = BillsMonthEntry{Month: i, Bills: byMonth[i]}
	}
	response.WriteJSON(w, http.StatusOK, BillsYearResponse{Year: year, Months: months})
}

func (h *BillsHandler) MarkPaid(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	billID := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req MarkPaidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payment, err := h.service.MarkPaid(r.Context(), userID, billID, req.ComputedDueDate, req.PaidDate, req.Note)
	if err != nil {
		if errors.Is(err, apperrors.ErrBillNotFound) {
			response.WriteError(w, http.StatusNotFound, "bill not found")
		} else if errors.Is(err, apperrors.ErrBillValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else if errors.Is(err, apperrors.ErrBillAlreadyPaid) {
			response.WriteError(w, http.StatusConflict, "bill occurrence already paid")
		} else {
			slog.Error("failed to mark bill as paid", "error", err, "user_id", userID, "bill_id", billID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, BillPaymentResponse{
		ID:              payment.ID,
		BillID:          payment.BillID,
		ComputedDueDate: payment.ComputedDueDate,
		PaidDate:        payment.PaidDate,
		Note:            payment.Note,
		CreatedAt:       payment.CreatedAt,
	})
}

func (h *BillsHandler) UnmarkPaid(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	paymentID := chi.URLParam(r, "paymentId")
	if err := h.service.Unmark(r.Context(), userID, paymentID); err != nil {
		if errors.Is(err, apperrors.ErrBillPaymentNotFound) {
			response.WriteError(w, http.StatusNotFound, "bill payment not found")
		} else {
			slog.Error("failed to unmark bill payment", "error", err, "user_id", userID, "payment_id", paymentID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *BillsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
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
	userID, ok := middleware.UserIDFromRequest(r)
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
	userID, ok := middleware.UserIDFromRequest(r)
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
	userID, ok := middleware.UserIDFromRequest(r)
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
